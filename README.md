# grounded-commercial-proposals

Draft a commercial proposal from a supplied brief and approved claims, and get a
draft where **every material claim cites approved evidence of that customer or is
marked unresolved**, every price is an open placeholder, and no sentence is a
binding offer.

The model is the last step, never the whole pipeline. Everything that can be
checked deterministically is plain Go with tests: the brief's readiness, the
binding of a claim to an approved record, the verbatim comparison of a
quotation, the detection of a promise, the pricing rules and the last pass over
the assembled draft. Two small ADK agents propose an outline and a list of
claims with citations, and a claim the rules cannot stand behind is reported
instead of being repaired, hidden or dropped.

Status: complete for the scope recorded in
[`odd/tasks/grounded-commercial-proposals.md`](odd/tasks/grounded-commercial-proposals.md).
Read the "What this repository is not" section before you point it at anything
that matters.

## What this repository guarantees

Every rule below is a pure function in `internal/proposal` with a test that pins
the code, the subject and the resulting status. A `warning` is visible and does
not block. An `error` blocks: the command writes the draft, reports the finding
and exits `3`.

| Code | Severity | What it means |
| --- | --- | --- |
| `brief.customer.missing` | error | The brief names no customer, so nothing can be scoped or grounded. |
| `brief.need.missing` | warning | The brief states no need. |
| `brief.scope.missing` | warning | The brief states no scope. |
| `brief.constraint.missing` | warning | The brief states no deadline, budget or legal limit. |
| `evidence.*` | error | An approved record is unusable: no id, no customer, no excerpt, an unknown kind or assertion, or a claim of approval with no approver or a date that is not `YYYY-MM-DD`. |
| `claim.no_evidence` | warning | The claim cites nothing, so it stays `unresolved` and visible. |
| `claim.evidence_unknown` | error | The claim cites a reference that names nothing. The claim is `rejected`. |
| `claim.evidence_other_customer` | error | The claim cites another customer's record. The claim is `rejected`, and the cited text is removed from the draft. |
| `claim.evidence_not_approved` | warning | The cited record is not approved and attributable, so it grounds nothing. |
| `claim.quotation_not_verbatim` | warning | No cited record contains the quotation character for character. Nothing is grounded without a verbatim quotation, so a claim that states none is never `grounded`. |
| `claim.forbidden_language` | error | The claim states a guarantee, an absolute or a superlative the business has not evidenced. The claim is `rejected`. |
| `claim.contradictory_evidence` | warning | The cited approved records disagree. The claim is downgraded to `unresolved`; the program never settles it. |
| `claim.status.invalid` | error | A status that is not `grounded`, `unresolved` or `rejected`, or a claim that calls itself grounded while citing nothing. |
| `section.claim_unlinked` | error | A claim is placed in no section or in a section that does not exist; or a section cites a claim the draft does not carry. |
| `pricing.customer.missing`, `pricing.label.missing`, `pricing.currency.invalid`, `pricing.terms.missing` | error | A catalogue line that cannot be offered. The currency is a closed list — see `proposal.AcceptedCurrencies()`. |
| `pricing.line_not_catalogued` | error | A requested priced line that the customer's catalogue does not carry. No one invents a line. |
| `pricing.line_other_customer` | error | A requested priced line exists only under another customer. |
| `pricing.value_not_a_placeholder` | error | A priced line carries an amount instead of an open placeholder. A stated amount is a binding offer. |

Three rules deserve their own sentence:

- **A claim is `grounded` only when** one cited record is approved, belongs to
  the brief's own customer, and contains the claim's quotation verbatim
  (whitespace normalized, case and punctuation exact).
- **A contradiction is never resolved.** A claim whose cited records argue both
  ways, or disagree on the same attribute, comes back `unresolved` with the
  disagreement named and the resolution string
  `unresolved: human review required`.
- **The pricing catalogue is read-only and has no amount field.** No type in
  this program can hold a price. A placeholder carries the label, currency and
  terms from the catalogue and a value of `{{label}}`, and stays that way until
  a human fills it in.

## Requirements

- Go 1.26.6 or newer (`GOTOOLCHAIN=auto` picks the cached toolchain; the module
  needs 1.26.6 because `google.golang.org/adk/v2 v2.6.0` does)
- `GOOGLE_API_KEY` for the model path only. The `-baseline` path runs offline
  with no credentials and no network.

## Quick start

The baseline path runs the whole deterministic pipeline and calls no model:

```bash
go run . -baseline \
  -evidence  testdata/evidence \
  -template  testdata/template.json \
  -pricing   testdata/pricing.json \
  testdata/brief-acme.json
```

Output (trimmed):

```json
{
  "brief": { "customer": "Acme", "needs": ["cut the manual reconciliation work"] },
  "mode": "baseline",
  "sections": [{ "id": "S-1", "title": "Understanding of the need", "claimIds": ["CL-1"] }],
  "claims": [
    { "id": "CL-1", "section": "S-1", "text": "cut the manual reconciliation work",
      "status": "unresolved", "reason": "it cites no approved record" }
  ],
  "pricingPlaceholders": [
    { "id": "P-1", "label": "Implementation fee", "currency": "USD",
      "terms": "net 30", "value": "{{Implementation fee}}" }
  ],
  "findings": [
    { "code": "claim.no_evidence", "severity": "warning", "subject": "CL-1",
      "message": "claim \"CL-1\" cites no approved record, so it stays unresolved and visible instead of being dropped" }
  ]
}
```

