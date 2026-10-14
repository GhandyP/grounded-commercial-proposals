package agent

import (
	"strings"
	"testing"

	"github.com/GhandyP/grounded-commercial-proposals/internal/proposal"
)

func TestParseSectionsAcceptsBothShapesAndKeepsOrder(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{
			name: "wrapped object",
			raw:  `{"sections":[{"title":"Understanding of the need"},{"title":"Proposed scope"}]}`,
			want: []string{"Understanding of the need", "Proposed scope"},
		},
		{
			name: "bare array",
			raw:  `[{"title":"Understanding of the need"},{"title":"Proposed scope"}]`,
			want: []string{"Understanding of the need", "Proposed scope"},
		},
		{
			name: "a body is carried",
			raw:  `{"sections":[{"title":"Scope","body":"What we will do."}]}`,
			want: []string{"Scope"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sections, err := ParseSections(tc.raw)
			if err != nil {
				t.Fatalf("ParseSections() error = %v", err)
			}
			if len(sections) != len(tc.want) {
				t.Fatalf("ParseSections() = %+v, want %d sections", sections, len(tc.want))
			}
			for i, title := range tc.want {
				if sections[i].Title != title {
					t.Fatalf("section %d title = %q, want %q", i, sections[i].Title, title)
				}
			}
		})
	}
}

func TestParseSectionsRefusesWhatItCannotTrust(t *testing.T) {
	cases := map[string]string{
		"not json":           `nonsense`,
		"empty":              ``,
		"blank title":        `{"sections":[{"title":"   "}]}`,
		"missing title":      `{"sections":[{"body":"text"}]}`,
		"unknown field":      `{"sections":[{"title":"Need","claims":["CL-1"]}]}`,
		"unknown root field": `{"sections":[],"status":"approved"}`,
		"wrong shape":        `{"sections":"Need"}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseSections(raw); err == nil {
				t.Fatal("ParseSections() error = nil, want a refusal")
			}
		})
	}
}

func TestParseClaimsAcceptsBothShapesAndKeepsOrder(t *testing.T) {
	wrapped := `{"claims":[
      {"section":"S-1","text":"First.","quotation":"We migrated 27 customers.","evidence":["claims/acme.json#C-014"]},
      {"section":"S-2","text":"Second."}
    ]}`
	bare := `[{"section":"S-1","text":"First."}]`

	claims, err := ParseClaims(wrapped)
	if err != nil {
		t.Fatalf("ParseClaims() error = %v", err)
	}
	if len(claims) != 2 {
		t.Fatalf("ParseClaims() = %+v, want two claims", claims)
	}
	if claims[0].Section != "S-1" || claims[0].Text != "First." {
		t.Fatalf("claim 0 = %+v", claims[0])
	}
	if claims[0].Quotation != "We migrated 27 customers." {
		t.Fatalf("claim 0 quotation = %q, want the quotation copied through unchanged", claims[0].Quotation)
	}
	if len(claims[0].Evidence) != 1 || claims[0].Evidence[0] != "claims/acme.json#C-014" {
		t.Fatalf("claim 0 evidence = %+v", claims[0].Evidence)
	}
	if claims[1].Quotation != "" || claims[1].Evidence != nil {
		t.Fatalf("claim 1 = %+v, want the absent fields left absent", claims[1])
	}

	bareClaims, err := ParseClaims(bare)
	if err != nil {
		t.Fatalf("ParseClaims() error = %v", err)
	}
	if len(bareClaims) != 1 || bareClaims[0].Text != "First." {
		t.Fatalf("ParseClaims() = %+v", bareClaims)
	}
}

func TestParseClaimsRefusesWhatItCannotTrust(t *testing.T) {
	cases := map[string]string{
		"not json":           `nonsense`,
		"blank text":         `{"claims":[{"section":"S-1","text":"   "}]}`,
		"missing text":       `{"claims":[{"section":"S-1"}]}`,
		"unknown field":      `{"claims":[{"section":"S-1","text":"First.","status":"grounded"}]}`,
		"unknown root field": `{"claims":[],"mode":"grounded"}`,
		"wrong shape":        `{"claims":"First."}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseClaims(raw); err == nil {
				t.Fatal("ParseClaims() error = nil, want a refusal")
			}
		})
	}
}

func TestParseClaimsNeverInventsAStatus(t *testing.T) {
	claims, err := ParseClaims(`{"claims":[{"section":"S-1","text":"First."}]}`)
	if err != nil {
		t.Fatalf("ParseClaims() error = %v", err)
	}
	// ClaimCandidate has no status field at all; this asserts the type the
	// parser returns cannot carry one.
	var _ = proposal.ClaimCandidate(claims[0])
	if strings.Contains(strings.ToLower(claims[0].Text), "approved") {
		t.Fatal("the parser wrote prose of its own")
	}
}
