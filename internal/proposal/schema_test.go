package proposal

import (
	"strings"
	"testing"
)

func TestEvidenceRefIsStable(t *testing.T) {
	tests := []struct {
		name string
		in   Evidence
		want string
	}{
		{
			name: "source and id",
			in:   Evidence{Source: "claims/acme.json", ID: "C-014"},
			want: "claims/acme.json#C-014",
		},
		{
			name: "id only falls back to the id",
			in:   Evidence{ID: "C-014"},
			want: "C-014",
		},
		{
			name: "empty record has no reference",
			in:   Evidence{},
			want: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.in.Ref(); got != tc.want {
				t.Fatalf("Ref() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNormalizeTextCollapsesWhitespaceOnly(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "newlines and runs of spaces", in: "  We   delivered\n\tthe migration ", want: "We delivered the migration"},
		{name: "already normal", in: "27 countries", want: "27 countries"},
		{name: "empty stays empty", in: " \n ", want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeText(tc.in); got != tc.want {
				t.Fatalf("NormalizeText(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestContainsVerbatimIgnoresWhitespaceButNotCase(t *testing.T) {
	excerpt := "We migrated 27 customers\n to the new platform in 2025."

	tests := []struct {
		name  string
		quote string
		want  bool
	}{
		{name: "same words, different whitespace", quote: "migrated 27 customers to the new platform", want: true},
		{name: "whole excerpt", quote: excerpt, want: true},
		{name: "different case is not verbatim", quote: "we migrated 27 customers", want: false},
		{name: "invented number is not verbatim", quote: "migrated 28 customers", want: false},
		{name: "empty quote is never verbatim", quote: "", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ContainsVerbatim(excerpt, tc.quote); got != tc.want {
				t.Fatalf("ContainsVerbatim(%q, %q) = %v, want %v", excerpt, tc.quote, got, tc.want)
			}
		})
	}
}

func TestDecodeBriefRejectsUnknownFieldsAndMalformedInput(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{name: "valid brief", in: `{"customer":"Acme","needs":["one"]}`},
		{name: "unknown field", in: `{"customer":"Acme","budget":1000}`, wantErr: true},
		{name: "not json", in: `customer=Acme`, wantErr: true},
		{name: "trailing content", in: `{"customer":"Acme"} {"customer":"Beta"}`, wantErr: true},
		{name: "empty input", in: ``, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodeBrief(strings.NewReader(tc.in))
			if tc.wantErr != (err != nil) {
				t.Fatalf("DecodeBrief() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestDecodeBriefKeepsTheCustomer(t *testing.T) {
	brief, err := DecodeBrief(strings.NewReader(`{"customer":"Acme","needs":["cut cost"],"scope":["billing"],"constraints":["Q3"]}`))
	if err != nil {
		t.Fatalf("DecodeBrief() error = %v", err)
	}
	if brief.Customer != "Acme" {
		t.Fatalf("customer = %q, want %q", brief.Customer, "Acme")
	}
	if len(brief.Needs) != 1 || len(brief.Scope) != 1 || len(brief.Constraints) != 1 {
		t.Fatalf("brief lists not decoded: %+v", brief)
	}
}

func TestDecodeEvidenceRejectsUnknownAssertion(t *testing.T) {
	valid := `[{"id":"C-1","customer":"Acme","kind":"approved-claim","source":"s","excerpt":"e","approved":true,"approvedBy":"legal","approvedOn":"2026-01-02","assertion":"supports"}]`
	if _, err := DecodeEvidence(strings.NewReader(valid)); err != nil {
		t.Fatalf("DecodeEvidence() error = %v, want nil", err)
	}

	invalid := `[{"id":"C-1","customer":"Acme","kind":"approved-claim","source":"s","excerpt":"e","assertion":"maybe"}]`
	if _, err := DecodeEvidence(strings.NewReader(invalid)); err == nil {
		t.Fatal("DecodeEvidence() error = nil, want an error for an unknown assertion")
	}

	unknownKind := `[{"id":"C-1","customer":"Acme","kind":"brochure","source":"s","excerpt":"e","assertion":"supports"}]`
	if _, err := DecodeEvidence(strings.NewReader(unknownKind)); err == nil {
		t.Fatal("DecodeEvidence() error = nil, want an error for an unknown kind")
	}
}

func TestEveryFindingCodeHasASeverity(t *testing.T) {
	codes := FindingCodes()
	if len(codes) == 0 {
		t.Fatal("FindingCodes() is empty")
	}
	for _, code := range codes {
		sev, ok := SeverityOf(code)
		if !ok {
			t.Fatalf("SeverityOf(%q) is unknown; every exported code needs a severity", code)
		}
		if !sev.Valid() {
			t.Fatalf("SeverityOf(%q) = %q, which is not a valid severity", code, sev)
		}
	}
	if _, ok := SeverityOf("not.a.code"); ok {
		t.Fatal("SeverityOf() accepted an unknown code")
	}
}

func TestValidateEvidenceRequiresApproverOnApprovedRecords(t *testing.T) {
	base := Evidence{
		ID:        "C-1",
		Customer:  "Acme",
		Kind:      EvidenceApprovedClaim,
		Source:    "claims/acme.json",
		Excerpt:   "We migrated 27 customers.",
		Assertion: AssertionSupports,
	}

	tests := []struct {
		name string
		mut  func(Evidence) Evidence
		want string
	}{
		{name: "approved and attributable", mut: func(e Evidence) Evidence {
			e.Approved = true
			e.ApprovedBy = "legal@acme"
			e.ApprovedOn = "2026-01-02"
			return e
		}},
		{name: "approved without an approver", mut: func(e Evidence) Evidence {
			e.Approved = true
			e.ApprovedOn = "2026-01-02"
			return e
		}, want: CodeEvidenceApproverMissing},
		{name: "approved with a malformed date", mut: func(e Evidence) Evidence {
			e.Approved = true
			e.ApprovedBy = "legal@acme"
			e.ApprovedOn = "02/01/2026"
			return e
		}, want: CodeEvidenceApprovedOnInvalid},
		{name: "missing customer", mut: func(e Evidence) Evidence { e.Customer = ""; return e }, want: CodeEvidenceCustomerMissing},
		{name: "missing excerpt", mut: func(e Evidence) Evidence { e.Excerpt = "  "; return e }, want: CodeEvidenceExcerptMissing},
		{name: "unknown kind", mut: func(e Evidence) Evidence { e.Kind = "brochure"; return e }, want: CodeEvidenceKindInvalid},
		{name: "unknown assertion", mut: func(e Evidence) Evidence { e.Assertion = "maybe"; return e }, want: CodeEvidenceAssertionInvalid},
		{name: "missing id", mut: func(e Evidence) Evidence { e.ID = ""; return e }, want: CodeEvidenceIDMissing},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			findings := ValidateEvidence([]Evidence{tc.mut(base)})
			if tc.want == "" {
				if len(findings) != 0 {
					t.Fatalf("ValidateEvidence() = %+v, want no findings", findings)
				}
				return
			}
			if !hasCode(findings, tc.want) {
				t.Fatalf("ValidateEvidence() = %+v, want code %q", findings, tc.want)
			}
		})
	}
}

func TestValidatePricingCatalogRejectsUnusableLines(t *testing.T) {
	good := PricingLine{Customer: "Acme", Label: "Implementation fee", Currency: "USD", Terms: "net 30"}
	if findings := ValidatePricingCatalog([]PricingLine{good}); len(findings) != 0 {
		t.Fatalf("ValidatePricingCatalog() = %+v, want no findings", findings)
	}

	tests := []struct {
		name string
		mut  func(PricingLine) PricingLine
		want string
	}{
		{name: "unknown currency", mut: func(l PricingLine) PricingLine { l.Currency = "dollars"; return l }, want: CodePricingCurrencyInvalid},
		{name: "lower case currency", mut: func(l PricingLine) PricingLine { l.Currency = "usd"; return l }, want: CodePricingCurrencyInvalid},
		{name: "empty terms", mut: func(l PricingLine) PricingLine { l.Terms = " "; return l }, want: CodePricingTermsMissing},
		{name: "empty label", mut: func(l PricingLine) PricingLine { l.Label = ""; return l }, want: CodePricingLabelMissing},
		{name: "missing customer", mut: func(l PricingLine) PricingLine { l.Customer = ""; return l }, want: CodePricingCustomerMissing},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			findings := ValidatePricingCatalog([]PricingLine{tc.mut(good)})
			if !hasCode(findings, tc.want) {
				t.Fatalf("ValidatePricingCatalog() = %+v, want code %q", findings, tc.want)
			}
		})
	}
}

func TestSortersAreDeterministic(t *testing.T) {
	evidence := []Evidence{{ID: "b", Source: "s"}, {ID: "a", Source: "s"}}
	SortEvidence(evidence)
	if evidence[0].ID != "a" || evidence[1].ID != "b" {
		t.Fatalf("SortEvidence() = %+v, want a before b", evidence)
	}

	claims := []Claim{{ID: "z"}, {ID: "m"}}
	SortClaims(claims)
	if claims[0].ID != "m" || claims[1].ID != "z" {
		t.Fatalf("SortClaims() = %+v, want m before z", claims)
	}

	sections := []Section{{ID: "2"}, {ID: "1"}}
	SortSections(sections)
	if sections[0].ID != "1" || sections[1].ID != "2" {
		t.Fatalf("SortSections() = %+v, want 1 before 2", sections)
	}

	findings := []Finding{
		{Code: CodeClaimNoEvidence, Severity: SeverityWarning, Subject: "b"},
		{Code: CodeClaimNoEvidence, Severity: SeverityWarning, Subject: "a"},
		{Code: CodePricingValueNotPlaceholder, Severity: SeverityError, Subject: "a"},
	}
	SortFindings(findings)
	if findings[0].Severity != SeverityError {
		t.Fatalf("SortFindings() = %+v, want errors first", findings)
	}
	if findings[1].Subject != "a" || findings[2].Subject != "b" {
		t.Fatalf("SortFindings() = %+v, want a before b inside one code", findings)
	}
}

func TestNewFindingCarriesTheCodeSeverityAndSubject(t *testing.T) {
	f := NewFinding(CodeClaimNoEvidence, "CL-2", "claim %q cites nothing", "CL-2")
	if f.Code != CodeClaimNoEvidence {
		t.Fatalf("code = %q", f.Code)
	}
	if f.Severity != SeverityWarning {
		t.Fatalf("severity = %q, want %q", f.Severity, SeverityWarning)
	}
	if f.Subject != "CL-2" {
		t.Fatalf("subject = %q", f.Subject)
	}
	if !strings.Contains(f.Message, `claim "CL-2" cites nothing`) {
		t.Fatalf("message = %q", f.Message)
	}
}

func TestErrorsInReturnsOnlyErrors(t *testing.T) {
	findings := []Finding{
		{Code: CodeClaimNoEvidence, Severity: SeverityWarning, Subject: "a"},
		{Code: CodeClaimEvidenceOtherCustomer, Severity: SeverityError, Subject: "b"},
	}
	errs := ErrorsIn(findings)
	if len(errs) != 1 || errs[0].Subject != "b" {
		t.Fatalf("ErrorsIn() = %+v, want only the error finding", errs)
	}
	if len(ErrorsIn(nil)) != 0 {
		t.Fatal("ErrorsIn(nil) is not empty")
	}
}

// hasCode reports whether any finding carries the code.
func hasCode(findings []Finding, code string) bool {
	for _, f := range findings {
		if f.Code == code {
			return true
		}
	}
	return false
}
