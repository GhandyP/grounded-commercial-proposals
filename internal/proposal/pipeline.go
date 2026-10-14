package proposal

import (
	"context"
	"fmt"
	"slices"
)

// Pipeline runs the whole drafting path.
//
// It is the only place the sources, the model and the rules meet, and it
// decides nothing of its own: every judgement it reports comes from a rule in
// this package, and the model is reduced to the two small interfaces it is
// allowed to satisfy.
//
// A nil Outliner or Checker is not a degraded mode with different rules. The
// baseline path skips the model and still produces a draft: the template gives
// the outline, the brief's own needs become the claims, and because nothing was
// looked up they all come back unresolved. The rules are identical on both
// paths.
type Pipeline struct {
	// Evidence serves the approved records of the brief's customer. Required.
	Evidence ClaimSource
	// Pricing serves the read-only pricing catalogue. A nil source means the
	// draft carries no priced line.
	Pricing PricingSource
	// Template serves the section template. A nil source means the built-in one.
	Template TemplateSource
	// Outliner proposes the outline. Nil runs the deterministic path alone.
	Outliner Outliner
	// Checker proposes the claims. Nil runs the deterministic path alone.
	Checker Checker
	// RequestedPricing names the catalogue lines to offer. Empty means every
	// line of the customer's catalogue.
	RequestedPricing []string
}

// Build runs the path and returns the draft, or an error when a source fails.
//
// A rule that finds a problem is never an error: it becomes a finding, and the
// caller decides whether to stop. An error here means the program could not
// read what it needed, which is a different thing from a proposal with gaps.
func (p Pipeline) Build(ctx context.Context, brief Brief) (Draft, error) {
	customer := NormalizeText(brief.Customer)

	findings := BriefReadiness(brief)
	assumptions := AssumptionsFromBrief(brief)

	sections, err := p.outline(ctx, brief)
	if err != nil {
		return Draft{}, err
	}

	// Every record a source returns is handed to the rules, including one the
	// source should not have returned: that is what makes a citation to another
	// customer's record reportable as exactly that, instead of looking like a
	// citation to nothing. Only this customer's records reach the draft.
	var (
		retrieved []Evidence
		carried   []Evidence
	)
	if customer != "" {
		retrieved, err = p.retrieve(ctx, brief)
		if err != nil {
			return Draft{}, err
		}
		findings = append(findings, ValidateEvidence(retrieved)...)
		for _, record := range retrieved {
			if NormalizeText(record.Customer) == customer {
				carried = append(carried, record)
			}
		}
	}
	SortEvidence(carried)

	candidates, err := p.claims(ctx, brief, sections, retrieved)
	if err != nil {
		return Draft{}, err
	}
	claims, conflicts, claimFindings := GroundClaims(brief, candidates, retrieved)
	findings = append(findings, claimFindings...)
	SortClaims(claims)
	linkSections(sections, claims)

	placeholders, pricingFindings := p.pricing(ctx, brief)
	findings = append(findings, pricingFindings...)

	draft := Draft{
		Brief:               brief,
		Mode:                p.mode(),
		Sections:            sections,
		Claims:              nonNil(claims),
		Assumptions:         nonNil(assumptions),
		PricingPlaceholders: nonNil(placeholders),
		Evidence:            nonNil(carried),
		Conflicts:           conflicts,
	}
	findings = append(findings, ValidateDraft(draft)...)
	SortFindings(findings)
	draft.Findings = nonNil(findings)

	return draft, nil
}

// mode records honestly whether a model took part.
func (p Pipeline) mode() Mode {
	if p.Outliner == nil && p.Checker == nil {
		return ModeBaseline
	}
	return ModeGrounded
}

