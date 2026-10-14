// Package proposal holds the typed contract of a commercial proposal and every
// rule that can be decided without a model.
//
// The division of labour is the point of this package. A model may propose an
// outline and a list of claims with citations; it never decides whether a claim
// is grounded, never sets a price, and never approves anything. Everything a
// machine can check deterministically lives here, in pure functions with tests:
//
//	brief.go   what the brief is missing before a draft can be written
//	ground.go  which claims have approved evidence of the same customer
//	rules.go   forbidden promises, priced placeholders, contradictions
//	draft.go   assembly and the last validation pass over the whole draft
//
// Only internal/agent talks to a model, and it does so through the two small
// interfaces declared at the end of this file.
package proposal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
)

// ---------------------------------------------------------------------------
// Enumerations
// ---------------------------------------------------------------------------

// Severity separates a draft that must not be shared from one that merely needs
// a human to look at it.
type Severity string

const (
	// SeverityError marks a draft that must not leave the machine.
	SeverityError Severity = "error"
	// SeverityWarning marks something a reviewer has to decide, which does not
	// by itself stop the draft.
	SeverityWarning Severity = "warning"
)

// Valid reports whether the severity is one of the two accepted values.
func (s Severity) Valid() bool {
	return s == SeverityError || s == SeverityWarning
}

// Mode records whether a draft was produced with a model or by the
// deterministic path alone.
type Mode string

const (
	// ModeGrounded means the outline and the claims came from the ADK agents.
	ModeGrounded Mode = "grounded"
	// ModeBaseline means the deterministic path ran alone: no model was called.
	ModeBaseline Mode = "baseline"
)

// Valid reports whether the mode is one of the two accepted values.
func (m Mode) Valid() bool { return m == ModeGrounded || m == ModeBaseline }

// EvidenceKind is the kind of approved record a claim may rest on.
type EvidenceKind string

const (
	// EvidenceApprovedClaim is a claim sales, legal or delivery already approved.
	EvidenceApprovedClaim EvidenceKind = "approved-claim"
	// EvidenceCaseStudy is an approved case study or customer reference.
	EvidenceCaseStudy EvidenceKind = "case-study"
)

// Valid reports whether the kind is one of the two accepted values.
func (k EvidenceKind) Valid() bool {
	return k == EvidenceApprovedClaim || k == EvidenceCaseStudy
}

// Assertion is the direction of an evidence record with respect to the claim
// text around it.
type Assertion string

const (
	// AssertionSupports means the record argues for the statement.
	AssertionSupports Assertion = "supports"
	// AssertionContradicts means the record argues against the statement.
	AssertionContradicts Assertion = "contradicts"
)

// Valid reports whether the assertion is one of the two accepted values.
func (a Assertion) Valid() bool { return a == AssertionSupports || a == AssertionContradicts }

// ClaimStatus is the decision the program reaches about a claim. A model never
// produces this value.
type ClaimStatus string

const (
	// ClaimGrounded means the claim cites at least one approved record of the
	// same customer and its quotation appears verbatim in that record.
	ClaimGrounded ClaimStatus = "grounded"
	// ClaimUnresolved means the claim is visible but not yet supported.
	ClaimUnresolved ClaimStatus = "unresolved"
	// ClaimRejected means the claim must not appear in a shared proposal.
	ClaimRejected ClaimStatus = "rejected"
)

// Valid reports whether the status is one of the three accepted values.
func (c ClaimStatus) Valid() bool {
	return c == ClaimGrounded || c == ClaimUnresolved || c == ClaimRejected
}

// ConflictKind is the shape of a disagreement between two evidence records.
type ConflictKind string

const (
	// ConflictPolarity means the cited records argue both ways on one claim.
	ConflictPolarity ConflictKind = "polarity"
	// ConflictAttribute means the cited records assert different values for
	// the same attribute.
	ConflictAttribute ConflictKind = "attribute"
)

// Valid reports whether the kind is one of the two accepted values.
func (k ConflictKind) Valid() bool { return k == ConflictPolarity || k == ConflictAttribute }

// ResolutionUnresolved is the only resolution this program ever writes. A
// disagreement between approved records is a human decision.
const ResolutionUnresolved = "unresolved: human review required"

