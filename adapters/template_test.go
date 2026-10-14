package adapters

import (
	"context"
	"testing"

	"github.com/GhandyP/grounded-commercial-proposals/internal/proposal"
)

func TestFileTemplateSourceFallsBackToTheBuiltinTemplate(t *testing.T) {
	source, err := NewFileTemplateSource("")
	if err != nil {
		t.Fatalf("NewFileTemplateSource() error = %v", err)
	}
	sections, err := source.Template(context.Background())
	if err != nil {
		t.Fatalf("Template() error = %v", err)
	}

	want := proposal.BuiltinTemplate()
	if len(sections) != len(want) {
		t.Fatalf("Template() = %+v, want the built-in template %+v", sections, want)
	}
	for i := range want {
		if sections[i].Title != want[i].Title {
			t.Fatalf("Template()[%d] = %q, want %q", i, sections[i].Title, want[i].Title)
		}
	}
}

func TestFileTemplateSourceReadsATemplateFile(t *testing.T) {
	path := writeFixture(t, "template.json", `[{"title":"Need"},{"title":"Scope","body":"What we will do."}]`)
	source, err := NewFileTemplateSource(path)
	if err != nil {
		t.Fatalf("NewFileTemplateSource() error = %v", err)
	}
	sections, err := source.Template(context.Background())
	if err != nil {
		t.Fatalf("Template() error = %v", err)
	}
	if len(sections) != 2 || sections[0].Title != "Need" || sections[1].Body != "What we will do." {
		t.Fatalf("Template() = %+v", sections)
	}
}

func TestNewFileTemplateSourceRefusesAnUnusableTemplate(t *testing.T) {
	cases := map[string]string{
		"empty template":      `[]`,
		"blank title":         `[{"title":"  "}]`,
		"unknown json field":  `[{"title":"Need","claims":[]}]`,
		"not json":            `nonsense`,
		"a missing structure": `{"sections":[]}`,
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := NewFileTemplateSource(writeFixture(t, "template.json", content)); err == nil {
				t.Fatal("NewFileTemplateSource() error = nil, want a refused template")
			}
		})
	}
}

func TestNewFileTemplateSourceRefusesAMissingPath(t *testing.T) {
	if _, err := NewFileTemplateSource("/nonexistent/template.json"); err == nil {
		t.Fatal("NewFileTemplateSource() error = nil, want an error for a missing path")
	}
}

func TestFileTemplateSourceSatisfiesThePort(t *testing.T) {
	source, err := NewFileTemplateSource("")
	if err != nil {
		t.Fatalf("NewFileTemplateSource() error = %v", err)
	}
	var _ proposal.TemplateSource = source
}
