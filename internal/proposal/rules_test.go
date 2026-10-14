package proposal

import (
	"slices"
	"strings"
	"testing"
)

func TestForbiddenPhrasesAreSortedUniqueAndNonEmpty(t *testing.T) {
	phrases := ForbiddenPhrases()
	if len(phrases) == 0 {
		t.Fatal("ForbiddenPhrases() is empty")
	}
	if !slices.IsSorted(phrases) {
		t.Fatalf("ForbiddenPhrases() is not sorted: %+v", phrases)
	}
	seen := map[string]bool{}
	for _, p := range phrases {
		if strings.TrimSpace(p) == "" {
			t.Fatal("ForbiddenPhrases() contains a blank phrase")
		}
		if seen[p] {
			t.Fatalf("ForbiddenPhrases() repeats %q", p)
		}
		seen[p] = true
	}
}

func TestForbiddenLanguageMatchesWordsWholeAndPunctuationLiterally(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{name: "a plain promise", text: "We guarantee delivery in four weeks.", want: []string{"guarantee"}},
		{name: "case is ignored", text: "We GUARANTEE delivery.", want: []string{"guarantee"}},
		{name: "a word is not matched inside another word", text: "The canalwaysomething device shipped.", want: nil},
		{name: "an absolute", text: "The service is always available.", want: []string{"always"}},
		{name: "a percentage promise", text: "Uptime of 100% is included.", want: []string{"100%"}},
		{name: "a coverage promise", text: "Support is available 24/7.", want: []string{"24/7"}},
		{name: "several promises are all reported", text: "A world-class team that never fails.", want: []string{"never fails", "world-class"}},
		{name: "nothing forbidden", text: "We migrated 27 customers in 2025.", want: nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ForbiddenLanguage(tc.text)
			if len(got) != len(tc.want) {
				t.Fatalf("ForbiddenLanguage(%q) = %+v, want %+v", tc.text, got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("ForbiddenLanguage(%q) = %+v, want %+v", tc.text, got, tc.want)
				}
			}
		})
	}
}

func TestForbiddenLanguageNeverRepeatsAPhrase(t *testing.T) {
	got := ForbiddenLanguage("We guarantee it. We guarantee it again.")
	if len(got) != 1 {
		t.Fatalf("ForbiddenLanguage() = %+v, want one entry", got)
	}
}

func evidencePair() (Evidence, Evidence) {
	supporting := Evidence{
		ID:        "C-1",
		Customer:  "Acme",
		Kind:      EvidenceApprovedClaim,
		Source:    "claims/acme.json",
		Excerpt:   "The migration completed in six weeks.",
		Approved:  true,
		Assertion: AssertionSupports,
	}
	other := supporting
	other.ID = "C-2"
	return supporting, other
}

func TestDetectConflictsReportsPolarity(t *testing.T) {
	supporting, contradicting := evidencePair()
	contradicting.Excerpt = "The migration did not complete."
	contradicting.Assertion = AssertionContradicts

	claims := []Claim{{
		ID:          "CL-1",
		EvidenceIDs: []string{supporting.Ref(), contradicting.Ref()},
	}}
	conflicts := DetectConflicts(claims, []Evidence{supporting, contradicting})

	if len(conflicts) != 1 {
		t.Fatalf("DetectConflicts() = %+v, want one conflict", conflicts)
	}
	c := conflicts[0]
	if c.Kind != ConflictPolarity || c.ClaimID != "CL-1" {
		t.Fatalf("conflict = %+v, want a polarity conflict on CL-1", c)
	}
	if c.Resolution != ResolutionUnresolved {
		t.Fatalf("resolution = %q, want %q", c.Resolution, ResolutionUnresolved)
	}
	if len(c.EvidenceIDs) != 2 {
		t.Fatalf("evidence = %+v, want both refs", c.EvidenceIDs)
	}
}

