# Legacy prototype (case 39)

This directory preserves the original case files this repository was rebuilt from.
The Go tool ignores dot directories, so nothing here is compiled.

- `README.legacy.md` — the case-39 source plan, kept verbatim. It is the plan the
  specs in `odd/tasks/grounded-commercial-proposals.md` quote.
- `main.go` — the original prototype. It called the private
  `github.com/GhandyP/adk-go-enterprise-cookbook/internal/adkhelper` package,
  which is not publicly resolvable, so this prototype only ever built inside the
  cookbook. It is kept as provenance, not as buildable code.

Nothing in this directory is part of the delivered command.
