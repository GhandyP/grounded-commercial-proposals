package proposal

import (
	"slices"
	"strings"
	"testing"
)

// acmeRecord is the record every grounding test starts from: an approved claim
// of the brief's own customer, with a quotable excerpt.
func acmeRecord() Evidence {
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

func TestGroundClaimsGroundsAClaimWithAVerbatimQuotation(t *testing.T) {
	record := acmeRecord()
	brief := Brief{Customer: "Acme"}

	candidates := []ClaimCandidate{{
		Section:   "S-1",
		Text:      "We have migrated comparable customers before.",
		Quotation: "migrated 27 customers to the new platform",
		Evidence:  []string{record.Ref()},
	}}

	claims, conflicts, findings := GroundClaims(brief, candidates, []Evidence{record})

	if len(findings) != 0 {
		t.Fatalf("GroundClaims() findings = %+v, want none", findings)
	}
	if len(conflicts) != 0 {
		t.Fatalf("GroundClaims() conflicts = %+v, want none", conflicts)
	}
	if len(claims) != 1 {
		t.Fatalf("GroundClaims() = %+v, want one claim", claims)
	}
	claim := claims[0]
	if claim.ID != "CL-1" {
		t.Fatalf("claim id = %q, want %q", claim.ID, "CL-1")
	}
	if claim.Section != "S-1" {
		t.Fatalf("claim section = %q, want %q", claim.Section, "S-1")
	}
	if claim.Status != ClaimGrounded {
		t.Fatalf("claim status = %q (reason %q), want %q", claim.Status, claim.Reason, ClaimGrounded)
	}
	if len(claim.EvidenceIDs) != 1 || claim.EvidenceIDs[0] != record.Ref() {
		t.Fatalf("claim evidence = %+v, want the cited reference", claim.EvidenceIDs)
	}
	if claim.Reason != "" {
		t.Fatalf("grounded claim carries a reason: %q", claim.Reason)
	}
}

func TestGroundClaimsRejectsAnotherCustomersRecordEvenWhenItMatches(t *testing.T) {
	record := acmeRecord()
	record.Customer = "Beta"
	record.Source = "claims/beta.json"

	brief := Brief{Customer: "Acme"}
	candidates := []ClaimCandidate{{
		Section:   "S-1",
		Text:      "We have migrated comparable customers before.",
		Quotation: "migrated 27 customers to the new platform",
		Evidence:  []string{record.Ref()},
	}}

	claims, _, findings := GroundClaims(brief, candidates, []Evidence{record})

	if !hasCode(findings, CodeClaimEvidenceOtherCustomer) {
		t.Fatalf("findings = %+v, want %q", findings, CodeClaimEvidenceOtherCustomer)
	}
	if sev := findings[0].Severity; sev != SeverityError {
		t.Fatalf("severity = %q, want %q: another customer's record must block", sev, SeverityError)
	}
	if claims[0].Status != ClaimRejected {
		t.Fatalf("status = %q, want %q", claims[0].Status, ClaimRejected)
	}
	if claims[0].Reason == "" {
		t.Fatal("rejected claim carries no reason")
	}
}

func TestGroundClaimsRejectsAnUnknownCitation(t *testing.T) {
	tests := []struct {
		name     string
		evidence []string
	}{
		{name: "reference that matches nothing", evidence: []string{"claims/acme.json#C-999"}},
		{name: "blank reference", evidence: []string{"   "}},
		{name: "empty reference", evidence: []string{""}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			candidates := []ClaimCandidate{{
				Section:   "S-1",
				Text:      "We have migrated comparable customers before.",
				Quotation: "migrated 27 customers",
				Evidence:  tc.evidence,
			}}
			claims, _, findings := GroundClaims(Brief{Customer: "Acme"}, candidates, []Evidence{acmeRecord()})

			if !hasCode(findings, CodeClaimEvidenceUnknown) {
				t.Fatalf("findings = %+v, want %q", findings, CodeClaimEvidenceUnknown)
			}
			if claims[0].Status != ClaimRejected {
				t.Fatalf("status = %q, want %q", claims[0].Status, ClaimRejected)
			}
		})
	}
}

