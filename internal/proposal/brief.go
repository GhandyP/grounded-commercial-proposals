package proposal

// BriefReadiness reports what a brief is missing before a draft can be written.
//
// A missing customer is an error, because every record the draft may cite is
// scoped to a customer and there is nothing to scope to. A missing need, scope
// or constraint is a warning: the draft still gets written, and the gap stays
// visible in the assumptions and the findings.
//
// Findings are emitted in the fixed order customer, needs, scope, constraints,
// so two runs over the same brief produce the same list.
func BriefReadiness(brief Brief) []Finding {
	var findings []Finding

	if NormalizeText(brief.Customer) == "" {
		findings = append(findings, NewFinding(CodeBriefCustomerMissing, "brief.customer",
			"the brief does not say which customer the proposal is for"))
	}
	if !hasContent(brief.Needs) {
		findings = append(findings, NewFinding(CodeBriefNeedMissing, "brief.needs",
			"the brief states no need, so the proposal has nothing to answer"))
	}
	if !hasContent(brief.Scope) {
		findings = append(findings, NewFinding(CodeBriefScopeMissing, "brief.scope",
			"the brief states no scope, so the proposal cannot say what is included"))
	}
	if !hasContent(brief.Constraints) {
		findings = append(findings, NewFinding(CodeBriefConstraintMissing, "brief.constraints",
			"the brief states no constraint, so no deadline, budget or legal limit is known"))
	}

	return findings
}

// AssumptionsFromBrief lists what the brief did not state, so the draft marks it
// for a human instead of inventing it.
//
// The order is the same fixed order as BriefReadiness: customer, needs, scope,
// constraints. Every assumption needs confirmation, because an assumption the
// program made is exactly the thing a proposal must not quietly assert.
func AssumptionsFromBrief(brief Brief) []Assumption {
	var assumptions []Assumption
	add := func(text, basis string) {
		assumptions = append(assumptions, Assumption{
			ID:                assumptionID(len(assumptions) + 1),
			Text:              text,
			Basis:             basis,
			NeedsConfirmation: true,
		})
	}

	if NormalizeText(brief.Customer) == "" {
		add("The customer this proposal is addressed to.", "brief.customer is empty")
	}
	if !hasContent(brief.Needs) {
		add("The customer's needs are not stated in the brief.", "brief.needs is empty")
	}
	if !hasContent(brief.Scope) {
		add("The scope this proposal may cover is not stated in the brief.", "brief.scope is empty")
	}
	if !hasContent(brief.Constraints) {
		add("No deadline, budget or legal constraint is stated in the brief.", "brief.constraints is empty")
	}

	return assumptions
}

func assumptionID(n int) string { return "A-" + itoa(n) }

// hasContent reports whether a brief list holds at least one non-blank entry.
func hasContent(entries []string) bool {
	for _, entry := range entries {
		if NormalizeText(entry) != "" {
			return true
		}
	}
	return false
}

// itoa is a tiny integer formatter for the identifiers this package builds. It
// keeps the identifiers dependency free and identical on every run.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
