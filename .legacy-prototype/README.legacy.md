# 39 — Commercial proposals from a brief

## Goal

Draft a grounded commercial proposal from supplied brief details and approved claims without inventing capabilities, prices, or customer evidence.

## Run the current prototype

From this case directory:

```bash
export GOOGLE_API_KEY="your-key"
go run .
```

`internal/adkhelper.Run` uses ADK Go v2.2.0, Gemini, and `gemini-flash-latest`; set `ADK_MODEL` to override the model.

Sample prompt:

```text
Draft a proposal outline from this brief and the approved claims.
```

## What exists now

- `main.go (exists)` sends brief text and claims to one focused Gemini ADK agent.
- The output is free-form; requirements, claims, pricing placeholders, assumptions, sources, and approval state are not typed.
- This is intentionally text-only: it does not retrieve approved case studies, calculate pricing, or submit a proposal.

## Missing elements

- **Contract:** Define `ProposalBrief{customer, needs[], scope, constraints}` -> `ProposalDraft{sections[], claims[], assumptions[], pricingPlaceholders[], evidence[]}`.
- **ADK workflow:** Add brief-to-outline and claim-checker sub-agents with schemas and a proposal session; quote/pricing tools stay read-only until approval.
- **Domain boundary:** Add approved-claim/case-study, pricing, template, CRM, and document-renderer adapters.
- **Fixtures and validation:** Cover missing requirements, forbidden claims, currency/term placeholders, contradictory evidence, and unsupported case studies.
- **Safety:** Require sales/legal approval before sharing externally; never make a binding offer or expose another customer’s data.

## Target structure

```text
39-commercial-proposals-from-brief/
├── main.go (exists)
├── README.md (exists)
├── workflow.go (next)
├── schema.go (next)
├── adapters/claims.go (next)
├── testdata/brief.json (next)
└── main_test.go (next)
```

## Implementation order

1. Define brief, requirement, claim, evidence, and draft schemas.
2. Add approved-claim retrieval and session-scoped grounding.
3. Validate assumptions, placeholders, currency, and forbidden claims.
4. Test incomplete briefs, contradictory evidence, and customer-data boundaries.
5. Add a reviewable renderer and approval-gated external export.

## Done when

- [ ] Every material claim links to approved evidence or is marked unresolved.
- [ ] Missing requirements and pricing placeholders are visible.
- [ ] Fixtures reject unsupported claims and cross-customer leakage.
- [ ] External sharing requires sales/legal approval and remains dry-run first.

## Next improvement

Create a typed proposal draft with approved-claim citations before adding pricing or rendering adapters.
