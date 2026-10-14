package agent

import (
	"fmt"
	"strings"

	"google.golang.org/genai"

	"github.com/GhandyP/grounded-commercial-proposals/internal/proposal"
)

// instruction is the whole contract handed to both agents.
//
// It is one instruction for both stages on purpose: the rules a model must not
// break are the same in the outline and in the claims, and the program re-checks
// every one of them anyway. The model is not asked to be careful; it is told
// exactly what it may produce and what will be refused.
const instruction = `You draft the outline and the claims of ONE commercial proposal, for an internal reviewer.

Rules:
- Propose sections and claims only. Everything else in the draft is written by the program, not by you.
- Copy every quotation character for character from the approved records you are given. A quotation a record does not contain is rejected, and the claim it was meant to support stays unresolved.
- Cite only the record references you were given, copied exactly. Never invent a reference, a customer, a case study, a capability, a date or an approval.
- Set the section field of every claim to the id of one of the supplied sections.
- Never state an amount, a price, a currency or a payment term. The pricing lines come from a read-only catalogue and the amount stays open until a human fills it in.
- Never state a guarantee, a promise or an offer, and never write that anything was approved, signed, delivered or is binding.
- When the records do not support a statement, still return the claim with an empty evidence list. The program marks it unresolved and a human decides. Dropping it hides the gap.
- Write plain sentences an account manager can review. No markdown, no tables, no commentary about yourself.
- You never approve anything. The proposal stays a draft until a named human approves it.`

// Instruction returns the agent instruction. Tests pin the grounding boundary,
// the no-invention and no-amount rules, and the draft marker on it.
func Instruction() string { return instruction }

// OutlinePrompt is the first stage: the brief in, an outline out.
func OutlinePrompt(brief proposal.Brief) string {
	var builder strings.Builder
	builder.WriteString("Draft the outline of one commercial proposal from this brief.\n\n")
	writeBrief(&builder, brief)
	builder.WriteString("\nReturn the sections only, in the order a reviewer should read them. ")
	builder.WriteString("Write no claim, no quotation and no amount.\n")
	return builder.String()
}

// CheckPrompt is the second stage: the brief, the outline and the citable
// records in, a list of claim candidates out.
//
// Only records belonging to the brief's own customer are rendered. The caller
// is expected to have scoped them already and this filter is the second half of
// the same rule, so one customer's excerpt can never reach a model on another
// customer's proposal.
func CheckPrompt(brief proposal.Brief, sections []proposal.Section, records []proposal.Evidence) string {
	customer := proposal.NormalizeText(brief.Customer)

	var builder strings.Builder
	builder.WriteString("Propose the claims of this proposal: one candidate per statement a reviewer has to be able to check.\n\n")
	writeBrief(&builder, brief)

	builder.WriteString("\nSections you may place a claim in:\n")
	if len(sections) == 0 {
		builder.WriteString("- (none)\n")
	}
	for _, section := range sections {
		fmt.Fprintf(&builder, "- %s: %s\n", section.ID, section.Title)
	}

	builder.WriteString("\nApproved records of this customer. The reference is the string in brackets and must be copied exactly. ")
	builder.WriteString("A quotation must come from the excerpt shown:\n")
	written := 0
	for _, record := range records {
		if proposal.NormalizeText(record.Customer) != customer {
			continue
		}
		fmt.Fprintf(&builder, "- [%s] %s\n  excerpt: %s\n", record.Ref(), record.Kind, record.Excerpt)
		written++
	}
	if written == 0 {
		builder.WriteString("- (none: every claim you propose will be unresolved)\n")
	}

	builder.WriteString("\nReturn the statements the brief requires, and for each one the section it belongs to, ")
	builder.WriteString("the quotation it rests on when a record supports it, and the references it cites. ")
	builder.WriteString("Return an empty evidence list rather than a statement the records do not support.\n")
	return builder.String()
}

// OutlineSchema describes the only thing the first stage may return.
func OutlineSchema() *genai.Schema {
	return &genai.Schema{
		Type:        genai.TypeObject,
		Description: "The sections of one commercial proposal.",
		Properties: map[string]*genai.Schema{
			"sections": {
				Type:        genai.TypeArray,
				Description: "The sections of the proposal, in the order a reviewer should read them.",
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"title": {
							Type:        genai.TypeString,
							Description: "Short section title, taken from the template when one was supplied.",
						},
						"body": {
							Type:        genai.TypeString,
							Description: "The prose of the section, or empty when the section is written from the brief instead.",
						},
					},
					Required: []string{"title"},
				},
			},
		},
		Required: []string{"sections"},
	}
}

// CheckSchema describes the only thing the second stage may return.
//
// There is no status, no amount, no currency, no payment term and no approver
// property, because the program supplies all of those and the model therefore
// cannot get them wrong.
func CheckSchema() *genai.Schema {
	return &genai.Schema{
		Type:        genai.TypeObject,
		Description: "The claim candidates of one commercial proposal.",
		Properties: map[string]*genai.Schema{
			"claims": {
				Type:        genai.TypeArray,
				Description: "One candidate per statement a reviewer has to be able to check.",
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"section": {
							Type:        genai.TypeString,
							Description: "Id of the section this claim belongs to, copied from the supplied sections.",
						},
						"text": {
							Type:        genai.TypeString,
							Description: "The statement as it would appear in the proposal.",
						},
						"quotation": {
							Type:        genai.TypeString,
							Description: "The span copied character for character from one cited record's excerpt. Empty when no record supports the claim.",
						},
						"evidence": {
							Type:        genai.TypeArray,
							Description: "References of the records this claim cites, copied exactly. Empty when no record supports the claim.",
							Items:       &genai.Schema{Type: genai.TypeString},
						},
					},
					Required: []string{"section", "text"},
				},
			},
		},
		Required: []string{"claims"},
	}
}

// writeBrief renders the brief as the model sees it. A list the brief does not
// state is rendered as such, so the model can see the gap instead of filling it.
func writeBrief(builder *strings.Builder, brief proposal.Brief) {
	fmt.Fprintf(builder, "Customer: %s\n", brief.Customer)
	writeList(builder, "Needs", brief.Needs)
	writeList(builder, "Scope", brief.Scope)
	writeList(builder, "Constraints", brief.Constraints)
}

func writeList(builder *strings.Builder, label string, items []string) {
	if len(items) == 0 {
		fmt.Fprintf(builder, "%s: (the brief states none)\n", label)
		return
	}
	fmt.Fprintf(builder, "%s:\n", label)
	for _, item := range items {
		fmt.Fprintf(builder, "- %s\n", item)
	}
}
