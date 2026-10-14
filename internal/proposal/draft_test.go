package proposal

import "testing"

// completeBrief is the brief every draft test starts from: nothing is missing,
// so a well-formed draft reports no findings at all.
func completeBrief() Brief {
	return Brief{
		Customer:    "Acme",
		Needs:       []string{"cut the manual reconciliation work"},
		Scope:       []string{"billing migration"},
		Constraints: []string{"must land before Q3 close"},
	}
}

func wellFormedDraft() Draft {
	return Draft{
		Brief: completeBrief(),
		Mode:  ModeGrounded,
		Sections: []Section{
			{ID: "S-1", Title: "Understanding of the need", ClaimIDs: []string{"CL-1"}},
			{ID: "S-2", Title: "Commercial terms"},
		},
		Claims: []Claim{{
			ID:          "CL-1",
			Section:     "S-1",
			Text:        "We have migrated comparable customers before.",
			Quotation:   "migrated 27 customers",
			Status:      ClaimGrounded,
			EvidenceIDs: []string{"claims/acme.json#C-014"},
		}},
		PricingPlaceholders: []PricingPlaceholder{{
			ID:       "P-1",
			Label:    "Implementation fee",
			Currency: "USD",
			Terms:    "net 30",
			Value:    "{{Implementation fee}}",
		}},
	}
}

func TestValidateDraftAcceptsAWellFormedDraft(t *testing.T) {
	draft := wellFormedDraft()
	if findings := ValidateDraft(draft); len(findings) != 0 {
		t.Fatalf("ValidateDraft() = %+v, want no findings", findings)
	}
}

func TestValidateDraftReportsTheBriefItCannotWriteFrom(t *testing.T) {
	draft := wellFormedDraft()
	draft.Brief = Brief{}

	findings := ValidateDraft(draft)

	if !hasCode(findings, CodeBriefCustomerMissing) {
		t.Fatalf("findings = %+v, want %q", findings, CodeBriefCustomerMissing)
	}
	if !hasCode(findings, CodeBriefNeedMissing) {
		t.Fatalf("findings = %+v, want %q", findings, CodeBriefNeedMissing)
	}
}

func TestValidateDraftRejectsAClaimWithNoSection(t *testing.T) {
	tests := []struct {
		name    string
		section string
	}{
		{name: "no section at all", section: ""},
		{name: "a section that does not exist", section: "S-9"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			draft := wellFormedDraft()
			draft.Claims[0].Section = tc.section
			draft.Sections[0].ClaimIDs = nil

			findings := ValidateDraft(draft)

			if !hasCode(findings, CodeSectionClaimUnlinked) {
				t.Fatalf("findings = %+v, want %q", findings, CodeSectionClaimUnlinked)
			}
			if findings[0].Severity != SeverityError {
				t.Fatalf("severity = %q, want %q", findings[0].Severity, SeverityError)
			}
		})
	}
}

func TestValidateDraftRejectsASectionCitingAnUnknownClaim(t *testing.T) {
	draft := wellFormedDraft()
	draft.Sections[1].ClaimIDs = []string{"CL-404"}

	findings := ValidateDraft(draft)

	if !hasCode(findings, CodeSectionClaimUnlinked) {
		t.Fatalf("findings = %+v, want %q", findings, CodeSectionClaimUnlinked)
	}
	if findings[0].Subject != "S-2" {
		t.Fatalf("subject = %q, want the section id", findings[0].Subject)
	}
}

func TestValidateDraftRejectsAPlaceholderCarryingAnAmount(t *testing.T) {
	for _, value := range []string{"5000", "USD 5,000.00", "", "around five thousand"} {
		t.Run(value, func(t *testing.T) {
			draft := wellFormedDraft()
			draft.PricingPlaceholders[0].Value = value

			findings := ValidateDraft(draft)

			if !hasCode(findings, CodePricingValueNotPlaceholder) {
				t.Fatalf("findings = %+v, want %q for value %q", findings, CodePricingValueNotPlaceholder, value)
			}
			if findings[0].Subject != "P-1" {
				t.Fatalf("subject = %q, want the placeholder id", findings[0].Subject)
			}
		})
	}
}

func TestValidateDraftRejectsAnUnknownClaimStatus(t *testing.T) {
	draft := wellFormedDraft()
	draft.Claims[0].Status = "probably"

	findings := ValidateDraft(draft)

	if len(findings) != 1 || findings[0].Severity != SeverityError {
		t.Fatalf("findings = %+v, want one error", findings)
	}
}

func TestValidateDraftRejectsAGroundedClaimWithNothingBehindIt(t *testing.T) {
	draft := wellFormedDraft()
	draft.Claims[0].EvidenceIDs = nil

	findings := ValidateDraft(draft)

	if len(findings) != 1 || findings[0].Severity != SeverityError {
		t.Fatalf("findings = %+v, want one error", findings)
	}
}