// ---------------------------------------------------------------------------
// Finding codes
// ---------------------------------------------------------------------------

// Finding codes. They are part of the command contract: a caller may branch on
// them, so they are stable and documented in the README.
const (
	// Brief readiness.
	CodeBriefCustomerMissing   = "brief.customer.missing"
	CodeBriefNeedMissing       = "brief.need.missing"
	CodeBriefScopeMissing      = "brief.scope.missing"
	CodeBriefConstraintMissing = "brief.constraint.missing"

	// Evidence records.
	CodeEvidenceIDMissing         = "evidence.id.missing"
	CodeEvidenceCustomerMissing   = "evidence.customer.missing"
	CodeEvidenceExcerptMissing    = "evidence.excerpt.missing"
	CodeEvidenceKindInvalid       = "evidence.kind.invalid"
	CodeEvidenceAssertionInvalid  = "evidence.assertion.invalid"
	CodeEvidenceApproverMissing   = "evidence.approver.missing"
	CodeEvidenceApprovedOnInvalid = "evidence.approved_on.invalid"

	// Pricing catalogue.
	CodePricingCustomerMissing = "pricing.customer.missing"
	CodePricingLabelMissing    = "pricing.label.missing"
	CodePricingCurrencyInvalid = "pricing.currency.invalid"
	CodePricingTermsMissing    = "pricing.terms.missing"

	// Grounding.
	CodeClaimNoEvidence            = "claim.no_evidence"
	CodeClaimEvidenceUnknown       = "claim.evidence_unknown"
	CodeClaimEvidenceNotApproved   = "claim.evidence_not_approved"
	CodeClaimEvidenceOtherCustomer = "claim.evidence_other_customer"
	CodeClaimQuotationNotVerbatim  = "claim.quotation_not_verbatim"
	CodeClaimForbiddenLanguage     = "claim.forbidden_language"
	CodeClaimContradictoryEvidence = "claim.contradictory_evidence"
	CodeSectionClaimUnlinked       = "section.claim_unlinked"

	// Draft pricing.
	CodePricingLineNotCatalogued   = "pricing.line_not_catalogued"
	CodePricingLineOtherCustomer   = "pricing.line_other_customer"
	CodePricingValueNotPlaceholder = "pricing.value_not_a_placeholder"
)

// findingSeverity maps every finding code to its severity. Adding a code
// without adding it here fails TestEveryFindingCodeHasASeverity.
var findingSeverity = map[string]Severity{
	CodeBriefCustomerMissing:   SeverityError,
	CodeBriefNeedMissing:       SeverityWarning,
	CodeBriefScopeMissing:      SeverityWarning,
	CodeBriefConstraintMissing: SeverityWarning,

	CodeEvidenceIDMissing:         SeverityError,
	CodeEvidenceCustomerMissing:   SeverityError,
	CodeEvidenceExcerptMissing:    SeverityError,
	CodeEvidenceKindInvalid:       SeverityError,
	CodeEvidenceAssertionInvalid:  SeverityError,
	CodeEvidenceApproverMissing:   SeverityError,
	CodeEvidenceApprovedOnInvalid: SeverityError,

	CodePricingCustomerMissing: SeverityError,
	CodePricingLabelMissing:    SeverityError,
	CodePricingCurrencyInvalid: SeverityError,
	CodePricingTermsMissing:    SeverityError,

	CodeClaimNoEvidence:            SeverityWarning,
	CodeClaimEvidenceUnknown:       SeverityError,
	CodeClaimEvidenceNotApproved:   SeverityWarning,
	CodeClaimEvidenceOtherCustomer: SeverityError,
	CodeClaimQuotationNotVerbatim:  SeverityWarning,
	CodeClaimForbiddenLanguage:     SeverityError,
	CodeClaimContradictoryEvidence: SeverityWarning,
	CodeSectionClaimUnlinked:       SeverityError,

	CodePricingLineNotCatalogued:   SeverityError,
	CodePricingLineOtherCustomer:   SeverityError,
	CodePricingValueNotPlaceholder: SeverityError,
}