The model path needs a key:

```bash
export GOOGLE_API_KEY="your-key"
go run . -evidence testdata/evidence -template testdata/template.json \
  -pricing testdata/pricing.json testdata/brief-acme.json
```

## Command reference

```text
usage: grounded-commercial-proposals [flags] brief.json

Flags:
  -evidence string   approved claim records, as a JSON file or a directory (required)
  -pricing string    read-only pricing catalogue JSON (default: no priced line)
  -template string   section template JSON (default: the built-in template)
  -lines string      comma-separated pricing labels to offer (default: the whole catalogue)
  -baseline          run the deterministic path only and never call a model
  -model string      Gemini model name (default gemini-flash-latest, or ADK_MODEL)

Exit codes: 0 ok, 1 failure, 2 usage, 3 validation errors.
```

`-baseline` is not a debug flag: it is the honest way to see what the rules find
without a model. It uses the template for the outline and turns the brief's own
needs and scope items into claims, and because nothing was looked up they all
come back `unresolved`. The rules are identical on both paths; only `mode`
changes.

The command reads files, writes one JSON document to stdout, and stops. It never
writes to disk, never opens a socket except to the model, and never produces a
number it did not read from the catalogue.

## Input shapes

```jsonc
// brief.json
{ "customer": "Acme", "needs": ["..."], "scope": ["..."], "constraints": ["..."] }

// an approved record (evidence/*.json)
{ "id": "C-014", "customer": "Acme", "kind": "approved-claim", "source": "claims/acme.json",
  "excerpt": "We migrated 27 customers to the new platform in 2025.",
  "approved": true, "approvedBy": "delivery@acme", "approvedOn": "2026-01-02",
  "attribute": "delivery_time", "value": "six weeks", "assertion": "supports" }

// a pricing catalogue line
{ "customer": "Acme", "label": "Implementation fee", "currency": "USD", "terms": "net 30" }
```

Every file is decoded strictly: an unknown field is an error, not a silently
dropped requirement. `kind` is `approved-claim` or `case-study`; `assertion` is
`supports` or `contradicts`.

## What this repository is not

Say this out loud before you use it:

- **It has no export path.** There is no renderer, no document writer, no CRM
  adapter and no way to send anything anywhere. The fourth "done when" item of
  the source plan — external sharing requires sales and legal approval and stays
  dry-run first — is satisfied by absence, not by a gate: there is no door to
  open. Adding one means adding the approval gate with it.
- **The live Gemini call was not executed.** No API key was available in the
  environment this was built in. The agent layer is verified offline: the
  instruction, the output schemas, the parser, the refusals and the full ADK
  runner path against an injected fake model. What is *not* verified is how
  Gemini answers the real prompts. Treat the first live run as an experiment.
- **The forbidden-phrase list is a guardrail, not a proof.** A paraphrase it
  does not carry is a false negative, and a quotation from an approved record
  can be a false positive. Both are visible to the human reading the findings.
  The list is `proposal.ForbiddenPhrases()`; the code says so too.
- **The baseline claims are not claims.** They are the brief's own needs and
  scope items, surfaced so the gap is visible.
- **The whole approved-record set of a customer goes to the model.** A real
  deployment retrieves a subset. Nothing here chooses one.
- **It is a single-author example, not a service.** No concurrency, no retries,
  no storage, no rate limits and no authentication.

## How it is built

```text
main.go                 the command, its flags and its exit codes
internal/proposal/      the contract and every rule that needs no model
  schema.go             types, finding codes, ports, strict decoders
  brief.go              what the brief is missing
  ground.go             which claims have approved evidence of this customer
  rules.go              forbidden promises, contradictions, priced lines
  draft.go              the last pass over the assembled draft
  pipeline.go           the one place the sources, the model and the rules meet
internal/agent/         the only place a model is reached
adapters/               the offline JSON sources and the read-only catalogue
testdata/               the fixtures the end-to-end tests run on
odd/tasks/              the ODD record of how this was built
.legacy-prototype/      the original case files, kept as provenance
```

The dependency direction is one way: `internal/proposal` knows nothing about ADK,
Gemini, files or the command line. It declares five small interfaces and the
adapters and the agents satisfy them, which is why the whole rule set is
testable with no key, no network and no fixture on disk.

## Development

```bash
gofmt -l .
go vet ./...
go build ./...
go test ./... -count=1
```

## Provenance

This repository is a from-scratch production of case 39 of the author's ADK
cookbook. The original case files are preserved under
[`.legacy-prototype/`](.legacy-prototype/README.md): the legacy README is the
source plan the specs quote, and its legacy `main.go` called the cookbook's
private helper package, which is why the prototype never built outside the
cookbook.

## License

MIT — see [LICENSE](LICENSE).
