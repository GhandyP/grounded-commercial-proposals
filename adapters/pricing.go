package adapters

import (
	"context"
	"fmt"

	"github.com/GhandyP/grounded-commercial-proposals/internal/proposal"
)

// FilePricingSource serves the read-only pricing catalogue of one customer.
//
// The type has no setter, and no amount field exists anywhere in the contract,
// so nothing in this program can turn a catalogue line into a price. Reading
// the catalogue is all a pricing source is for; the amount stays an open
// placeholder until a human fills it in.
type FilePricingSource struct {
	lines []proposal.PricingLine
}

// NewFilePricingSource loads the catalogue from one JSON file.
func NewFilePricingSource(path string) (*FilePricingSource, error) {
	lines, err := loadJSON(path, proposal.DecodePricingCatalog)
	if err != nil {
		return nil, fmt.Errorf("adapters: pricing catalogue: %w", err)
	}
	return &FilePricingSource{lines: lines}, nil
}

// Pricing returns the catalogue lines of one customer, in catalogue order.
func (s *FilePricingSource) Pricing(_ context.Context, customer string) ([]proposal.PricingLine, error) {
	want := proposal.NormalizeText(customer)
	if want == "" {
		return nil, fmt.Errorf("adapters: a customer is required to read a pricing catalogue")
	}

	var served []proposal.PricingLine
	for _, line := range s.lines {
		if proposal.NormalizeText(line.Customer) == want {
			served = append(served, line)
		}
	}
	return served, nil
}
