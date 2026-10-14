package proposal

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type fakeClaimSource struct {
	records []Evidence
	err     error
	calls   int
}

func (f *fakeClaimSource) Evidence(_ context.Context, _ string) ([]Evidence, error) {
	f.calls++
	return f.records, f.err
}

type fakePricingSource struct {
	lines []PricingLine
	err   error
}

func (f fakePricingSource) Pricing(_ context.Context, _ string) ([]PricingLine, error) {
	return f.lines, f.err
}

type fakeTemplateSource struct {
	sections []SectionCandidate
	err      error
}

func (f fakeTemplateSource) Template(_ context.Context) ([]SectionCandidate, error) {
	return f.sections, f.err
}

type fakeOutliner struct {
	sections []SectionCandidate
	err      error
	calls    int
}

func (f *fakeOutliner) Outline(_ context.Context, _ Brief) ([]SectionCandidate, error) {
	f.calls++
	return f.sections, f.err
}

type fakeChecker struct {
	candidates []ClaimCandidate
	err        error
	calls      int
	records    []Evidence
}

func (f *fakeChecker) Check(_ context.Context, _ Brief, _ []Section, records []Evidence) ([]ClaimCandidate, error) {
	f.calls++
	f.records = records
	return f.candidates, f.err
}

func pipelineBrief() Brief {
	return Brief{
		Customer:    "Acme",
		Needs:       []string{"cut the manual reconciliation work"},
		Scope:       []string{"billing migration"},
		Constraints: []string{"must land before Q3 close"},
	}
}

func approvedRecord() Evidence {
	return Evidence{
		ID:         "C-014",
		Customer:   "Acme",
		Kind:       EvidenceApprovedClaim,
		Source:     "claims/acme.json",
		Excerpt:    "We migrated 27 customers\n to the new platform in 2025.",
		Approved:   true,
		ApprovedBy: "delivery@acme",
		ApprovedOn: "2026-01-02",
		Assertion:  AssertionSupports,
	}
}

func baselinePipeline() Pipeline {
	return Pipeline{
		Evidence: &fakeClaimSource{},
		Template: fakeTemplateSource{sections: []SectionCandidate{{Title: "Understanding of the need"}, {Title: "Proposed scope"}}},
	}
}

func TestPipelineBaselineNeverCallsAModelAndSaysSo(t *testing.T) {
	draft, err := baselinePipeline().Build(context.Background(), pipelineBrief())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if draft.Mode != ModeBaseline {
		t.Fatalf("mode = %q, want %q: a draft written without a model must say so", draft.Mode, ModeBaseline)
	}
	if len(draft.Sections) != 2 || draft.Sections[0].ID != "S-1" || draft.Sections[1].ID != "S-2" {
		t.Fatalf("sections = %+v", draft.Sections)
	}
	if len(draft.Claims) != 2 {
		t.Fatalf("claims = %+v, want one per need and scope item", draft.Claims)
	}
	for _, claim := range draft.Claims {
		if claim.Status != ClaimUnresolved {
			t.Fatalf("claim %s status = %q, want %q: nothing was looked up", claim.ID, claim.Status, ClaimUnresolved)
		}
		if claim.Reason == "" {
			t.Fatalf("claim %s has no reason", claim.ID)
		}
	}
}

func TestPipelineGroundedModeUsesBothAgents(t *testing.T) {
	outliner := &fakeOutliner{sections: []SectionCandidate{{Title: "Understanding of the need"}}}
	checker := &fakeChecker{candidates: []ClaimCandidate{{
		Section:   "S-1",
		Text:      "We have migrated comparable customers before.",
		Quotation: "migrated 27 customers to the new platform",
		Evidence:  []string{"claims/acme.json#C-014"},
	}}}

	pipeline := baselinePipeline()
	pipeline.Outliner = outliner
	pipeline.Checker = checker

	draft, err := pipeline.Build(context.Background(), pipelineBrief())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	_ = draft

	if draft.Mode != ModeGrounded {
		t.Fatalf("mode = %q, want %q", draft.Mode, ModeGrounded)
	}
	if outliner.calls != 1 || checker.calls != 1 {
		t.Fatalf("agent calls = %d and %d, want exactly one each", outliner.calls, checker.calls)
	}
}

