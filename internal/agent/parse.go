package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/GhandyP/grounded-commercial-proposals/internal/proposal"
)

// ParseSections reads what the first stage returned.
//
// It accepts the wrapped object the output schema describes and a bare array,
// refuses unknown fields, and refuses a section with no title. It never repairs
// and never invents a field: an answer that does not fit is an error the caller
// reports, not something this program patches.
func ParseSections(raw string) ([]proposal.SectionCandidate, error) {
	sections, err := decodeList[proposal.SectionCandidate](raw, "sections")
	if err != nil {
		return nil, fmt.Errorf("agent: sections: %w", err)
	}
	for i, section := range sections {
		if proposal.NormalizeText(section.Title) == "" {
			return nil, fmt.Errorf("agent: section %d has no title", i+1)
		}
	}
	return sections, nil
}

// ParseClaims reads what the second stage returned. It accepts the wrapped
// object the output schema describes and a bare array, refuses unknown fields,
// and refuses a claim with no text. The returned candidates carry no status at
// all: whether a claim is grounded is decided in internal/proposal.
func ParseClaims(raw string) ([]proposal.ClaimCandidate, error) {
	claims, err := decodeList[proposal.ClaimCandidate](raw, "claims")
	if err != nil {
		return nil, fmt.Errorf("agent: claims: %w", err)
	}
	for i, claim := range claims {
		if proposal.NormalizeText(claim.Text) == "" {
			return nil, fmt.Errorf("agent: claim %d has no text", i+1)
		}
	}
	return claims, nil
}

// decodeList reads either the wrapped object {"<key>": [...]} or a bare JSON
// array, and refuses every field the caller did not ask for.
func decodeList[T any](raw, key string) ([]T, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("the answer is empty")
	}

	if strings.HasPrefix(trimmed, "[") {
		var items []T
		if err := decodeStrict(strings.NewReader(trimmed), &items); err != nil {
			return nil, fmt.Errorf("the answer is not an array: %w", err)
		}
		return items, nil
	}

	var envelope map[string]json.RawMessage
	if err := decodeStrict(strings.NewReader(trimmed), &envelope); err != nil {
		return nil, fmt.Errorf("the answer is not JSON: %w", err)
	}
	for field := range envelope {
		if field != key {
			return nil, fmt.Errorf("the answer carries an unexpected %q field", field)
		}
	}
	payload, ok := envelope[key]
	if !ok {
		return nil, fmt.Errorf("the answer has no %q field", key)
	}

	var items []T
	if err := decodeStrict(bytes.NewReader(payload), &items); err != nil {
		return nil, fmt.Errorf("the %q field is not an array: %w", key, err)
	}
	return items, nil
}

// decodeStrict reads exactly one JSON value and refuses unknown fields, so a
// model that answers with extra keys is refused instead of half understood.
func decodeStrict(r io.Reader, out any) error {
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	if err := decoder.Decode(new(json.RawMessage)); err != io.EOF {
		return fmt.Errorf("unexpected content after the first JSON value")
	}
	return nil
}
