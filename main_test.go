package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/GhandyP/grounded-commercial-proposals/internal/proposal"
)

// runCLI runs the command the way the process does, and returns its exit code
// and what it wrote to stdout.
func runCLI(t *testing.T, args ...string) (int, string) {
	t.Helper()
	var stdout bytes.Buffer
	code := run(context.Background(), args, &stdout)
	return code, stdout.String()
}

func baselineArgs(extra ...string) []string {
	args := []string{
		"-baseline",
		"-evidence", "testdata/evidence",
		"-template", "testdata/template.json",
		"-pricing", "testdata/pricing.json",
	}
	return append(args, extra...)
}

func TestRunBaselineWritesADraftAndExitsZero(t *testing.T) {
	code, stdout := runCLI(t, baselineArgs("testdata/brief-acme.json")...)
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d\nstdout: %s", code, exitOK, stdout)
	}

	var draft proposal.Draft
	if err := json.Unmarshal([]byte(stdout), &draft); err != nil {
		t.Fatalf("stdout is not a draft: %v\n%s", err, stdout)
	}
	if draft.Mode != proposal.ModeBaseline {
		t.Fatalf("mode = %q, want %q: the run never called a model and must say so", draft.Mode, proposal.ModeBaseline)
	}
	if len(draft.Sections) != 4 {
		t.Fatalf("sections = %+v, want the four template sections", draft.Sections)
	}
	if len(draft.Claims) == 0 {
		t.Fatal("the baseline run produced no claim, so an unsupported statement is invisible")
	}
	for _, claim := range draft.Claims {
		if claim.Status != proposal.ClaimUnresolved {
			t.Fatalf("claim %s status = %q, want %q", claim.ID, claim.Status, proposal.ClaimUnresolved)
		}
	}
	if len(draft.PricingPlaceholders) != 2 {
		t.Fatalf("placeholders = %+v, want the two Acme lines", draft.PricingPlaceholders)
	}
	for _, placeholder := range draft.PricingPlaceholders {
		if !proposal.IsPlaceholderValue(placeholder.Value) {
			t.Fatalf("placeholder %s value = %q, want an open amount", placeholder.ID, placeholder.Value)
		}
	}
}

func TestRunNeverCarriesAnotherCustomersRecord(t *testing.T) {
	code, stdout := runCLI(t, baselineArgs("testdata/brief-acme.json")...)
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}

	if strings.Contains(stdout, "confidential") {
		t.Fatal("another customer's excerpt reached the output")
	}

	var draft proposal.Draft
	if err := json.Unmarshal([]byte(stdout), &draft); err != nil {
		t.Fatalf("stdout is not a draft: %v", err)
	}
	for _, record := range draft.Evidence {
		if record.Customer != "Acme" {
			t.Fatalf("the draft carries a record of %q", record.Customer)
		}
	}
	if len(draft.Evidence) != 4 {
		t.Fatalf("evidence = %+v, want the four Acme records", draft.Evidence)
	}
}

func TestRunBlocksOnAMissingCustomerAndExitsWithTheValidationCode(t *testing.T) {
	code, stdout := runCLI(t, baselineArgs("testdata/brief-incomplete.json")...)
	if code != exitValidation {
		t.Fatalf("exit code = %d, want %d\nstdout: %s", code, exitValidation, stdout)
	}

	var draft proposal.Draft
	if err := json.Unmarshal([]byte(stdout), &draft); err != nil {
		t.Fatalf("stdout is not a draft: %v", err)
	}
	if len(proposal.ErrorsIn(draft.Findings)) == 0 {
		t.Fatalf("findings = %+v, want a blocking finding", draft.Findings)
	}
	if !hasFinding(draft.Findings, proposal.CodeBriefCustomerMissing) {
		t.Fatalf("findings = %+v, want %q", draft.Findings, proposal.CodeBriefCustomerMissing)
	}
}