func TestPipelineGroundsAClaimThatSurvivesTheRules(t *testing.T) {
	source := &fakeClaimSource{records: []Evidence{approvedRecord()}}
	outliner := &fakeOutliner{sections: []SectionCandidate{{Title: "Understanding of the need"}}}
	checker := &fakeChecker{candidates: []ClaimCandidate{{
		Section:   "S-1",
		Text:      "We have migrated comparable customers before.",
		Quotation: "migrated 27 customers to the new platform",
		Evidence:  []string{"claims/acme.json#C-014"},
	}}}

	pipeline := Pipeline{Evidence: source, Outliner: outliner, Checker: checker}
	draft, err := pipeline.Build(context.Background(), pipelineBrief())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if len(draft.Claims) != 1 || draft.Claims[0].Status != ClaimGrounded {
		t.Fatalf("claims = %+v, want one grounded claim", draft.Claims)
	}
	if len(draft.Evidence) != 1 || draft.Evidence[0].Ref() != "claims/acme.json#C-014" {
		t.Fatalf("evidence = %+v, want the record the claim rests on", draft.Evidence)
	}
	if len(draft.Findings) != 0 {
		t.Fatalf("findings = %+v, want none", draft.Findings)
	}
	if len(draft.Sections[0].ClaimIDs) != 1 || draft.Sections[0].ClaimIDs[0] != "CL-1" {
		t.Fatalf("section claims = %+v, want CL-1 linked", draft.Sections[0].ClaimIDs)
	}
}

func TestPipelineRejectsAnotherCustomersRecordAndNeverCarriesIt(t *testing.T) {
	foreign := Evidence{
		ID:         "C-900",
		Customer:   "Beta",
		Kind:       EvidenceCaseStudy,
		Source:     "claims/beta.json",
		Excerpt:    "Beta's confidential migration data.",
		Approved:   true,
		ApprovedBy: "legal@beta",
		ApprovedOn: "2026-01-02",
		Assertion:  AssertionSupports,
	}
	source := &fakeClaimSource{records: []Evidence{approvedRecord(), foreign}}
	checker := &fakeChecker{candidates: []ClaimCandidate{{
		Section:   "S-1",
		Text:      "We have migrated comparable customers before.",
		Quotation: "Beta's confidential migration data.",
		Evidence:  []string{foreign.Ref()},
	}}}

	pipeline := Pipeline{Evidence: source, Checker: checker}
	draft, err := pipeline.Build(context.Background(), pipelineBrief())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if !hasCode(draft.Findings, CodeClaimEvidenceOtherCustomer) {
		t.Fatalf("findings = %+v, want %q", draft.Findings, CodeClaimEvidenceOtherCustomer)
	}
	if draft.Claims[0].Status != ClaimRejected {
		t.Fatalf("claim status = %q, want %q", draft.Claims[0].Status, ClaimRejected)
	}
	for _, record := range draft.Evidence {
		if NormalizeText(record.Customer) != "Acme" {
			t.Fatalf("the draft carries a record of %q", record.Customer)
		}
	}

	encoded, err := json.Marshal(draft)
	if err != nil {
		t.Fatalf("marshalling the draft: %v", err)
	}
	if strings.Contains(string(encoded), "confidential") {
		t.Fatal("the draft carries another customer's excerpt")
	}
}

func TestPipelineKeepsAnUnsupportedClaimVisible(t *testing.T) {
	checker := &fakeChecker{candidates: []ClaimCandidate{{
		Section: "S-1",
		Text:    "We can deliver in four weeks.",
	}}}
	pipeline := Pipeline{Evidence: &fakeClaimSource{}, Checker: checker}

	draft, err := pipeline.Build(context.Background(), pipelineBrief())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if len(draft.Claims) != 1 {
		t.Fatalf("claims = %+v, want the unsupported claim reported, not dropped", draft.Claims)
	}
	if draft.Claims[0].Status != ClaimUnresolved {
		t.Fatalf("status = %q, want %q", draft.Claims[0].Status, ClaimUnresolved)
	}
	if !hasCode(draft.Findings, CodeClaimNoEvidence) {
		t.Fatalf("findings = %+v, want %q", draft.Findings, CodeClaimNoEvidence)
	}
	if len(ErrorsIn(draft.Findings)) != 0 {
		t.Fatalf("an unsupported claim produced a blocking finding: %+v", ErrorsIn(draft.Findings))
	}
}