func TestDetectConflictsReportsDifferingAttributeValues(t *testing.T) {
	first, second := evidencePair()
	first.Attribute = "delivery_time"
	first.Value = "six weeks"
	second.Attribute = "delivery_time"
	second.Value = "four weeks"

	claims := []Claim{{
		ID:          "CL-1",
		EvidenceIDs: []string{first.Ref(), second.Ref()},
	}}
	conflicts := DetectConflicts(claims, []Evidence{first, second})

	if len(conflicts) != 1 {
		t.Fatalf("DetectConflicts() = %+v, want one conflict", conflicts)
	}
	c := conflicts[0]
	if c.Kind != ConflictAttribute {
		t.Fatalf("kind = %q, want %q", c.Kind, ConflictAttribute)
	}
	if c.Attribute != "delivery_time" {
		t.Fatalf("attribute = %q, want %q", c.Attribute, "delivery_time")
	}
	if c.ValueA != "four weeks" || c.ValueB != "six weeks" {
		t.Fatalf("values = %q and %q, want them ordered four weeks before six weeks", c.ValueA, c.ValueB)
	}
}

func TestDetectConflictsIgnoresAgreementAndClaimsWithoutEvidence(t *testing.T) {
	first, second := evidencePair()
	first.Attribute = "delivery_time"
	first.Value = "six weeks"
	second.Attribute = "delivery_time"
	second.Value = "six weeks"

	claims := []Claim{
		{ID: "CL-1", EvidenceIDs: []string{first.Ref(), second.Ref()}},
		{ID: "CL-2"},
	}
	if conflicts := DetectConflicts(claims, []Evidence{first, second}); len(conflicts) != 0 {
		t.Fatalf("DetectConflicts() = %+v, want none", conflicts)
	}
}

func TestDetectConflictsIsDeterministic(t *testing.T) {
	first, second := evidencePair()
	first.Attribute = "delivery_time"
	first.Value = "six weeks"
	second.Attribute = "delivery_time"
	second.Value = "four weeks"
	third := second
	third.ID = "C-3"
	third.Attribute = "team_size"
	third.Value = "four engineers"
	fourth := second
	fourth.ID = "C-4"
	fourth.Attribute = "team_size"
	fourth.Value = "two engineers"

	claims := []Claim{
		{ID: "CL-2", EvidenceIDs: []string{first.Ref(), second.Ref()}},
		{ID: "CL-1", EvidenceIDs: []string{third.Ref(), fourth.Ref()}},
	}

	run := DetectConflicts(claims, []Evidence{first, second, third, fourth})
	if len(run) != 2 {
		t.Fatalf("DetectConflicts() = %+v, want two conflicts", run)
	}
	if run[0].ClaimID != "CL-1" || run[0].Attribute != "team_size" {
		t.Fatalf("first conflict = %+v, want the claim sorted first with its attribute", run[0])
	}
	if run[1].ClaimID != "CL-2" || run[1].Attribute != "delivery_time" {
		t.Fatalf("second conflict = %+v", run[1])
	}
}

func catalogue() []PricingLine {
	return []PricingLine{
		{Customer: "Acme", Label: "Implementation fee", Currency: "USD", Terms: "net 30"},
		{Customer: "Acme", Label: "Annual support", Currency: "USD", Terms: "12 months, net 30"},
		{Customer: "Beta", Label: "Implementation fee", Currency: "EUR", Terms: "net 60"},
	}
}

func TestPricingPlaceholdersTurnsTheWholeCatalogueIntoOpenLines(t *testing.T) {
	brief := Brief{Customer: "Acme"}
	placeholders, findings := PricingPlaceholders(brief, nil, catalogue())

	if len(findings) != 0 {
		t.Fatalf("findings = %+v, want none", findings)
	}
	if len(placeholders) != 2 {
		t.Fatalf("placeholders = %+v, want the two Acme lines", placeholders)
	}
	if placeholders[0].ID != "P-1" || placeholders[1].ID != "P-2" {
		t.Fatalf("ids = %q, %q", placeholders[0].ID, placeholders[1].ID)
	}
	if placeholders[0].Label != "Implementation fee" || placeholders[0].Currency != "USD" || placeholders[0].Terms != "net 30" {
		t.Fatalf("line = %+v, want the catalogue values copied", placeholders[0])
	}
	if placeholders[0].Value != "{{Implementation fee}}" {
		t.Fatalf("value = %q, want an open placeholder", placeholders[0].Value)
	}
	if !IsPlaceholderValue(placeholders[0].Value) {
		t.Fatalf("%q is not recognised as a placeholder", placeholders[0].Value)
	}
}

