package proposal

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// forbiddenPhrases is the closed list of promises a commercial proposal must
// never make.
//
// The list covers three ways a proposal becomes something it is not: a
// guarantee or an absolute ("guarantee", "always", "100%"), a superlative the
// business has not evidenced ("world-class", "best-in-class"), and the words
// that turn a draft into an offer ("binding offer", "final offer").
//
// This is a guardrail, not a proof. A paraphrase it does not list is a false
// negative, and a quotation from an approved record can be a false positive;
// both are visible to the human who reads the findings.
var forbiddenPhrases = []string{
	"100%",
	"24/7",
	"99.9",
	"always",
	"best in class",
	"best-in-class",
	"binding offer",
	"final offer",
	"guarantee",
	"guaranteed",
	"guarantees",
	"industry-leading",
	"never fails",
	"no risk",
	"risk-free",
	"unbeatable",
	"unmatched",
	"warranted",
	"we promise",
	"world-class",
}

// ForbiddenPhrases returns the phrases a proposal must never say, sorted and
// without duplicates.
func ForbiddenPhrases() []string { return slices.Clone(forbiddenPhrases) }

// forbiddenMatcher pairs a canonical phrase with the way it is matched.
type forbiddenMatcher struct {
	phrase string
	lower  string
	// wordBounded is set when the phrase both starts and ends with a letter, so
	// it is matched on word boundaries and "always" cannot match inside another
	// word. A phrase that carries punctuation ("100%", "24/7") is matched as a
	// plain substring, because a word boundary cannot follow a punctuation mark.
	wordBounded bool
	boundary    *regexp.Regexp
}

var forbiddenMatchers = buildForbiddenMatchers()

func buildForbiddenMatchers() []forbiddenMatcher {
	out := make([]forbiddenMatcher, 0, len(forbiddenPhrases))
	for _, phrase := range forbiddenPhrases {
		m := forbiddenMatcher{phrase: phrase, lower: strings.ToLower(phrase)}
		first := rune(phrase[0])
		last := rune(phrase[len(phrase)-1])
		if unicode.IsLetter(first) && unicode.IsLetter(last) {
			m.wordBounded = true
			m.boundary = regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(phrase) + `\b`)
		}
		out = append(out, m)
	}
	return out
}

// ForbiddenLanguage returns the phrases from ForbiddenPhrases that appear in
// text, in sorted order and without duplicates. Matching is case insensitive.
func ForbiddenLanguage(text string) []string {
	if NormalizeText(text) == "" {
		return nil
	}
	lowered := strings.ToLower(text)

	var found []string
	for _, m := range forbiddenMatchers {
		if m.wordBounded {
			if m.boundary.MatchString(lowered) {
				found = append(found, m.phrase)
			}
			continue
		}
		if strings.Contains(lowered, m.lower) {
			found = append(found, m.phrase)
		}
	}
	slices.Sort(found)
	return found
}

// DetectConflicts reports every disagreement between the records a claim cites,
// and never resolves one.
//
// Two shapes matter. Records that argue both ways on one statement are a
// polarity conflict. Records that assert different values for the same
// attribute are an attribute conflict. Both name the records involved and carry
// ResolutionUnresolved, because a disagreement between approved records is a
// decision for a human.
func DetectConflicts(claims []Claim, records []Evidence) []Conflict {
	byRef := indexByRef(records)

	var conflicts []Conflict
	for _, claim := range claims {
		cited := citedRecords(claim, byRef)
		if len(cited) == 0 {
			continue
		}
		refs := refsOf(cited)

		if conflict, ok := polarityConflict(claim, cited, refs); ok {
			conflicts = append(conflicts, conflict)
		}
		conflicts = append(conflicts, attributeConflicts(claim, cited, refs)...)
	}

	slices.SortFunc(conflicts, func(a, b Conflict) int {
		if c := strings.Compare(a.ClaimID, b.ClaimID); c != 0 {
			return c
		}
		if c := strings.Compare(string(a.Kind), string(b.Kind)); c != 0 {
			return c
		}
		return strings.Compare(a.Attribute, b.Attribute)
	})
	return conflicts
}

func polarityConflict(claim Claim, cited []Evidence, refs []string) (Conflict, bool) {
	supports, contradicts := false, false
	for _, rec := range cited {
		switch rec.Assertion {
		case AssertionContradicts:
			contradicts = true
		case AssertionSupports:
			supports = true
		}
	}
	if !supports || !contradicts {
		return Conflict{}, false
	}
	return Conflict{
		Kind:        ConflictPolarity,
		ClaimID:     claim.ID,
		EvidenceIDs: refs,
		Resolution:  ResolutionUnresolved,
	}, true
}

func attributeConflicts(claim Claim, cited []Evidence, refs []string) []Conflict {
	values := map[string]map[string]bool{}
	for _, rec := range cited {
		attribute := NormalizeText(rec.Attribute)
		value := NormalizeText(rec.Value)
		if attribute == "" || value == "" {
			continue
		}
		if values[attribute] == nil {
			values[attribute] = map[string]bool{}
		}
		values[attribute][value] = true
	}

	attributes := make([]string, 0, len(values))
	for attribute := range values {
		attributes = append(attributes, attribute)
	}
	slices.Sort(attributes)

	var conflicts []Conflict
	for _, attribute := range attributes {
		distinct := make([]string, 0, len(values[attribute]))
		for value := range values[attribute] {
			distinct = append(distinct, value)
		}
		if len(distinct) < 2 {
			continue
		}
		slices.Sort(distinct)
		conflicts = append(conflicts, Conflict{
			Kind:        ConflictAttribute,
			ClaimID:     claim.ID,
			Attribute:   attribute,
			ValueA:      distinct[0],
			ValueB:      distinct[1],
			EvidenceIDs: refs,
			Resolution:  ResolutionUnresolved,
		})
	}
	return conflicts
}

