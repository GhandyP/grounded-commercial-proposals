# grounded-commercial-proposals

Objective: publish a from-scratch public GitHub portfolio repository that drafts a
commercial proposal from a supplied brief and approved claims without inventing
capabilities, prices, or customer evidence, then continue building it under ODD.
Local path: `/home/ghandy/Documents/39-commercial-proposals-from-brief` (module
`github.com/GhandyP/grounded-commercial-proposals`).
Source plan: the legacy case README (case 39 of the author's ADK cookbook),
preserved verbatim at `.legacy-prototype/README.legacy.md`.
Delivery strategy: bounded work-unit commits pushed to `main` of a new
single-author repository (no PR flow exists); each unit stays under the
~400-authored-line heuristic.
Test runner: `gofmt -l .`, `go vet ./...`, `go build ./...`, `go test ./... -count=1`.

## Specs

- S1 (repo): Public GitHub repository named `grounded-commercial-proposals` under the user's account `GhandyP`, created as a new repository inside the user's portfolio. User's exact words: "crea un repo publico que la idea es hacerlo desde el inicio pero para que sea un repo dentro de mi portafolio en github".
- S2 (stack): Build the repository from scratch with Go and Google ADK. User's exact words: "implementa el plan con go google adk , que sea un proyecto repo desde cero". Standalone module `github.com/GhandyP/grounded-commercial-proposals` depending on `google.golang.org/adk/v2` (verified latest cached `v2.6.0`, which requires `go 1.26.6`) and `google.golang.org/genai` (verified cached `v1.74.0`); no dependency on the legacy private `internal/adkhelper` package nor on `github.com/GhandyP/adk-go-enterprise-cookbook` (verified: not publicly resolvable).
- S3 (workflow): Continue the project under ODD. User's exact words: "luego continuar con crear en proyecto con odd".
- S4 (repo name): user's exact answer: "grounded-commercial-proposals".
- S5 (scope): user's exact answer: "Núcleo: contrato, validación, agentes, CLI, fakes". This delivery covers the typed `ProposalBrief`/`ProposalDraft` contract, the deterministic grounding and validation rules, the two ADK agents, the CLI, the fixtures and the tests, and the approved-claim, pricing and template adapters as ports with offline JSON implementations. Explicitly out of this delivery: the document renderer, and the approval gate with external export.
- S6 (license): user's exact answer: "MIT" — an MIT `LICENSE` file is part of this delivery.
- S7 (home repository hygiene): user's exact answer: "Sí, agregar la línea al .gitignore de ~" — add `Documents/39-commercial-proposals-from-brief/` to `/home/ghandy/.gitignore`, following the pattern already used for cases 07, 19, 21 and 37.
- S8 (goal, verbatim from the legacy README): "Draft a grounded commercial proposal from supplied brief details and approved claims without inventing capabilities, prices, or customer evidence."
- S9 (contract, verbatim from the legacy README): "**Contract:** Define `ProposalBrief{customer, needs[], scope, constraints}` -> `ProposalDraft{sections[], claims[], assumptions[], pricingPlaceholders[], evidence[]}`."
- S10 (ADK workflow, verbatim from the legacy README): "**ADK workflow:** Add brief-to-outline and claim-checker sub-agents with schemas and a proposal session; quote/pricing tools stay read-only until approval."
- S11 (domain boundary, verbatim from the legacy README): "**Domain boundary:** Add approved-claim/case-study, pricing, template, CRM, and document-renderer adapters." In-scope part for this delivery: the approved-claim/case-study source, the read-only pricing catalogue and the section template, each defined as a port with an offline JSON implementation. CRM and document-renderer adapters are out of this slice, as is any write path into a CRM.
- S12 (fixtures and validation, verbatim from the legacy README): "**Fixtures and validation:** Cover missing requirements, forbidden claims, currency/term placeholders, contradictory evidence, and unsupported case studies."
- S13 (safety, verbatim from the legacy README): "**Safety:** Require sales/legal approval before sharing externally; never make a binding offer or expose another customer's data." In-scope part for this delivery: no binding offer and no cross-customer exposure. The approval gate is out of scope because no external-sharing path exists in this slice.
- S14 (done when, verbatim from the legacy README): "Every material claim links to approved evidence or is marked unresolved." / "Missing requirements and pricing placeholders are visible." / "Fixtures reject unsupported claims and cross-customer leakage." / "External sharing requires sales/legal approval and remains dry-run first." The fourth item is satisfied by absence in this delivery: there is no export path to gate, and the README must say so rather than claim a gate that does not exist.
- S15 (honesty): no Gemini API key is available in this environment (`GOOGLE_API_KEY` unset), so the model path is verified offline (instruction, output schema, parser, validation, injected model) and the README states that the live Gemini call was not executed. No check is claimed that was not observed.
- S16 (implementation order, verbatim from the legacy README): "1. Define brief, requirement, claim, evidence, and draft schemas. 2. Add approved-claim retrieval and session-scoped grounding. 3. Validate assumptions, placeholders, currency, and forbidden claims. 4. Test incomplete briefs, contradictory evidence, and customer-data boundaries. 5. Add a reviewable renderer and approval-gated external export." Step 5 is deferred by the S5 scope decision and is not part of this delivery.

## Tasks

| ID | Specs | Route | Status | Evidence |
|---|---|---|---|---|
| T1 | S1, S2, S3, S4, S6, S7 | inline | pending | - |
| T2 | S9, S16 | inline | pending | - |
| T3 | S9, S12, S16 | delegated (writer A, background) | pending | - |
| T4 | S10, S11, S16 | delegated (writer B, background) | pending | - |
| T5 | S9, S10, S12 | inline | pending | - |
| T6 | S8, S12, S13 | inline | pending | - |
| T7 | S10, S13, S15 | inline | pending | - |
| T8 | S5, S11, S14 | inline | pending | - |
| T9 | S1..S16 | delegated (independent verifier) | pending | - |

## Log

- L1 (user request, verbatim): "explora este proyecto , crea un repo publico que la idea es hacerlo desde el inicio pero para que sea un repo dentro de mi portafolio en github , y luego continuar con crear en proyecto con odd , implementa el plan con go google adk , que sea un proyecto repo desde cero"
- L2 (user answers, verbatim, in one batch): "grounded-commercial-proposals" (repository name), "Núcleo: contrato, validación, agentes, CLI, fakes" (S5 scope), "MIT" (S6 license), "Sí, agregar la línea al .gitignore de ~" (S7).
- L3 (exploration): `/home/ghandy/Documents/39-commercial-proposals-from-brief` held only `README.md` (the case-39 source plan) and `main.go`. `main.go` imported the private `github.com/GhandyP/adk-go-enterprise-cookbook/internal/adkhelper`, which is not present locally and not publicly resolvable, so the prototype does not build here. The directory sits untracked inside the home-directory Git repository (`/home/ghandy/.git`), which has no commits, so the new project repository is initialized locally in place (same pattern as cases 04, 12, 21, 32 and 34).
- L4 (decision, stack): ADK Go v2 verified as `google.golang.org/adk/v2 v2.6.0` with `agent`, `agent/llmagent`, `model/gemini`, `runner`, `session` and `tool` packages present in the local module cache; its own `go.mod` requires `go 1.26.6` while the installed toolchain is `go1.26.4`, and cached toolchains go up to `go1.26.9`, so `GOTOOLCHAIN=auto` resolves it offline. `google.golang.org/genai v1.74.0` is cached. The legacy README's "ADK Go v2.2.0" claim is stale and is not repeated.
- L5 (decision, provenance): the legacy case files are preserved under `.legacy-prototype/` (a dot directory, which the Go tool ignores, so `go build ./...` never compiles the unusable legacy `main.go`): `README.legacy.md` is the source plan quoted by the specs, and `main.go` stays as the prototype that referenced the private helper.
- L6 (delivery strategy): every chained-PR option assumes an existing repository with a PR flow; this is a brand-new single-author public repository, so the scaffold and each later work unit are committed and pushed directly to `main`.
- L7 (decision, model boundary): everything that can be checked without credentials is a pure function — brief readiness, evidence binding, forbidden-language detection, placeholder and currency validation, contradiction reporting and draft assembly. Only `internal/agent` talks to a model, and the model is reached through small interfaces (`Outliner`, `Checker`) so the whole pipeline is testable offline with fakes.
- L8 (decision, grounding rule): the model proposes claim text and an evidence reference; deterministic Go decides the claim status. A claim is `grounded` only when at least one cited evidence record is approved, belongs to the brief's own customer, and the claim's quotation appears verbatim in that record's excerpt. Otherwise the claim is `unresolved`, or `rejected` when the citation itself is invalid.
- L9 (decision, contradiction rule): following the case-21 idiom, a contradiction is never resolved by the program. A claim whose cited evidence asserts both `supports` and `contradicts`, or whose cited records disagree on the same attribute, becomes a reported conflict with both sides named and the resolution string `unresolved: human review required`.
- L10 (decision, pricing rule): the pricing catalogue is read-only and the model may not invent a line. A pricing placeholder must name a catalogue entry for the brief's customer and must keep a placeholder value (`"{{label}}"`, `TBD` or `[to confirm]`); any numeric or currency-shaped value is an error, because a number in a proposal is a binding offer.
- L11 (routing, T3/T4): both units fire the Writer rule's parallelism reason (two independent units, disjoint edit surfaces, each heavier than a subagent start) and are launched together in the background against the frozen contract from T2.
- L12 (delivery scope, recorded): the S5 answer keeps the document renderer and the approval-gated export out of this delivery. The README therefore states plainly that the command has no export path, that no proposal leaves the machine, and that external sharing remains unimplemented rather than claiming a gate that does not exist.