// FindingCodes returns every documented finding code in a stable order.
func FindingCodes() []string {
	codes := make([]string, 0, len(findingSeverity))
	for code := range findingSeverity {
		codes = append(codes, code)
	}
	slices.Sort(codes)
	return codes
}

// SeverityOf reports the severity of a finding code.
func SeverityOf(code string) (Severity, bool) {
	sev, ok := findingSeverity[code]
	return sev, ok
}

// ---------------------------------------------------------------------------
// The contract
// ---------------------------------------------------------------------------

// Brief is the input of the command: what a customer asked for, as supplied by
// the person drafting the proposal.
type Brief struct {
	Customer    string   `json:"customer"`
	Needs       []string `json:"needs,omitempty"`
	Scope       []string `json:"scope,omitempty"`
	Constraints []string `json:"constraints,omitempty"`
}

// Evidence is one approved record a claim may rest on. Customer is the record's
// owner, and the only customer allowed to cite it.
type Evidence struct {
	ID         string       `json:"id"`
	Customer   string       `json:"customer"`
	Kind       EvidenceKind `json:"kind"`
	Source     string       `json:"source"`
	Excerpt    string       `json:"excerpt"`
	Approved   bool         `json:"approved"`
	ApprovedBy string       `json:"approvedBy,omitempty"`
	ApprovedOn string       `json:"approvedOn,omitempty"`
	Attribute  string       `json:"attribute,omitempty"`
	Value      string       `json:"value,omitempty"`
	Assertion  Assertion    `json:"assertion"`
	Supersedes string       `json:"supersedes,omitempty"`
}

// Ref is the stable identity of the record, and the string a claim cites.
func (e Evidence) Ref() string {
	switch {
	case e.Source == "" && e.ID == "":
		return ""
	case e.Source == "":
		return e.ID
	case e.ID == "":
		return e.Source
	default:
		return e.Source + "#" + e.ID
	}
}

// SectionCandidate is an outline entry as a model may propose it. The body is
// prose the model wrote; it carries no claims of its own.
type SectionCandidate struct {
	Title string `json:"title"`
	Body  string `json:"body,omitempty"`
}

// ClaimCandidate is a claim as a model may propose it: text plus citations. It
// carries no status, because no model decides whether a claim is grounded.
// Section is the identifier of the outline section the claim belongs to.
type ClaimCandidate struct {
	Section   string   `json:"section,omitempty"`
	Text      string   `json:"text"`
	Quotation string   `json:"quotation,omitempty"`
	Evidence  []string `json:"evidence,omitempty"`
}

// Section is one section of the drafted proposal.
type Section struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Body     string   `json:"body"`
	ClaimIDs []string `json:"claimIds,omitempty"`
}

// Claim is a decided claim: the text as it would appear, the quotation it rests
// on, and the status the program reached. Section is the outline section the
// claim belongs to, and must name a section of the same draft.
type Claim struct {
	ID          string      `json:"id"`
	Section     string      `json:"section,omitempty"`
	Text        string      `json:"text"`
	Quotation   string      `json:"quotation,omitempty"`
	Status      ClaimStatus `json:"status"`
	EvidenceIDs []string    `json:"evidenceIds,omitempty"`
	Reason      string      `json:"reason,omitempty"`
}

// Assumption is something the brief did not say and the draft must not invent.
type Assumption struct {
	ID                string `json:"id"`
	Text              string `json:"text"`
	Basis             string `json:"basis"`
	NeedsConfirmation bool   `json:"needsConfirmation"`
}

// PricingLine is one entry of the read-only pricing catalogue. It names a line
// the business has approved to offer; it never carries an amount, because an
// amount in a proposal is a binding offer.
type PricingLine struct {
	Customer string `json:"customer"`
	Label    string `json:"label"`
	Currency string `json:"currency"`
	Terms    string `json:"terms"`
}

// PricingPlaceholder is a priced line of the draft with the amount left for a
// human to fill in.
type PricingPlaceholder struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Currency string `json:"currency"`
	Terms    string `json:"terms"`
	Value    string `json:"value"`
}

// Finding is one observable result of a rule.
type Finding struct {
	Code     string   `json:"code"`
	Severity Severity `json:"severity"`
	Subject  string   `json:"subject"`
	Message  string   `json:"message"`
}