// PricingPlaceholders turns requested pricing lines into placeholders with the
// amount left open for a human.
//
// The catalogue is the only source of a line: a requested label that the
// customer's catalogue does not carry is an error rather than a guess, and a
// label that only exists under a different customer is refused, so one
// customer's commercial terms never appear in another customer's proposal.
// Nothing here can produce an amount.
func PricingPlaceholders(brief Brief, requested []string, catalog []PricingLine) ([]PricingPlaceholder, []Finding) {
	customer := NormalizeText(brief.Customer)

	var findings []Finding
	labels := make([]string, 0, len(requested))
	seen := map[string]bool{}

	if len(requested) == 0 {
		for _, line := range catalog {
			if NormalizeText(line.Customer) != customer {
				continue
			}
			label := NormalizeText(line.Label)
			if label == "" || seen[label] {
				continue
			}
			seen[label] = true
			labels = append(labels, label)
		}
	} else {
		for _, raw := range requested {
			label := NormalizeText(raw)
			if label == "" {
				findings = append(findings, NewFinding(CodePricingLineNotCatalogued, "pricing.request",
					"a requested pricing line has no label, so nothing can be looked up"))
				continue
			}
			if seen[label] {
				continue
			}
			seen[label] = true
			labels = append(labels, label)
		}
	}

	placeholders := make([]PricingPlaceholder, 0, len(labels))
	for _, label := range labels {
		line, owned, foreign := lookupPricingLine(catalog, customer, label)
		switch {
		case owned:
			placeholders = append(placeholders, PricingPlaceholder{
				ID:       "P-" + itoa(len(placeholders)+1),
				Label:    line.Label,
				Currency: line.Currency,
				Terms:    line.Terms,
				Value:    PlaceholderValue(line.Label),
			})
		case foreign:
			findings = append(findings, NewFinding(CodePricingLineOtherCustomer, label,
				"pricing line %q belongs to another customer and never appears in this proposal", label))
		default:
			findings = append(findings, NewFinding(CodePricingLineNotCatalogued, label,
				"pricing line %q is not in this customer's read-only catalogue, and no one invents one", label))
		}
	}

	SortFindings(findings)
	return placeholders, findings
}

// lookupPricingLine resolves a label against the catalogue. owned means the
// customer has that line; foreign means only another customer has it.
func lookupPricingLine(catalog []PricingLine, customer, label string) (line PricingLine, owned, foreign bool) {
	want := strings.ToLower(label)
	for _, candidate := range catalog {
		if strings.ToLower(NormalizeText(candidate.Label)) != want {
			continue
		}
		if NormalizeText(candidate.Customer) == customer {
			return candidate, true, false
		}
		line, foreign = candidate, true
	}
	if foreign {
		return line, false, true
	}
	return PricingLine{}, false, false
}

// PlaceholderValue is the token written for a line whose amount a human must
// fill in.
func PlaceholderValue(label string) string { return "{{" + NormalizeText(label) + "}}" }

// IsPlaceholderValue reports whether a value is an unfilled placeholder rather
// than an amount. Anything else — an empty value, a number, a currency amount,
// or prose — is refused, because an amount in a proposal is a binding offer.
func IsPlaceholderValue(value string) bool {
	trimmed := NormalizeText(value)
	if trimmed == "TBD" || trimmed == "[to confirm]" {
		return true
	}
	if !strings.HasPrefix(trimmed, "{{") || !strings.HasSuffix(trimmed, "}}") {
		return false
	}
	return NormalizeText(strings.TrimSuffix(strings.TrimPrefix(trimmed, "{{"), "}}")) != ""
}

// indexByRef maps every referenceable record to itself.
func indexByRef(records []Evidence) map[string]Evidence {
	byRef := make(map[string]Evidence, len(records))
	for _, rec := range records {
		if ref := rec.Ref(); ref != "" {
			byRef[ref] = rec
		}
	}
	return byRef
}

// citedRecords resolves the references a claim cites, ignoring references that
// name nothing.
func citedRecords(claim Claim, byRef map[string]Evidence) []Evidence {
	cited := make([]Evidence, 0, len(claim.EvidenceIDs))
	for _, ref := range claim.EvidenceIDs {
		if rec, ok := byRef[ref]; ok {
			cited = append(cited, rec)
		}
	}
	return cited
}

// refsOf returns the sorted references of a record set.
func refsOf(records []Evidence) []string {
	refs := make([]string, 0, len(records))
	for _, rec := range records {
		if ref := rec.Ref(); ref != "" {
			refs = append(refs, ref)
		}
	}
	slices.Sort(refs)
	return slices.Compact(refs)
}

// claimSubject is the identifier a finding about a claim is filed under.
func claimSubject(claim Claim) string {
	if claim.ID == "" {
		return "claim"
	}
	return claim.ID
}

// describeList renders a phrase list for a finding message.
func describeList(items []string) string {
	if len(items) == 1 {
		return fmt.Sprintf("%q", items[0])
	}
	quoted := make([]string, 0, len(items))
	for _, item := range items {
		quoted = append(quoted, fmt.Sprintf("%q", item))
	}
	return strings.Join(quoted, ", ")
}
