// Package adapters holds the offline implementations of the ports the proposal
// package declares.
//
// Everything here is a file on disk and nothing here talks to a network. The
// adapters exist so the domain boundary is visible in the code: where the
// approved records come from, where the pricing catalogue comes from and where
// the section template comes from. Swapping one of them for a real system is
// meant to be a matter of writing one more type that satisfies the same
// interface, with no change to a single rule.
//
// Two properties are deliberately structural rather than promised:
//
//   - A source serves one customer. Another customer's record never leaves the
//     source, and the program re-checks the customer of every record it is
//     handed anyway.
//   - The pricing catalogue has no setter and no amount field exists anywhere
//     in the contract, so no adapter can price a line.
package adapters

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/GhandyP/grounded-commercial-proposals/internal/proposal"
)

// FileClaimSource serves approved claim and case-study records out of one JSON
// file, or out of every *.json file of a directory.
type FileClaimSource struct {
	records []proposal.Evidence
}

// NewFileClaimSource loads the records from path. A directory is read in sorted
// file-name order, so two runs over the same directory produce the same record
// set.
func NewFileClaimSource(path string) (*FileClaimSource, error) {
	records, err := loadJSON(path, proposal.DecodeEvidence)
	if err != nil {
		return nil, fmt.Errorf("adapters: approved records: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("adapters: approved records: %s holds no record", path)
	}
	return &FileClaimSource{records: records}, nil
}

// Evidence returns the approved records of one customer, sorted by reference. A
// record owned by any other customer is never returned, even when it sits in
// the same file: serving one customer's records is this adapter's half of the
// confidentiality rule, and the program re-checks the customer of every record
// it receives.
func (s *FileClaimSource) Evidence(_ context.Context, customer string) ([]proposal.Evidence, error) {
	want := proposal.NormalizeText(customer)
	if want == "" {
		return nil, fmt.Errorf("adapters: a customer is required to read approved records")
	}

	var served []proposal.Evidence
	for _, record := range s.records {
		if proposal.NormalizeText(record.Customer) == want {
			served = append(served, record)
		}
	}
	proposal.SortEvidence(served)
	return served, nil
}

// loadJSON reads one JSON document, or every *.json document of a directory in
// sorted file-name order, through one of the strict decoders of the contract.
func loadJSON[T any](path string, decode func(io.Reader) ([]T, error)) ([]T, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return readJSON(path, decode)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		names = append(names, entry.Name())
	}
	slices.Sort(names)

	var all []T
	for _, name := range names {
		records, err := readJSON(filepath.Join(path, name), decode)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		all = append(all, records...)
	}
	return all, nil
}

// readJSON decodes one file through one of the strict decoders of the contract,
// so a mistyped fixture fails loudly instead of quietly dropping a field.
func readJSON[T any](path string, decode func(io.Reader) ([]T, error)) ([]T, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return decode(file)
}