// Conflict is a disagreement between approved records that the program names
// and refuses to resolve.
type Conflict struct {
	Kind        ConflictKind `json:"kind"`
	ClaimID     string       `json:"claimId"`
	Attribute   string       `json:"attribute,omitempty"`
	ValueA      string       `json:"valueA,omitempty"`
	ValueB      string       `json:"valueB,omitempty"`
	EvidenceIDs []string     `json:"evidenceIds,omitempty"`
	Resolution  string       `json:"resolution"`
}

// Draft is the output of the command.
type Draft struct {
	Brief               Brief                `json:"brief"`
	Mode                Mode                 `json:"mode"`
	Sections            []Section            `json:"sections"`
	Claims              []Claim              `json:"claims"`
	Assumptions         []Assumption         `json:"assumptions"`
	PricingPlaceholders []PricingPlaceholder `json:"pricingPlaceholders"`
	Evidence            []Evidence           `json:"evidence"`
	Conflicts           []Conflict           `json:"conflicts,omitempty"`
	Findings            []Finding            `json:"findings"`
}

// ---------------------------------------------------------------------------
// Pure helpers shared by every rule
// ---------------------------------------------------------------------------

// NormalizeText collapses every run of whitespace to a single space and trims
// the ends. It is the only normalization used when comparing a quotation with
// an approved excerpt, so two spellings of the same words compare equal.
func NormalizeText(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// ContainsVerbatim reports whether quote appears in excerpt once whitespace is
// normalized. Comparison stays case sensitive and punctuation exact: a
// quotation the approved text does not contain is not verbatim.
func ContainsVerbatim(excerpt, quote string) bool {
	q := NormalizeText(quote)
	if q == "" {
		return false
	}
	return strings.Contains(NormalizeText(excerpt), q)
}

// NewFinding builds a finding with the severity registered for its code. An
// unregistered code is a programming error and reports as an error.
func NewFinding(code, subject, format string, args ...any) Finding {
	sev, ok := SeverityOf(code)
	if !ok {
		sev = SeverityError
	}
	return Finding{
		Code:     code,
		Severity: sev,
		Subject:  subject,
		Message:  fmt.Sprintf(format, args...),
	}
}

// ErrorsIn returns only the findings that block a shareable draft.
func ErrorsIn(findings []Finding) []Finding {
	var errs []Finding
	for _, f := range findings {
		if f.Severity == SeverityError {
			errs = append(errs, f)
		}
	}
	return errs
}

// SortFindings orders findings by severity, then code, then subject, then
// message, so two runs over the same input print the same list.
func SortFindings(findings []Finding) {
	slices.SortFunc(findings, func(a, b Finding) int {
		if c := rank(a.Severity) - rank(b.Severity); c != 0 {
			return c
		}
		if c := strings.Compare(a.Code, b.Code); c != 0 {
			return c
		}
		if c := strings.Compare(a.Subject, b.Subject); c != 0 {
			return c
		}
		return strings.Compare(a.Message, b.Message)
	})
}

func rank(s Severity) int {
	if s == SeverityError {
		return 0
	}
	return 1
}

// SortEvidence orders records by reference.
func SortEvidence(records []Evidence) {
	slices.SortFunc(records, func(a, b Evidence) int { return strings.Compare(a.Ref(), b.Ref()) })
}

// SortClaims orders claims by identifier.
func SortClaims(claims []Claim) {
	slices.SortFunc(claims, func(a, b Claim) int { return strings.Compare(a.ID, b.ID) })
}

// SortSections orders sections by identifier.
func SortSections(sections []Section) {
	slices.SortFunc(sections, func(a, b Section) int { return strings.Compare(a.ID, b.ID) })
}

// SortConflict lists the evidence of a conflict in a stable order.
func SortConflict(c *Conflict) { slices.Sort(c.EvidenceIDs) }

// ---------------------------------------------------------------------------
// Decoding
// ---------------------------------------------------------------------------

// DecodeBrief reads exactly one strict JSON brief. An unknown field is an
// error, because a mistyped fixture must fail loudly instead of silently
// dropping a requirement.
func DecodeBrief(r io.Reader) (Brief, error) {
	var brief Brief
	if err := decodeStrict(r, &brief); err != nil {
		return Brief{}, fmt.Errorf("brief: %w", err)
	}
	return brief, nil
}

// DecodeEvidence reads a strict JSON array of approved records and rejects
// records whose enumerations are not accepted values.
func DecodeEvidence(r io.Reader) ([]Evidence, error) {
	var records []Evidence
	if err := decodeStrict(r, &records); err != nil {
		return nil, fmt.Errorf("evidence: %w", err)
	}
	for i, rec := range records {
		if rec.Kind != "" && !rec.Kind.Valid() {
			return nil, fmt.Errorf("evidence: record %d has kind %q, which is not %q or %q", i, rec.Kind, EvidenceApprovedClaim, EvidenceCaseStudy)
		}
		if rec.Assertion != "" && !rec.Assertion.Valid() {
			return nil, fmt.Errorf("evidence: record %d has assertion %q, which is not %q or %q", i, rec.Assertion, AssertionSupports, AssertionContradicts)
		}
	}
	return records, nil
}

// DecodePricingCatalog reads a strict JSON array of approved pricing lines.
func DecodePricingCatalog(r io.Reader) ([]PricingLine, error) {
	var lines []PricingLine
	if err := decodeStrict(r, &lines); err != nil {
		return nil, fmt.Errorf("pricing catalogue: %w", err)
	}
	return lines, nil
}

// DecodeTemplate reads a strict JSON array of section candidates.
func DecodeTemplate(r io.Reader) ([]SectionCandidate, error) {
	var sections []SectionCandidate
	if err := decodeStrict(r, &sections); err != nil {
		return nil, fmt.Errorf("template: %w", err)
	}
	return sections, nil
}

// decodeStrict reads exactly one JSON value and refuses unknown fields, so a
// typo in a fixture can never pass unnoticed.
func decodeStrict(r io.Reader, out any) error {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return err
	}
	if err := dec.Decode(new(json.RawMessage)); err != io.EOF {
		return fmt.Errorf("unexpected content after the first JSON value")
	}
	return nil
}

