// Command grounded-commercial-proposals drafts a commercial proposal from a
// supplied brief and approved claims, and writes the validated draft as JSON on
// stdout.
//
// The command has no export path. It reads files, writes one JSON document to
// stdout, and stops. Nothing leaves the machine, and no external sharing is
// implemented: there is no gate to open because there is no door.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/GhandyP/grounded-commercial-proposals/adapters"
	"github.com/GhandyP/grounded-commercial-proposals/internal/agent"
	"github.com/GhandyP/grounded-commercial-proposals/internal/proposal"
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
  -pricing string    read-only pricing catalogue JSON (default: no priced line)
  -template string   section template JSON (default: the built-in template)
  -lines string      comma-separated pricing labels to offer (default: the whole catalogue)
  -baseline          run the deterministic path only and never call a model
  -model string      Gemini model name (default gemini-flash-latest, or ADK_MODEL)

The command never writes to disk, never produces a price and never makes a
binding offer. Exit codes: 0 ok, 1 failure, 2 usage, 3 validation errors.
`

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout))
}

// run keeps main testable and returns the process exit code.
func run(ctx context.Context, args []string, stdout io.Writer) int {
	flags := flag.NewFlagSet("grounded-commercial-proposals", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	var (
		evidencePath = flags.String("evidence", "", "approved claim records, as a JSON file or a directory (required)")
		pricingPath  = flags.String("pricing", "", "read-only pricing catalogue JSON")
		templatePath = flags.String("template", "", "section template JSON")
		lines        = flags.String("lines", "", "comma-separated pricing labels to offer")
		baseline     = flags.Bool("baseline", false, "run the deterministic path only and never call a model")
		modelName    = flags.String("model", "", "Gemini model name")
	)
	flags.Usage = func() { fmt.Fprint(os.Stderr, usage) }

	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	switch {
	case flags.NArg() != 1:
		fmt.Fprintln(os.Stderr, "grounded-commercial-proposals: exactly one brief file is required")
		fmt.Fprint(os.Stderr, usage)
		return exitUsage
	case strings.TrimSpace(*evidencePath) == "":
		fmt.Fprintln(os.Stderr, "grounded-commercial-proposals: -evidence is required: nothing is grounded without approved records")
		fmt.Fprint(os.Stderr, usage)
		return exitUsage
	}

	brief, err := readBrief(flags.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "grounded-commercial-proposals: %v\n", err)
		return exitFailure
	}

	pipeline, err := buildPipeline(ctx, *evidencePath, *pricingPath, *templatePath, *lines, *baseline, *modelName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "grounded-commercial-proposals: %v\n", err)
		return exitFailure
	}

	draft, err := pipeline.Build(ctx, brief)
	if err != nil {
		fmt.Fprintf(os.Stderr, "grounded-commercial-proposals: %v\n", err)
		return exitFailure
	}

	encoded, err := json.MarshalIndent(draft, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "grounded-commercial-proposals: %v\n", err)
		return exitFailure
	}
	fmt.Fprintf(stdout, "%s\n", encoded)

	if blocking := proposal.ErrorsIn(draft.Findings); len(blocking) > 0 {
		fmt.Fprintf(os.Stderr, "grounded-commercial-proposals: %d finding(s) block sharing this draft\n", len(blocking))
		for _, finding := range blocking {
			fmt.Fprintf(os.Stderr, "  %s %s: %s\n", finding.Severity, finding.Code, finding.Message)
		}
		return exitValidation
	}
	return exitOK
}

// readBrief loads the one brief the command drafts from.
func readBrief(path string) (proposal.Brief, error) {
	file, err := os.Open(path)
	if err != nil {
		return proposal.Brief{}, err
	}
	defer file.Close()
	return proposal.DecodeBrief(file)
}

// buildPipeline wires the sources and, unless the caller asked for the baseline
// path, the two ADK agents.
func buildPipeline(ctx context.Context, evidencePath, pricingPath, templatePath, lines string, baseline bool, modelName string) (proposal.Pipeline, error) {
	evidence, err := adapters.NewFileClaimSource(evidencePath)
	if err != nil {
		return proposal.Pipeline{}, err
	}
	template, err := adapters.NewFileTemplateSource(templatePath)
	if err != nil {
		return proposal.Pipeline{}, err
	}

	pipeline := proposal.Pipeline{
		Evidence:         evidence,
		Template:         template,
		RequestedPricing: splitLabels(lines),
	}
	if strings.TrimSpace(pricingPath) != "" {
		pricing, err := adapters.NewFilePricingSource(pricingPath)
		if err != nil {
			return proposal.Pipeline{}, err
		}
		pipeline.Pricing = pricing
	}
	if baseline {
		return pipeline, nil
	}

	model, err := agent.NewModel(ctx, modelName)
	if err != nil {
		return proposal.Pipeline{}, err
	}
	agents, err := agent.New(ctx, agent.Config{Model: model})
	if err != nil {
		return proposal.Pipeline{}, err
	}
	pipeline.Outliner = agents
	pipeline.Checker = agents
	return pipeline, nil
}

// splitLabels reads the -lines flag. An empty flag means the whole catalogue.
func splitLabels(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	labels := make([]string, 0, len(parts))
	for _, part := range parts {
		labels = append(labels, strings.TrimSpace(part))
	}
	return labels
}