func TestGroundClaimsRejectsAForbiddenPromiseEvenWithPerfectEvidence(t *testing.T) {
	record := acmeRecord()
	candidates := []ClaimCandidate{{
		Section:   "S-1",
		Text:      "We guarantee the migration will finish on time.",
		Quotation: "migrated 27 customers to the new platform",
		Evidence:  []string{record.Ref()},
	}}

	claims, _, findings := GroundClaims(Brief{Customer: "Acme"}, candidates, []Evidence{record})

	if !hasCode(findings, CodeClaimForbiddenLanguage) {
		t.Fatalf("findings = %+v, want %q", findings, CodeClaimForbiddenLanguage)
	}
	if claims[0].Status != ClaimRejected {
		t.Fatalf("status = %q, want %q: a promise is never grounded", claims[0].Status, ClaimRejected)
	}
	if !strings.Contains(findings[0].Message, "guarantee") {
		t.Fatalf("message %q does not name the forbidden phrase", findings[0].Message)
	}
}

func TestGroundClaimsLeavesAnUnsupportedClaimUnresolved(t *testing.T) {
	candidates := []ClaimCandidate{{
		Section: "S-2",
		Text:    "We can deliver in four weeks.",
	}}
	claims, _, findings := GroundClaims(Brief{Customer: "Acme"}, candidates, []Evidence{acmeRecord()})

	if !hasCode(findings, CodeClaimNoEvidence) {
		t.Fatalf("findings = %+v, want %q", findings, CodeClaimNoEvidence)
	}
	if sev := findings[0].Severity; sev != SeverityWarning {
		t.Fatalf("severity = %q, want %q: an unsupported claim stays visible", sev, SeverityWarning)
	}
	if claims[0].Status != ClaimUnresolved {
		t.Fatalf("status = %q, want %q", claims[0].Status, ClaimUnresolved)
	}
	if claims[0].Reason == "" {
		t.Fatal("unresolved claim carries no reason")
	}
}

func TestGroundClaimsDoesNotGroundAnUnapprovedRecord(t *testing.T) {
	record := acmeRecord()
	record.Approved = false
	record.ApprovedBy = ""
	record.ApprovedOn = ""

	candidates := []ClaimCandidate{{
		Section:   "S-1",
		Text:      "We have migrated comparable customers before.",
		Quotation: "migrated 27 customers to the new platform",
		Evidence:  []string{record.Ref()},
	}}
	claims, _, findings := GroundClaims(Brief{Customer: "Acme"}, candidates, []Evidence{record})

	if !hasCode(findings, CodeClaimEvidenceNotApproved) {
		t.Fatalf("findings = %+v, want %q", findings, CodeClaimEvidenceNotApproved)
	}
	if claims[0].Status != ClaimUnresolved {
		t.Fatalf("status = %q, want %q", claims[0].Status, ClaimUnresolved)
	}
}

func TestGroundClaimsNeverGroundsWithoutAVerbatimQuotation(t *testing.T) {
	record := acmeRecord()
	tests := []struct {
		name      string
		quotation string
	}{
		{name: "no quotation at all", quotation: ""},
		{name: "blank quotation", quotation: "   "},
		{name: "words the excerpt does not contain", quotation: "migrated 31 customers to the new platform"},
		{name: "different case is not verbatim", quotation: "we MIGRATED 27 customers to the new platform"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			candidates := []ClaimCandidate{{
				Section:   "S-1",
				Text:      "We have migrated comparable customers before.",
				Quotation: tc.quotation,
				Evidence:  []string{record.Ref()},
			}}
			claims, _, findings := GroundClaims(Brief{Customer: "Acme"}, candidates, []Evidence{record})

			if !hasCode(findings, CodeClaimQuotationNotVerbatim) {
				t.Fatalf("findings = %+v, want %q", findings, CodeClaimQuotationNotVerbatim)
			}
			if claims[0].Status == ClaimGrounded {
				t.Fatalf("status = %q, want anything but grounded", claims[0].Status)
			}
		})
	}
}