func TestPricingPlaceholdersHonoursTheRequestedOrderAndDropsDuplicates(t *testing.T) {
	brief := Brief{Customer: "Acme"}
	requested := []string{"Annual support", "Implementation fee", "Annual support"}

	placeholders, findings := PricingPlaceholders(brief, requested, catalogue())

	if len(findings) != 0 {
		t.Fatalf("findings = %+v, want none", findings)
	}
	if len(placeholders) != 2 {
		t.Fatalf("placeholders = %+v, want two", placeholders)
	}
	if placeholders[0].Label != "Annual support" || placeholders[1].Label != "Implementation fee" {
		t.Fatalf("placeholders = %+v, want the requested order", placeholders)
	}
}

func TestPricingPlaceholdersRefusesToInventALine(t *testing.T) {
	brief := Brief{Customer: "Acme"}
	placeholders, findings := PricingPlaceholders(brief, []string{"Implementation fee", "Success bonus"}, catalogue())

	if !hasCode(findings, CodePricingLineNotCatalogued) {
		t.Fatalf("findings = %+v, want %q", findings, CodePricingLineNotCatalogued)
	}
	if findings[0].Subject != "Success bonus" {
		t.Fatalf("subject = %q, want the requested label", findings[0].Subject)
	}
	if len(placeholders) != 1 || placeholders[0].Label != "Implementation fee" {
		t.Fatalf("placeholders = %+v, want only the catalogued line", placeholders)
	}
}

func TestPricingPlaceholdersRefusesAnotherCustomersLine(t *testing.T) {
	brief := Brief{Customer: "Acme"}
	betaOnly := []PricingLine{{Customer: "Beta", Label: "Onboarding", Currency: "EUR", Terms: "net 60"}}

	placeholders, findings := PricingPlaceholders(brief, []string{"Onboarding"}, betaOnly)

	if !hasCode(findings, CodePricingLineOtherCustomer) {
		t.Fatalf("findings = %+v, want %q", findings, CodePricingLineOtherCustomer)
	}
	if len(placeholders) != 0 {
		t.Fatalf("placeholders = %+v, want none", placeholders)
	}
}

func TestPricingPlaceholdersRejectsABlankRequest(t *testing.T) {
	_, findings := PricingPlaceholders(Brief{Customer: "Acme"}, []string{"  "}, catalogue())
	if !hasCode(findings, CodePricingLineNotCatalogued) {
		t.Fatalf("findings = %+v, want %q", findings, CodePricingLineNotCatalogued)
	}
	if findings[0].Subject != "pricing.request" {
		t.Fatalf("subject = %q, want %q", findings[0].Subject, "pricing.request")
	}
}

func TestPlaceholderValueAndIsPlaceholderValueAgree(t *testing.T) {
	if got := PlaceholderValue("  Implementation  fee "); got != "{{Implementation fee}}" {
		t.Fatalf("PlaceholderValue() = %q", got)
	}

	tests := []struct {
		value string
		want  bool
	}{
		{value: "{{Implementation fee}}", want: true},
		{value: "TBD", want: true},
		{value: "[to confirm]", want: true},
		{value: "", want: false},
		{value: "   ", want: false},
		{value: "5000", want: false},
		{value: "USD 5,000.00", want: false},
		{value: "{{}}", want: false},
		{value: "around five thousand", want: false},
	}
	for _, tc := range tests {
		if got := IsPlaceholderValue(tc.value); got != tc.want {
			t.Fatalf("IsPlaceholderValue(%q) = %v, want %v", tc.value, got, tc.want)
		}
	}
}
