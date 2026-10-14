package adapters

import (
	"context"
	"fmt"

	"github.com/GhandyP/grounded-commercial-proposals/internal/proposal"
)

// FileTemplateSource serves the section template the outline follows. An empty
// path means the built-in template.
type FileTemplateSource struct {
	sections []proposal.SectionCandidate
}

// NewFileTemplateSource loads the template from one JSON file. An empty path
// selects the built-in template, which is what a caller who has not written a
// template of their own gets.
func NewFileTemplateSource(path string) (*FileTemplateSource, error) {
	if path == "" {
		return &FileTemplateSource{sections: proposal.BuiltinTemplate()}, nil
	}

	sections, err := loadJSON(path, proposal.DecodeTemplate)
	if err != nil {
		return nil, fmt.Errorf("adapters: section template: %w", err)
	}
	if len(sections) == 0 {
		return nil, fmt.Errorf("adapters: section template: %s declares no section", path)
	}
	for i, section := range sections {
		if proposal.NormalizeText(section.Title) == "" {
			return nil, fmt.Errorf("adapters: section template: section %d has no title", i+1)
		}
	}
	return &FileTemplateSource{sections: sections}, nil
}

// Template returns the sections of the outline, in template order.
func (s *FileTemplateSource) Template(_ context.Context) ([]proposal.SectionCandidate, error) {
	return s.sections, nil
}