func TestGroundClaimsReportsContradictionsAndDowngradesTheClaim(t *testing.T) {
	supporting := acmeRecord()
	contradicting := acmeRecord()
	contradicting.ID = "C-015"
	contradicting.Source = "claims/acme-conflicts.json"
	contradicting.Excerpt = "The 2025 migration was rolled back before completion."
	contradicting.Assertion = AssertionContradicts

	candidates := []ClaimCandidate{{
		Section:   "S-1",
		Text:      "We have migrated comparable customers before.",
		Quotation: "migrated 27 customers to the new platform",
		Evidence:  []string{supporting.Ref(), contradicting.Ref()},
	}}

	claims, conflicts, findings := GroundClaims(Brief{Customer: "Acme"}, candidates, []Evidence{supporting, contradicting})

	if len(conflicts) != 1 {
		t.Fatalf("conflicts = %+v, want one polarity conflict", conflicts)
	}
	if conflicts[0].Kind != ConflictPolarity {
		t.Fatalf("conflict kind = %q, want %q", conflicts[0].Kind, ConflictPolarity)
	}
	if conflicts[0].Resolution != ResolutionUnresolved {
		t.Fatalf("resolution = %q, want %q: the program never settles a disagreement", conflicts[0].Resolution, ResolutionUnresolved)
	}
	if conflicts[0].ClaimID != "CL-1" {
		t.Fatalf("conflict claim = %q, want %q", conflicts[0].ClaimID, "CL-1")
	}
	wantRefs := []string{supporting.Ref(), contradicting.Ref()}
	slices.Sort(wantRefs)
	if !slices.Equal(conflicts[0].EvidenceIDs, wantRefs) {
		t.Fatalf("conflict evidence = %+v, want %+v sorted", conflicts[0].EvidenceIDs, wantRefs)
	}
	if claims[0].Status != ClaimUnresolved {
		t.Fatalf("status = %q, want %q: a contested claim is never grounded", claims[0].Status, ClaimUnresolved)
	}
	if !hasCode(findings, CodeClaimContradictoryEvidence) {
		t.Fatalf("findings = %+v, want %q", findings, CodeClaimContradictoryEvidence)
	}
}

func TestGroundClaimsKeepsCandidateOrderAndNumbersClaims(t *testing.T) {
	record := acmeRecord()
	candidates := []ClaimCandidate{
		{Section: "S-1", Text: "First.", Quotation: "migrated 27 customers", Evidence: []string{record.Ref()}},
		{Section: "S-2", Text: "Second."},
		{Section: "S-3", Text: "Third.", Quotation: "migrated 27 customers", Evidence: []string{record.Ref()}},
	}

	claims, _, _ := GroundClaims(Brief{Customer: "Acme"}, candidates, []Evidence{record})

	if len(claims) != 3 {
		t.Fatalf("claims = %+v, want three", claims)
	}
	for i, want := range []string{"CL-1", "CL-2", "CL-3"} {
		if claims[i].ID != want {
			t.Fatalf("claim %d id = %q, want %q", i, claims[i].ID, want)
		}
	}
	if claims[0].Section != "S-1" || claims[1].Section != "S-2" || claims[2].Section != "S-3" {
		t.Fatalf("claims lost their section: %+v", claims)
	}
	if claims[0].Status != ClaimGrounded || claims[1].Status != ClaimUnresolved || claims[2].Status != ClaimGrounded {
		t.Fatalf("statuses = %q, %q, %q", claims[0].Status, claims[1].Status, claims[2].Status)
	}
}