// BuiltinTemplate is the section template used when no template file is given.
// It is deliberately small: a proposal that needs more sections should say so
// in a template a human wrote.
func BuiltinTemplate() []SectionCandidate {
	return []SectionCandidate{
		{Title: "Understanding of the need"},
		{Title: "Proposed scope"},
		{Title: "Assumptions and dependencies"},
		{Title: "Commercial terms"},
	}
}

// ---------------------------------------------------------------------------
// Contract shape validation
// ---------------------------------------------------------------------------

// ValidateEvidence reports the records that are unusable as approved evidence.
// A record that claims to be approved without naming who approved it, or with a
// date that is not YYYY-MM-DD, cannot ground anything.
func ValidateEvidence(records []Evidence) []Finding {
	var findings []Finding
	for i, rec := range records {
		subject := rec.Ref()
		if subject == "" {
			subject = fmt.Sprintf("evidence[%d]", i)
		}
		if strings.TrimSpace(rec.ID) == "" {
			findings = append(findings, NewFinding(CodeEvidenceIDMissing, subject, "record %d has no id", i))
		}
		if strings.TrimSpace(rec.Customer) == "" {
			findings = append(findings, NewFinding(CodeEvidenceCustomerMissing, subject, "record %q does not say which customer it belongs to", subject))
		}
		if NormalizeText(rec.Excerpt) == "" {
			findings = append(findings, NewFinding(CodeEvidenceExcerptMissing, subject, "record %q has no excerpt to quote", subject))
		}
		if !rec.Kind.Valid() {
			findings = append(findings, NewFinding(CodeEvidenceKindInvalid, subject, "record %q has kind %q, which is not %q or %q", subject, rec.Kind, EvidenceApprovedClaim, EvidenceCaseStudy))
		}
		if !rec.Assertion.Valid() {
			findings = append(findings, NewFinding(CodeEvidenceAssertionInvalid, subject, "record %q has assertion %q, which is not %q or %q", subject, rec.Assertion, AssertionSupports, AssertionContradicts))
		}
		if rec.Approved {
			if strings.TrimSpace(rec.ApprovedBy) == "" {
				findings = append(findings, NewFinding(CodeEvidenceApproverMissing, subject, "record %q is marked approved but names no approver", subject))
			}
			if !isISODate(rec.ApprovedOn) {
				findings = append(findings, NewFinding(CodeEvidenceApprovedOnInvalid, subject, "record %q has approvedOn %q, which is not YYYY-MM-DD", subject, rec.ApprovedOn))
			}
		}
	}
	SortFindings(findings)
	return findings
}

