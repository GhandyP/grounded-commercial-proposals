package adapters

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/GhandyP/grounded-commercial-proposals/internal/proposal"
)

const pricingCatalogue = `[
  {"customer":"Acme","label":"Implementation fee","currency":"USD","terms":"net 30"},
  {"customer":"Beta","label":"Onboarding","currency":"EUR","terms":"net 60"},
  {"customer":"Acme","label":"Annual support","currency":"USD","terms":"12 months, net 30"}
]`

func TestFilePricingSourceServesOnlyTheRequestedCustomerInCatalogueOrder(t *testing.T) {
	source, err := NewFilePricingSource(writeFixture(t, "pricing.json", pricingCatalogue))
	if err != nil {
		t.Fatalf("NewFilePricingSource() error = %v", err)
	}

	lines, err := source.Pricing(context.Background(), "Acme")
	if err != nil {
		t.Fatalf("Pricing() error = %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("Pricing() = %+v, want the two Acme lines", lines)
	}
	if lines[0].Label != "Implementation fee" || lines[1].Label != "Annual support" {
		t.Fatalf("Pricing() = %+v, want catalogue order", lines)
	}
	for _, line := range lines {
		if line.Customer != "Acme" {
			t.Fatalf("Pricing() returned a line of %q", line.Customer)
		}
	}
}

func TestFilePricingSourceRefusesABlankCustomer(t *testing.T) {
	source, err := NewFilePricingSource(writeFixture(t, "pricing.json", pricingCatalogue))
	if err != nil {
		t.Fatalf("NewFilePricingSource() error = %v", err)
	}
	if _, err := source.Pricing(context.Background(), ""); err == nil {
		t.Fatal("Pricing() error = nil, want an error: an unscoped read is never allowed")
	}
}

func TestNewFilePricingSourceRefusesAMissingPath(t *testing.T) {
	if _, err := NewFilePricingSource(filepath.Join(t.TempDir(), "absent.json")); err == nil {
		t.Fatal("NewFilePricingSource() error = nil, want an error for a missing path")
	}
}

func TestNewFilePricingSourceRefusesAMalformedCatalogue(t *testing.T) {
	if _, err := NewFilePricingSource(writeFixture(t, "pricing.json", `[{"customer":"Acme","label":"Fee","currency":"USD","terms":"net 30","amount":5000}]`)); err == nil {
		t.Fatal("NewFilePricingSource() error = nil, want an error: an amount field does not exist in the contract")
	}
}

func TestFilePricingSourceSatisfiesThePort(t *testing.T) {
	source, err := NewFilePricingSource(writeFixture(t, "pricing.json", pricingCatalogue))
	if err != nil {
		t.Fatalf("NewFilePricingSource() error = %v", err)
	}
	var _ proposal.PricingSource = source

	// The only way to change a catalogue is to change the file. The type has no
	// setter, so a caller cannot price a line through this port.
	if _, ok := any(source).(interface{ SetPrice(string, string) }); ok {
		t.Fatal("FilePricingSource exposes a write path")
	}
}
