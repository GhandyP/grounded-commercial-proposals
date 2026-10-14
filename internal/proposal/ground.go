package proposal

import (
	"slices"
	"strings"
)

// GroundClaims decides the status of every proposed claim against the approved
// evidence of the brief's own customer.
//
// The model proposes text and citations; this function decides. A claim is
// grounded only when at least one cited record is approved, belongs to the
// brief's own customer, and contains the claim's quotation verbatim. A claim is
// rejected when the citation itself is unusable — it names nothing, it names
// another customer's record, or the claim states a promise a proposal must
// never make. Everything else stays unresolved and visible, because a claim
// that cannot be supported is reported, never dropped and never invented.
//
// Claims keep candidate order and are numbered CL-1, CL-2, ... The returned
// conflicts are the disagreements between cited records, which this function
// never resolves: a contested claim is downgraded to unresolved.
func GroundClaims(brief Brief, candidates []ClaimCandidate, records []Evidence) ([]Claim, []Conflict, []Finding) {
	customer := NormalizeText(brief.Customer)
	byRef := indexByRef(records)

	claims := make([]Claim, 0, len(candidates))
	var findings []Finding

	for i, candidate := range candidates {
		claim := Claim{
			ID:        "CL-" + itoa(i+1),
			Section:   strings.TrimSpace(candidate.Section),
			Text:      strings.TrimSpace(candidate.Text),
			Quotation: candidate.Quotation,
		}

		var reasons []string
		rejected := false
		cited := make([]Evidence, 0, len(candidate.Evidence))
		citedRefs := map[string]bool{}
		foreignRefs := map[string]bool{}

		for _, raw := range candidate.Evidence {
			ref := strings.TrimSpace(raw)
			rec, ok := byRef[ref]
			if !ok {
				rejected = true
				reason := "it cites a record that does not exist"
				reasons = append(reasons, reason)
				findings = append(findings, NewFinding(CodeClaimEvidenceUnknown, claim.ID,
					"claim %q cites %q, which is not one of the approved records of customer %q", claim.ID, ref, customer))
				continue
			}
			if NormalizeText(rec.Customer) != customer {
				// A record of another customer is never quoted, never copied and
				// never grounds anything here. The reference is kept only so the
				// author can find and remove the wrong citation.
				rejected = true
				foreignRefs[rec.Ref()] = true
				reasons = append(reasons, "it cites a record owned by another customer")
				findings = append(findings, NewFinding(CodeClaimEvidenceOtherCustomer, claim.ID,
					"claim %q cites a record owned by %q, and another customer's record never grounds this proposal", claim.ID, rec.Customer))
				continue
			}
			cited = append(cited, rec)
			citedRefs[rec.Ref()] = true
		}

		if forbidden := ForbiddenLanguage(claim.Text + "\n" + claim.Quotation); len(forbidden) > 0 {
			rejected = true
			reasons = append(reasons, "it states a promise a proposal must not make")
			findings = append(findings, NewFinding(CodeClaimForbiddenLanguage, claim.ID,
				"claim %q states %s; a proposal never promises or guarantees an outcome", claim.ID, describeList(forbidden)))
		}

		verbatim := false
		var grounding []string
		for _, rec := range cited {
			if ContainsVerbatim(rec.Excerpt, claim.Quotation) {
				verbatim = true
			}
			if !rec.Approved || len(ValidateEvidence([]Evidence{rec})) > 0 {
				reasons = append(reasons, "the cited record is not an approved, attributable record")
				findings = append(findings, NewFinding(CodeClaimEvidenceNotApproved, claim.ID,
					"record %q is not an approved, attributable record and cannot ground a claim", rec.Ref()))
				continue
			}
			if ContainsVerbatim(rec.Excerpt, claim.Quotation) {
				grounding = append(grounding, rec.Ref())
			}
		}
		if len(candidate.Evidence) > 0 && !verbatim {
			reasons = append(reasons, "no citation contains the quotation verbatim")
			findings = append(findings, NewFinding(CodeClaimQuotationNotVerbatim, claim.ID,
				"claim %q states no quotation that appears verbatim in a cited record, and an unverified quotation is never cited", claim.ID))
		}

		switch {
		case rejected:
			claim.Status = ClaimRejected
		case len(grounding) > 0:
			claim.Status = ClaimGrounded
		default:
			claim.Status = ClaimUnresolved
		}

		if claim.Status != ClaimGrounded {
			if len(candidate.Evidence) == 0 {
				reasons = append(reasons, "it cites no approved record")
				findings = append(findings, NewFinding(CodeClaimNoEvidence, claim.ID,
					"claim %q cites no approved record, so it stays unresolved and visible instead of being dropped", claim.ID))
			}
			claim.Reason = strings.Join(dedupe(reasons), "; ")
		}

		refs := make([]string, 0, len(citedRefs)+len(foreignRefs))
		for ref := range citedRefs {
			refs = append(refs, ref)
		}
		for ref := range foreignRefs {
			refs = append(refs, ref)
		}
		slices.Sort(refs)
		claim.EvidenceIDs = slices.Compact(refs)

		claims = append(claims, claim)
	}

	conflicts := DetectConflicts(claims, records)
	contested := map[string]bool{}
	for _, conflict := range conflicts {
		contested[conflict.ClaimID] = true
	}
	for i := range claims {
		if !contested[claims[i].ID] {
			continue
		}
		if claims[i].Status == ClaimGrounded {
			claims[i].Status = ClaimUnresolved
		}
		claims[i].Reason = strings.Join(dedupe(append([]string{claims[i].Reason}, ResolutionUnresolved)), "; ")
		findings = append(findings, NewFinding(CodeClaimContradictoryEvidence, claims[i].ID,
			"claim %q rests on approved records that disagree, and %s", claims[i].ID, ResolutionUnresolved))
	}

	SortFindings(findings)
	return claims, conflicts, findings
}

// dedupe removes repeated strings while keeping the first occurrence.
func dedupe(items []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}