// outline returns the sections of the draft: the ones the model proposed, or
// the template when there is no model.
func (p Pipeline) outline(ctx context.Context, brief Brief) ([]Section, error) {
	var candidates []SectionCandidate

	switch {
	case p.Outliner != nil:
		proposed, err := p.Outliner.Outline(ctx, brief)
		if err != nil {
			return nil, fmt.Errorf("drafting: the outline agent failed: %w", err)
		}
		candidates = proposed
	case p.Template == nil:
		candidates = BuiltinTemplate()
	default:
		templated, err := p.Template.Template(ctx)
		if err != nil {
			return nil, fmt.Errorf("drafting: the section template failed: %w", err)
		}
		candidates = templated
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("drafting: the outline has no section")
	}

	sections := make([]Section, 0, len(candidates))
	for i, candidate := range candidates {
		sections = append(sections, Section{
			ID:    "S-" + itoa(i+1),
			Title: candidate.Title,
			Body:  candidate.Body,
		})
	}
	return sections, nil
}

// retrieve reads the customer's approved records.
func (p Pipeline) retrieve(ctx context.Context, brief Brief) ([]Evidence, error) {
	if p.Evidence == nil {
		return nil, fmt.Errorf("drafting: an approved-record source is required")
	}
	records, err := p.Evidence.Evidence(ctx, brief.Customer)
	if err != nil {
		return nil, fmt.Errorf("drafting: the approved-record source failed: %w", err)
	}
	return records, nil
}

// claims returns the claim candidates: the ones the model proposed, or one per
// statement the brief itself demands when there is no model.
func (p Pipeline) claims(ctx context.Context, brief Brief, sections []Section, records []Evidence) ([]ClaimCandidate, error) {
	if p.Checker == nil {
		return baselineClaims(brief, sections), nil
	}
	proposed, err := p.Checker.Check(ctx, brief, sections, records)
	if err != nil {
		return nil, fmt.Errorf("drafting: the claim checker failed: %w", err)
	}
	return proposed, nil
}

// pricing turns the customer's catalogue into open placeholders.
//
// A catalogue that cannot be read is reported as a finding rather than as an
// error, because the rest of the draft is still worth showing and the caller
// can see, from a blocking code, that the priced lines are missing.
func (p Pipeline) pricing(ctx context.Context, brief Brief) ([]PricingPlaceholder, []Finding) {
	if p.Pricing == nil {
		return nil, nil
	}
	catalog, err := p.Pricing.Pricing(ctx, brief.Customer)
	if err != nil {
		return nil, []Finding{NewFinding(CodePricingLineNotCatalogued, "pricing.catalogue",
			"the pricing catalogue could not be read (%v), so no priced line is offered", err)}
	}
	findings := ValidatePricingCatalog(catalog)
	placeholders, pricingFindings := PricingPlaceholders(brief, p.RequestedPricing, catalog)
	return placeholders, append(findings, pricingFindings...)
}

// baselineClaims proposes one claim per statement the brief itself demands.
//
// Nothing is looked up, so every one of them comes back unresolved: the
// baseline path shows what the proposal would have to support, and says plainly
// that none of it is supported yet. Sections are assigned by position and a
// human moves them; without a model there is no judgement about where a
// statement belongs, and inventing one would be the thing this program exists
// to avoid.
func baselineClaims(brief Brief, sections []Section) []ClaimCandidate {
	var candidates []ClaimCandidate
	add := func(text string) {
		candidates = append(candidates, ClaimCandidate{
			Section: sectionAt(sections, len(candidates)),
			Text:    text,
		})
	}

	for _, need := range brief.Needs {
		if NormalizeText(need) == "" {
			continue
		}
		add(need)
	}
	for _, scope := range brief.Scope {
		if NormalizeText(scope) == "" {
			continue
		}
		add(scope)
	}
	return candidates
}

func sectionAt(sections []Section, index int) string {
	if len(sections) == 0 {
		return ""
	}
	return sections[index%len(sections)].ID
}

// linkSections records on each section which claims were placed in it, so a
// reviewer sees the claim next to the prose it belongs to.
func linkSections(sections []Section, claims []Claim) {
	bySection := map[string][]string{}
	for _, claim := range claims {
		if claim.Section == "" {
			continue
		}
		bySection[claim.Section] = append(bySection[claim.Section], claim.ID)
	}
	for i := range sections {
		ids := bySection[sections[i].ID]
		slices.Sort(ids)
		sections[i].ClaimIDs = ids
	}
}

// nonNil turns a nil slice into an empty one, so the draft always marshals a
// list as a list and a consumer never has to read null as "none".
func nonNil[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}