// ValidatePricingCatalog reports the catalogue lines that cannot be offered.
func ValidatePricingCatalog(lines []PricingLine) []Finding {
	var findings []Finding
	for i, line := range lines {
		subject := fmt.Sprintf("pricing[%d]", i)
		if strings.TrimSpace(line.Label) != "" {
			subject = line.Label
		}
		if strings.TrimSpace(line.Customer) == "" {
			findings = append(findings, NewFinding(CodePricingCustomerMissing, subject, "catalogue line %q does not say which customer it belongs to", line.Label))
		}
		if strings.TrimSpace(line.Label) == "" {
			findings = append(findings, NewFinding(CodePricingLabelMissing, subject, "catalogue entry %d has no label", i))
		}
		if !validCurrency(line.Currency) {
			findings = append(findings, NewFinding(CodePricingCurrencyInvalid, subject, "catalogue line %q has currency %q, which is not an accepted ISO-4217 code", line.Label, line.Currency))
		}
		if NormalizeText(line.Terms) == "" {
			findings = append(findings, NewFinding(CodePricingTermsMissing, subject, "catalogue line %q has no payment terms", line.Label))
		}
	}
	SortFindings(findings)
	return findings
}

// acceptedCurrencies is the closed set of currencies a proposal line may name.
// A currency outside this list is refused rather than formatted: currencies
// with a differing minor unit are a commercial decision, not a formatting one.
var acceptedCurrencies = []string{
	"ARS", "BRL", "CAD", "CHF", "CLP", "COP", "EUR", "GBP", "MXN", "PEN", "USD", "UYU",
}

// AcceptedCurrencies returns the currencies a pricing line may name.
func AcceptedCurrencies() []string {
	return slices.Clone(acceptedCurrencies)
}

func validCurrency(code string) bool { return slices.Contains(acceptedCurrencies, code) }

// isISODate reports whether s is exactly YYYY-MM-DD with a real calendar date.
func isISODate(s string) bool {
	if len(s) != 10 || s[4] != '-' || s[7] != '-' {
		return false
	}
	for i, r := range s {
		if i == 4 || i == 7 {
			continue
		}
		if r < '0' || r > '9' {
			return false
		}
	}
	month := s[5:7]
	day := s[8:10]
	if month < "01" || month > "12" {
		return false
	}
	if day < "01" || day > "31" {
		return false
	}
	return true
}

// ---------------------------------------------------------------------------
// Ports
// ---------------------------------------------------------------------------

// ClaimSource serves the approved claim and case-study records of one customer.
// An implementation must never return a record belonging to another customer,
// and the program checks that again on every record it is handed.
type ClaimSource interface {
	Evidence(ctx context.Context, customer string) ([]Evidence, error)
}

// PricingSource serves the read-only pricing catalogue of one customer. It can
// report which lines may be offered; it can never set an amount.
type PricingSource interface {
	Pricing(ctx context.Context, customer string) ([]PricingLine, error)
}

// TemplateSource serves the section template the outline follows.
type TemplateSource interface {
	Template(ctx context.Context) ([]SectionCandidate, error)
}

// Outliner proposes the proposal outline from the brief. Only a model-backed
// implementation exists outside tests.
type Outliner interface {
	Outline(ctx context.Context, brief Brief) ([]SectionCandidate, error)
}

// Checker proposes claims with citations for a drafted outline. The returned
// candidates carry no status: the package decides whether they are grounded.
type Checker interface {
	Check(ctx context.Context, brief Brief, sections []Section) ([]ClaimCandidate, error)
}
