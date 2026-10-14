package adapters

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/GhandyP/grounded-commercial-proposals/internal/proposal"
)

const acmeClaims = `[
  {"id":"C-2","customer":"Acme","kind":"approved-claim","source":"claims/acme.json","excerpt":"We cut reconciliations by half.","approved":true,"approvedBy":"legal@acme","approvedOn":"2026-01-02","assertion":"supports"},
  {"id":"C-1","customer":"Acme","kind":"case-study","source":"claims/acme.json","excerpt":"We delivered the billing migration.","approved":true,"approvedBy":"delivery@acme","approvedOn":"2026-01-05","assertion":"supports"}
]`

const betaClaims = `[
  {"id":"C-9","customer":"Beta","kind":"approved-claim","source":"claims/beta.json","excerpt":"Beta only.","approved":true,"approvedBy":"legal@beta","approvedOn":"2026-01-02","assertion":"supports"}
]`

func writeFixture(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing fixture %s: %v", name, err)
	}
	return path
}

func TestFileClaimSourceServesOnlyTheRequestedCustomer(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{"acme.json": acmeClaims, "beta.json": betaClaims} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}

	source, err := NewFileClaimSource(dir)
	if err != nil {
		t.Fatalf("NewFileClaimSource() error = %v", err)
	}
	records, err := source.Evidence(context.Background(), "Acme")
	if err != nil {
		t.Fatalf("Evidence() error = %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("Evidence() = %+v, want exactly the two Acme records", records)
	}
	for _, rec := range records {
		if rec.Customer != "Acme" {
			t.Fatalf("Evidence() returned a record of %q, and another customer's record never reaches the proposal", rec.Customer)
		}
	}
}

func TestFileClaimSourceIsDeterministic(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{"b.json": betaClaims, "a.json": acmeClaims} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}
	source, err := NewFileClaimSource(dir)
	if err != nil {
		t.Fatalf("NewFileClaimSource() error = %v", err)
	}

	first, err := source.Evidence(context.Background(), "Acme")
	if err != nil {
		t.Fatalf("Evidence() error = %v", err)
	}
	if len(first) != 2 || first[0].ID != "C-1" || first[1].ID != "C-2" {
		t.Fatalf("Evidence() = %+v, want the records sorted by reference", first)
	}
}

func TestFileClaimSourceReadsOneFile(t *testing.T) {
	source, err := NewFileClaimSource(writeFixture(t, "claims.json", acmeClaims))
	if err != nil {
		t.Fatalf("NewFileClaimSource() error = %v", err)
	}
	records, err := source.Evidence(context.Background(), "Acme")
	if err != nil {
		t.Fatalf("Evidence() error = %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("Evidence() = %+v, want two records", records)
	}
}

func TestFileClaimSourceRefusesABlankCustomer(t *testing.T) {
	source, err := NewFileClaimSource(writeFixture(t, "claims.json", acmeClaims))
	if err != nil {
		t.Fatalf("NewFileClaimSource() error = %v", err)
	}
	if _, err := source.Evidence(context.Background(), "   "); err == nil {
		t.Fatal("Evidence() error = nil, want an error: an unscoped read is never allowed")
	}
}

func TestNewFileClaimSourceRefusesAMissingPath(t *testing.T) {
	if _, err := NewFileClaimSource(filepath.Join(t.TempDir(), "absent.json")); err == nil {
		t.Fatal("NewFileClaimSource() error = nil, want an error for a missing path")
	}
}

func TestNewFileClaimSourceRefusesMalformedRecords(t *testing.T) {
	cases := map[string]string{
		"not json":           `nonsense`,
		"unknown kind":       `[{"id":"C-1","customer":"Acme","kind":"brochure","source":"s","excerpt":"e","assertion":"supports"}]`,
		"unknown assertion":  `[{"id":"C-1","customer":"Acme","kind":"approved-claim","source":"s","excerpt":"e","assertion":"maybe"}]`,
		"unknown json field": `[{"id":"C-1","customer":"Acme","kind":"approved-claim","source":"s","excerpt":"e","assertion":"supports","amount":"1000"}]`,
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := NewFileClaimSource(writeFixture(t, "claims.json", content)); err == nil {
				t.Fatal("NewFileClaimSource() error = nil, want a loudly rejected fixture")
			}
		})
	}
}

func TestFileClaimSourceSatisfiesThePort(t *testing.T) {
	source, err := NewFileClaimSource(writeFixture(t, "claims.json", acmeClaims))
	if err != nil {
		t.Fatalf("NewFileClaimSource() error = %v", err)
	}
	var _ proposal.ClaimSource = source
}
