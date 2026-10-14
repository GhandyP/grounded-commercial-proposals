package proposal

// ValidateDraft is the last pass over the whole draft.
//
// Grounding decides each claim on its own; this pass checks the draft as a
// whole: a claim that is placed in no section or in a section that does not
// exist, a section citing a claim the draft does not carry, a status that is
// not one of the three accepted values, a claim that calls itself grounded
// while citing nothing, and a priced line that carries an amount instead of an
// open placeholder.
//
// It also re-checks the brief's readiness, so the two passes cannot drift: a
// draft assembled from an incomplete brief is reported here even if the caller
// forgot the earlier check.
//
// Findings are returned in the order they were found; the caller sorts them for
// output with SortFindings.
func ValidateDraft(draft Draft) []Finding {
	findings := BriefReadiness(draft.Brief)

	sectionIDs := make(map[string]bool, len(draft.Sections))
	for _, section := range draft.Sections {
		sectionIDs[section.ID] = true
	}
	claimIDs := make(map[string]bool, len(draft.Claims))
	for _, claim := range draft.Claims {
		claimIDs[claim.ID] = true
	}

	for _, claim := range draft.Claims {
		if !claim.Status.Valid() {
			findings = append(findings, NewFinding(CodeClaimStatusInvalid, claimSubject(claim),
				"claim %q has status %q, which is not %q, %q or %q", claim.ID, claim.Status, ClaimGrounded, ClaimUnresolved, ClaimRejected))
		}
		if claim.Status == ClaimGrounded && len(claim.EvidenceIDs) == 0 {
			findings = append(findings, NewFinding(CodeClaimStatusInvalid, claimSubject(claim),
				"claim %q is marked grounded but cites no evidence behind it", claim.ID))
		}
		switch {
		case claim.Section == "":
			findings = append(findings, NewFinding(CodeSectionClaimUnlinked, claimSubject(claim),
				"claim %q is not placed in any section, so a reviewer cannot see where it would appear", claim.ID))
		case !sectionIDs[claim.Section]:
			findings = append(findings, NewFinding(CodeSectionClaimUnlinked, claimSubject(claim),
				"claim %q names section %q, which is not in the draft", claim.ID, claim.Section))
		}
	}

	for _, section := range draft.Sections {
		for _, id := range section.ClaimIDs {
			if !claimIDs[id] {
				findings = append(findings, NewFinding(CodeSectionClaimUnlinked, section.ID,
					"section %q cites claim %q, which is not in the draft", section.ID, id))
			}
		}
	}

	for _, placeholder := range draft.PricingPlaceholders {
		if !IsPlaceholderValue(placeholder.Value) {
			findings = append(findings, NewFinding(CodePricingValueNotPlaceholder, placeholder.ID,
				"pricing placeholder %q carries %q; the amount stays open until a human fills it in, because a stated amount is a binding offer",
				placeholder.ID, placeholder.Value))
		}
	}

	return findings
}
