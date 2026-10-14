// Command grounded-commercial-proposals drafts a commercial proposal from a
// supplied brief and approved claims, and writes the validated draft as JSON on
// stdout.
//
// This entry point exists from the first commit so the module builds and the
// usage contract is visible. The command itself is wired in a later work unit
// (T5 and T7 of odd/tasks/grounded-commercial-proposals.md); until then it
// reports that the command is not implemented and exits with the usage code.
package main

import (
	"fmt"
	"os"
)

// Exit codes of the command. They are part of the command contract.
const (
	exitOK         = 0
	exitFailure    = 1
	exitUsage      = 2
	exitValidation = 3
)

const usage = `usage: grounded-commercial-proposals [flags] brief.json

Reads one ProposalBrief JSON document, grounds every claim in approved evidence
belonging to that brief's own customer, validates the draft, and writes the
resulting ProposalDraft as JSON on stdout.

Flags:
  -evidence string   approved claim records, as a JSON file or a directory (required)
  -pricing string    read-only pricing catalogue JSON (default: no pricing lines)
  -template string   section template JSON (default: the built-in template)
  -baseline          run the deterministic path only and never call a model
  -model string      Gemini model name for the ADK agents (default gemini-flash-latest)

The command never writes to disk, never produces a price and never makes a
binding offer. Exit codes: 0 ok, 1 failure, 2 usage, 3 validation errors.
`

func main() {
	os.Exit(run(os.Args[1:]))
}

// run keeps main testable and returns the process exit code.
func run(args []string) int {
	_ = args
	fmt.Fprintln(os.Stderr, "grounded-commercial-proposals: the command is not implemented yet")
	fmt.Fprint(os.Stderr, usage)
	return exitUsage
}
