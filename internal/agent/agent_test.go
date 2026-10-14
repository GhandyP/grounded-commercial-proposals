package agent

import (
	"context"
	"errors"
	"iter"
	"strings"
	"testing"

	"github.com/GhandyP/grounded-commercial-proposals/internal/proposal"
	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

// fakeModel is a model.LLM that answers with a canned reply and records every
// request it was handed. It is what makes the whole ADK layer testable without
// a key, without a network and without a live model.
type fakeModel struct {
	reply    string
	requests []*model.LLMRequest
	calls    int
}

func (f *fakeModel) Name() string { return "fake-gemini" }

func (f *fakeModel) GenerateContent(_ context.Context, req *model.LLMRequest, _ bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		f.calls++
		f.requests = append(f.requests, req)
		yield(&model.LLMResponse{
			Content: genai.NewContentFromText(f.reply, "model"),
		}, nil)
	}
}

func acmeBrief() proposal.Brief {
	return proposal.Brief{
		Customer:    "Acme",
		Needs:       []string{"cut the manual reconciliation work"},
		Scope:       []string{"billing migration"},
		Constraints: []string{"must land before Q3 close"},
	}
}

func TestNewRequiresAModel(t *testing.T) {
	if _, err := New(context.Background(), Config{}); err == nil {
		t.Fatal("New() error = nil, want an error when no model is injected")
	}
}

func TestOutlineRunsTheAgentAndParsesTheAnswer(t *testing.T) {
	fake := &fakeModel{reply: `{"sections":[{"title":"Understanding of the need"},{"title":"Proposed scope"}]}`}
	agents, err := New(context.Background(), Config{Model: fake})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	sections, err := agents.Outline(context.Background(), acmeBrief())
	if err != nil {
		t.Fatalf("Outline() error = %v", err)
	}
	if len(sections) != 2 || sections[0].Title != "Understanding of the need" {
		t.Fatalf("Outline() = %+v", sections)
	}

	if fake.calls != 1 {
		t.Fatalf("the model was called %d times, want exactly once", fake.calls)
	}
	req := fake.requests[0]
	if req.Config == nil || req.Config.ResponseSchema == nil {
		t.Fatal("the outline agent sent no response schema, so the model is not constrained")
	}
	if req.Config.SystemInstruction == nil || !containsText(req.Config.SystemInstruction, "character for character") {
		t.Fatal("the outline agent sent no instruction, so the model was not told what it may do")
	}
}

func TestCheckRunsTheSecondStageInTheSameSession(t *testing.T) {
	fake := &fakeModel{reply: `{"claims":[{"section":"S-1","text":"We have migrated comparable customers before.","quotation":"migrated 27 customers","evidence":["claims/acme.json#C-014"]}]}`}
	agents, err := New(context.Background(), Config{Model: fake})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if agents.SessionID() != agents.SessionID() || agents.SessionID() == "" {
		t.Fatal("the agents do not share one stable session")
	}

	sections := []proposal.Section{{ID: "S-1", Title: "Understanding of the need"}}
	records := []proposal.Evidence{{
		ID:       "C-014",
		Customer: "Acme",
		Source:   "claims/acme.json",
		Excerpt:  "We migrated 27 customers to the new platform in 2025.",
		Approved: true,
	}}

	claims, err := agents.Check(context.Background(), acmeBrief(), sections, records)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if len(claims) != 1 {
		t.Fatalf("Check() = %+v, want one claim", claims)
	}
	if claims[0].Section != "S-1" || claims[0].Evidence[0] != "claims/acme.json#C-014" {
		t.Fatalf("Check() = %+v, want the section and the reference copied through", claims[0])
	}

	req := fake.requests[0]
	if !containsText(req.Config.SystemInstruction, "character for character") &&
		!containsAnyText(req.Contents, "character for character") {
		t.Fatal("the check agent never saw the grounding rules")
	}
	if !containsAnyText(req.Contents, "28") && !containsAnyText(req.Contents, "27") {
		t.Fatal("the check agent never saw the approved excerpt it has to quote")
	}
}

func TestAgentsRefuseAnUnusableModelAnswer(t *testing.T) {
	tests := []struct {
		name  string
		reply string
		check func(*Agents, *testing.T)
	}{
		{
			name:  "outline answer is not JSON",
			reply: `I cannot help with that.`,
			check: func(a *Agents, t *testing.T) {
				if _, err := a.Outline(context.Background(), acmeBrief()); err == nil {
					t.Fatal("Outline() error = nil, want a refusal")
				}
			},
		},
		{
			name:  "outline answer has a blank title",
			reply: `{"sections":[{"title":"  "}]}`,
			check: func(a *Agents, t *testing.T) {
				if _, err := a.Outline(context.Background(), acmeBrief()); err == nil {
					t.Fatal("Outline() error = nil, want a refusal")
				}
			},
		},
		{
			name:  "check answer invents a status",
			reply: `{"claims":[{"section":"S-1","text":"First.","status":"grounded"}]}`,
			check: func(a *Agents, t *testing.T) {
				if _, err := a.Check(context.Background(), acmeBrief(), nil, nil); err == nil {
					t.Fatal("Check() error = nil, want a refusal: the model may not set a status")
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			agents, err := New(context.Background(), Config{Model: &fakeModel{reply: tc.reply}})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			tc.check(agents, t)
		})
	}
}

func TestNewModelRequiresAKeyAndHonoursTheModelEnvironment(t *testing.T) {
	t.Setenv(APIKeyEnv, "")
	t.Setenv(ModelEnv, "")

	if _, err := NewModel(context.Background(), "gemini-flash-latest"); !errors.Is(err, ErrMissingAPIKey) {
		t.Fatalf("NewModel() error = %v, want ErrMissingAPIKey", err)
	}
}

func TestNewModelBuildsTheNamedModelWithoutACall(t *testing.T) {
	t.Setenv(APIKeyEnv, "offline-placeholder-key")
	t.Setenv(ModelEnv, "gemini-flash-latest")

	built, err := NewModel(context.Background(), "")
	if err != nil {
		t.Fatalf("NewModel() error = %v, want a constructed client", err)
	}
	if built == nil || built.Name() == "" {
		t.Fatal("NewModel() returned no named model")
	}
}

func containsText(content *genai.Content, want string) bool {
	if content == nil {
		return false
	}
	for _, part := range content.Parts {
		if part != nil && strings.Contains(part.Text, want) {
			return true
		}
	}
	return false
}

func containsAnyText(contents []*genai.Content, want string) bool {
	for _, content := range contents {
		if containsText(content, want) {
			return true
		}
	}
	return false
}