func TestRunStillSucceedsWhenOnlyWarningsAreFound(t *testing.T) {
	code, stdout := runCLI(t, baselineArgs("testdata/brief-no-needs.json")...)
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d: a missing need is visible but does not block\nstdout: %s", code, exitOK, stdout)
	}

	var draft proposal.Draft
	if err := json.Unmarshal([]byte(stdout), &draft); err != nil {
		t.Fatalf("stdout is not a draft: %v", err)
	}
	if !hasFinding(draft.Findings, proposal.CodeBriefNeedMissing) {
		t.Fatalf("findings = %+v, want the missing need reported", draft.Findings)
	}
	if len(proposal.ErrorsIn(draft.Findings)) != 0 {
		t.Fatalf("findings = %+v, want no blocking finding", proposal.ErrorsIn(draft.Findings))
	}
	if len(draft.Assumptions) == 0 {
		t.Fatal("the draft assumed nothing about the missing need, so it invented it or ignored it")
	}
}

func TestRunRefusesToInventAPricedLine(t *testing.T) {
	code, stdout := runCLI(t, baselineArgs("-lines", "Success bonus", "testdata/brief-acme.json")...)
	if code != exitValidation {
		t.Fatalf("exit code = %d, want %d\nstdout: %s", code, exitValidation, stdout)
	}
	if !strings.Contains(stdout, proposal.CodePricingLineNotCatalogued) {
		t.Fatalf("stdout does not report %q", proposal.CodePricingLineNotCatalogued)
	}
}

func TestRunServesOnlyTheRequestedPricedLine(t *testing.T) {
	code, stdout := runCLI(t, baselineArgs("-lines", "Annual support", "testdata/brief-acme.json")...)
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d\nstdout: %s", code, exitOK, stdout)
	}

	var draft proposal.Draft
	if err := json.Unmarshal([]byte(stdout), &draft); err != nil {
		t.Fatalf("stdout is not a draft: %v", err)
	}
	if len(draft.PricingPlaceholders) != 1 || draft.PricingPlaceholders[0].Label != "Annual support" {
		t.Fatalf("placeholders = %+v, want only the requested line", draft.PricingPlaceholders)
	}
}

func TestRunRequiresEvidenceRecords(t *testing.T) {
	code, _ := runCLI(t, "-baseline", "testdata/brief-acme.json")

	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d: nothing is grounded without approved records", code, exitUsage)
	}
}

func TestRunRequiresExactlyOneBrief(t *testing.T) {
	code, _ := runCLI(t, "-baseline", "-evidence", "testdata/evidence")
	if code != exitUsage {
		t.Fatalf("exit code = %d with no brief, want %d", code, exitUsage)
	}
	code, _ = runCLI(t, "-baseline", "-evidence", "testdata/evidence", "testdata/brief-acme.json", "testdata/brief-no-needs.json")
	if code != exitUsage {
		t.Fatalf("exit code = %d with two briefs, want %d", code, exitUsage)
	}
}

func TestRunFailsOnAMissingBriefFile(t *testing.T) {
	code, _ := runCLI(t, "-baseline", "-evidence", "testdata/evidence", "testdata/absent.json")
	if code != exitFailure {
		t.Fatalf("exit code = %d, want %d", code, exitFailure)
	}
}

func TestRunFailsOnAUnknownFlag(t *testing.T) {
	code, _ := runCLI(t, "-baseline", "-evidence", "testdata/evidence", "-price", "1", "testdata/brief-acme.json")
	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
}

func TestRunWithoutTheBaselineFlagNeedsCredentials(t *testing.T) {
	t.Setenv("GOOGLE_API_KEY", "")

	code, stdout := runCLI(t, "-evidence", "testdata/evidence", "testdata/brief-acme.json")
	if code != exitFailure {
		t.Fatalf("exit code = %d, want %d: this environment has no key\nstdout: %s", code, exitFailure, stdout)
	}
}

func TestUsageNamesEveryFlag(t *testing.T) {
	for _, flagName := range []string{"-evidence", "-pricing", "-template", "-lines", "-baseline", "-model"} {
		if !strings.Contains(usage, flagName) {
			t.Fatalf("usage does not document %s", flagName)
		}
	}
}

func hasFinding(findings []proposal.Finding, code string) bool {
	for _, finding := range findings {
		if finding.Code == code {
			return true
		}
	}
	return false
}
