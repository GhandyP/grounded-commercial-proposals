package agent

import (
	"strings"
	"testing"

	"github.com/GhandyP/grounded-commercial-proposals/internal/proposal"
	"google.golang.org/genai"
)

func TestInstructionStatesTheWholeModelContract(t *testing.T) {
	text := strings.ToLower(Instruction())

	want := []string{
		"character for character", // quotations are copied, not paraphrased
		"never invent",            // no invented capability, customer or reference
		"amount",                  // no amount anywhere
		"guarantee",               // no promise and no offer
		"empty evidence list",     // an unsupported claim is returned, not dropped
		"unresolved",              // and the program marks it
		"section field",           // every claim names the section it belongs to
		"named human approves it", // the draft is never called approved
	}
	for _, phrase := range want {
		if !strings.Contains(text, phrase) {
			t.Fatalf("Instruction() does not state %q", phrase)
		}
	}
}

func TestInstructionForbidsThePricingCatalogue(t *testing.T) {
	text := strings.ToLower(Instruction())
	for _, phrase := range []string{"no amount", "never state an amount"} {
		if strings.Contains(text, phrase) {
			return
		}
	}
	t.Fatal("Instruction() does not forbid stating an amount")
}

func TestSchemasCarryNoDecisionTheProgramOwns(t *testing.T) {
	schemas := map[string]*genai.Schema{
		"outline": OutlineSchema(),
		"check":   CheckSchema(),
	}
	forbidden := []string{"status", "amount", "price", "currency", "terms", "approver", "approvedBy", "value", "reason"}

	for name, schema := range schemas {
		for _, property := range forbidden {
			if propertyNamed(schema, property) {
				t.Fatalf("%s schema declares %q, which the program owns and the model must not be able to set", name, property)
			}
		}
	}
}

func TestSchemasDescribeOnlyWhatTheModelMayReturn(t *testing.T) {
	outline := OutlineSchema()
	if len(outline.Properties) != 1 {
		t.Fatalf("OutlineSchema() properties = %v, want only sections", propertyNames(outline))
	}
	if outline.Properties["sections"] == nil || outline.Properties["sections"].Items == nil {
		t.Fatal("OutlineSchema() has no sections array")
	}
	if got := propertyNames(outline.Properties["sections"].Items); len(got) != 2 {
		t.Fatalf("outline item properties = %v, want title and body", got)
	}

	check := CheckSchema()
	if len(check.Properties) != 1 {
		t.Fatalf("CheckSchema() properties = %v, want only claims", propertyNames(check))
	}
	item := check.Properties["claims"].Items
	if item == nil {
		t.Fatal("CheckSchema() has no claims array")
	}
	if got := len(item.Properties); got != 4 {
		t.Fatalf("claim item properties = %v, want section, text, quotation and evidence", propertyNames(item))
	}
	for _, required := range []string{"section", "text", "quotation", "evidence"} {
		if item.Properties[required] == nil {
			t.Fatalf("claim item has no %q property: %v", required, propertyNames(item))
		}
	}
}

func TestPromptsCarryTheBriefAndTheCitableRecords(t *testing.T) {
	brief := proposal.Brief{
		Customer:    "Acme",
		Needs:       []string{"cut the manual reconciliation work"},
		Scope:       []string{"billing migration"},
		Constraints: []string{"must land before Q3 close"},
	}
	sections := []proposal.Section{{ID: "S-1", Title: "Understanding of the need"}}
	records := []proposal.Evidence{{
		ID:       "C-014",
		Customer: "Acme",
		Kind:     proposal.EvidenceApprovedClaim,
		Source:   "claims/acme.json",
		Excerpt:  "We migrated 27 customers to the new platform in 2025.",
		Approved: true,
	}}

	outline := OutlinePrompt(brief)
	for _, want := range []string{"Acme", "cut the manual reconciliation work", "billing migration", "must land before Q3 close"} {
		if !strings.Contains(outline, want) {
			t.Fatalf("OutlinePrompt() does not carry %q", want)
		}
	}

	check := CheckPrompt(brief, sections, records)
	for _, want := range []string{"S-1", "claims/acme.json#C-014", "We migrated 27 customers to the new platform in 2025."} {
		if !strings.Contains(check, want) {
			t.Fatalf("CheckPrompt() does not carry %q", want)
		}
	}
}

func TestCheckPromptNeverOffersAnotherCustomersRecord(t *testing.T) {
	brief := proposal.Brief{Customer: "Acme"}
	records := []proposal.Evidence{{
		ID:       "C-9",
		Customer: "Beta",
		Source:   "claims/beta.json",
		Excerpt:  "Beta's confidential migration.",
	}}

	prompt := CheckPrompt(brief, nil, records)

	if strings.Contains(prompt, "Beta's confidential migration.") {
		t.Fatal("CheckPrompt() handed the model another customer's excerpt")
	}
	if strings.Contains(prompt, "claims/beta.json#C-9") {
		t.Fatal("CheckPrompt() handed the model another customer's reference")
	}
}

func propertyNamed(schema *genai.Schema, name string) bool {
	if schema == nil {
		return false
	}
	if _, ok := schema.Properties[name]; ok {
		return true
	}
	for _, child := range schema.Properties {
		if propertyNamed(child, name) {
			return true
		}
	}
	return propertyNamed(schema.Items, name)
}

func propertyNames(schema *genai.Schema) []string {
	if schema == nil {
		return nil
	}
	names := make([]string, 0, len(schema.Properties))
	for name := range schema.Properties {
		names = append(names, name)
	}
	return names
}