func TestPipelineLeavesEveryAmountOpen(t *testing.T) {
	pipeline := baselinePipeline()
	pipeline.Pricing = fakePricingSource{lines: []PricingLine{
		{Customer: "Acme", Label: "Implementation fee", Currency: "USD", Terms: "net 30"},
	}}

	draft, err := pipeline.Build(context.Background(), pipelineBrief())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if len(draft.PricingPlaceholders) != 1 {
		t.Fatalf("placeholders = %+v, want one", draft.PricingPlaceholders)
	}
	placeholder := draft.PricingPlaceholders[0]
	if !IsPlaceholderValue(placeholder.Value) {
		t.Fatalf("value = %q, want an open placeholder", placeholder.Value)
	}

	encoded, err := json.Marshal(draft)
	if err != nil {
		t.Fatalf("marshalling the draft: %v", err)
	}
	for _, digit := range []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"} {
		if strings.Contains(placeholder.Value, digit) {
			t.Fatalf("the amount is not open: %q holds a digit", placeholder.Value)
		}
	}
	if len(encoded) == 0 {
		t.Fatal("the draft did not marshal")
	}
}

func TestPipelineRefusesToInventAPricedLine(t *testing.T) {
	pipeline := baselinePipeline()
	pipeline.Pricing = fakePricingSource{lines: []PricingLine{
		{Customer: "Acme", Label: "Implementation fee", Currency: "USD", Terms: "net 30"},
	}}
	pipeline.RequestedPricing = []string{"Success bonus"}

	draft, err := pipeline.Build(context.Background(), pipelineBrief())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if !hasCode(draft.Findings, CodePricingLineNotCatalogued) {
		t.Fatalf("findings = %+v, want %q", draft.Findings, CodePricingLineNotCatalogued)
	}
	if len(ErrorsIn(draft.Findings)) == 0 {
		t.Fatal("an invented priced line did not block the draft")
	}
}

func TestPipelineReportsAMissingCustomerWithoutReadingAnySource(t *testing.T) {
	source := &fakeClaimSource{records: []Evidence{approvedRecord()}}
	pipeline := Pipeline{Evidence: source, Template: fakeTemplateSource{sections: BuiltinTemplate()}}

	draft, err := pipeline.Build(context.Background(), Brief{Needs: []string{"cut cost"}})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if source.calls != 0 {
		t.Fatalf("the source was read %d times, want 0: there is no customer to scope to", source.calls)
	}
	if !hasCode(draft.Findings, CodeBriefCustomerMissing) {
		t.Fatalf("findings = %+v, want %q", draft.Findings, CodeBriefCustomerMissing)
	}
	if len(ErrorsIn(draft.Findings)) == 0 {
		t.Fatal("a draft with no customer is shareable")
	}
}

func TestPipelineSurfacesASourceFailureAsAnError(t *testing.T) {
	pipeline := baselinePipeline()
	pipeline.Evidence = &fakeClaimSource{err: errors.New("disk on fire")}

	if _, err := pipeline.Build(context.Background(), pipelineBrief()); err == nil {
		t.Fatal("Build() error = nil, want the source failure reported")
	}
}

func TestPipelineMarshalsAnEmptyListAsAList(t *testing.T) {
	pipeline := Pipeline{
		Evidence: &fakeClaimSource{},
		Template: fakeTemplateSource{sections: []SectionCandidate{{Title: "Only section"}}},
	}
	draft, err := pipeline.Build(context.Background(), pipelineBrief())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	encoded, err := json.Marshal(draft)
	if err != nil {
		t.Fatalf("marshalling the draft: %v", err)
	}
	for _, want := range []string{`"assumptions":[]`, `"pricingPlaceholders":[]`} {
		if !strings.Contains(string(encoded), want) {
			t.Fatalf("the draft does not carry %s:\n%s", want, encoded)
		}
	}
}

func TestPipelineFindingsComeBackSorted(t *testing.T) {
	pipeline := baselinePipeline()
	pipeline.Pricing = fakePricingSource{lines: []PricingLine{
		{Customer: "Acme", Label: "", Currency: "USD", Terms: "net 30"},
	}}

	draft, err := pipeline.Build(context.Background(), Brief{Customer: "Acme", Needs: []string{"cut cost"}})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(draft.Findings) < 2 {
		t.Fatalf("findings = %+v, want more than one", draft.Findings)
	}
	for i := 1; i < len(draft.Findings); i++ {
		if rank(draft.Findings[i-1].Severity) > rank(draft.Findings[i].Severity) {
			t.Fatalf("findings are not ordered by severity: %+v", draft.Findings)
		}
	}
}
