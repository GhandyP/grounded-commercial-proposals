// Package agent is the only place in this program that reaches a model.
//
// The package is deliberately thin. It builds two ADK agents, runs them in one
// session, and parses what they answer through its own types. It decides
// nothing: a model may propose an outline and a list of claims with citations,
// and every one of those still has to survive the rules in internal/proposal.
//
// Neither agent has a tool. That is the literal reading of the requirement that
// the quote and pricing tools stay read-only: the pricing catalogue is fetched
// by the caller from the read-only source, and the model is never given a way
// to look up, propose or set an amount.
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/model/gemini"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"

	"github.com/GhandyP/grounded-commercial-proposals/internal/proposal"
)

const (
	// DefaultModel is the Gemini model the agents run on unless another is named.
	DefaultModel = "gemini-flash-latest"
	// ModelEnv overrides DefaultModel.
	ModelEnv = "ADK_MODEL"
	// APIKeyEnv is the environment variable the model client reads.
	APIKeyEnv = "GOOGLE_API_KEY"
	// AppName identifies the ADK application that owns the sessions.
	AppName = "grounded-commercial-proposals"

	outlineAgentName = "proposal_outliner"
	checkAgentName   = "claim_checker"
	cliUserID        = "grounded-commercial-proposals-cli"
)

// ErrMissingAPIKey reports a model call attempted without credentials.
var ErrMissingAPIKey = fmt.Errorf("%s is required to call the model", APIKeyEnv)

// ErrNoAgentOutput reports an agent turn that returned nothing at all.
var ErrNoAgentOutput = errors.New("the agent finished without an answer")

// Config configures the two agents.
type Config struct {
	// Model is required. It is injected so the whole layer is testable offline
	// with a fake, and so the caller decides how the model is authenticated.
	Model model.LLM
	// AppName, UserID and SessionID default to sensible values.
	AppName   string
	UserID    string
	SessionID string
}

// Agents is the model half of the program: an agent that proposes the outline,
// and an agent that proposes claims for it.
//
// Both run through one in-memory session service and one session id, so the
// second stage sees what the first was told. Together they are one proposal
// session, not two unrelated conversations.
type Agents struct {
	outline   *runner.Runner
	check     *runner.Runner
	userID    string
	sessionID string
}

// New builds both agents and their runners.
func New(_ context.Context, cfg Config) (*Agents, error) {
	if cfg.Model == nil {
		return nil, errors.New("agent: a model is required")
	}

	appName := cfg.AppName
	if appName == "" {
		appName = AppName
	}
	userID := cfg.UserID
	if userID == "" {
		userID = cliUserID
	}
	sessionID := cfg.SessionID
	if sessionID == "" {
		sessionID = fmt.Sprintf("proposal-%d", time.Now().UnixNano())
	}

	// Mode is left at its default. ADK also offers a task mode that collects the
	// schema through an injected finish_task tool, but that path is driven by a
	// workflow node: with task mode set, a plain runner run yields no model call
	// at all (verified against adk/v2). In chat mode the framework puts the
	// agent's OutputSchema on the request as a native response schema, so the
	// model answers with JSON and this package still parses it through its own
	// types.
	outlineAgent, err := llmagent.New(llmagent.Config{
		Name:         outlineAgentName,
		Description:  "Proposes the sections of a commercial proposal from a brief, and cites nothing.",
		Model:        cfg.Model,
		Instruction:  instruction,
		OutputSchema: OutlineSchema(),
	})
	if err != nil {
		return nil, fmt.Errorf("agent: outline agent: %w", err)
	}

	checkAgent, err := llmagent.New(llmagent.Config{
		Name:         checkAgentName,
		Description:  "Proposes claims with quotations copied from the approved records it is given.",
		Model:        cfg.Model,
		Instruction:  instruction,
		OutputSchema: CheckSchema(),
	})
	if err != nil {
		return nil, fmt.Errorf("agent: claim checker agent: %w", err)
	}

	sessions := session.InMemoryService()
	outlineRunner, err := runner.New(runner.Config{
		AppName:           appName,
		Agent:             outlineAgent,
		SessionService:    sessions,
		AutoCreateSession: true,
	})
	if err != nil {
		return nil, fmt.Errorf("agent: outline runner: %w", err)
	}
	checkRunner, err := runner.New(runner.Config{
		AppName:           appName,
		Agent:             checkAgent,
		SessionService:    sessions,
		AutoCreateSession: true,
	})
	if err != nil {
		return nil, fmt.Errorf("agent: claim checker runner: %w", err)
	}

	return &Agents{outline: outlineRunner, check: checkRunner, userID: userID, sessionID: sessionID}, nil
}

// SessionID is the session both stages write to.
func (a *Agents) SessionID() string { return a.sessionID }

// Outline implements proposal.Outliner.
func (a *Agents) Outline(ctx context.Context, brief proposal.Brief) ([]proposal.SectionCandidate, error) {
	raw, err := a.run(ctx, a.outline, OutlinePrompt(brief))
	if err != nil {
		return nil, err
	}
	return ParseSections(raw)
}

// Check implements proposal.Checker.
//
// The records are passed in already scoped to the brief's customer by the
// caller, and are filtered again here, so a record of another customer can
// never reach the model even if a caller hands one over.
func (a *Agents) Check(ctx context.Context, brief proposal.Brief, sections []proposal.Section, records []proposal.Evidence) ([]proposal.ClaimCandidate, error) {
	raw, err := a.run(ctx, a.check, CheckPrompt(brief, sections, records))
	if err != nil {
		return nil, err
	}
	return ParseClaims(raw)
}

// run sends one turn to one agent and returns its answer as JSON text.
//
// The framework surfaces a parsed value in event.Output only on the workflow
// node path. On the runner path the model's schema-constrained JSON arrives as
// the final text turn, so both shapes are accepted and both are parsed again by
// this package.
func (a *Agents) run(ctx context.Context, r *runner.Runner, prompt string) (string, error) {
	message := &genai.Content{
		Role:  "user",
		Parts: []*genai.Part{{Text: prompt}},
	}

	var output any
	var finalText string
	for event, err := range r.Run(ctx, a.userID, a.sessionID, message, agent.RunConfig{}) {
		if err != nil {
			return "", fmt.Errorf("agent: run: %w", err)
		}
		if event == nil {
			continue
		}
		if event.Output != nil {
			output = event.Output
		}
		if text, ok := finalModelText(event); ok {
			finalText = text
		}
	}

	switch {
	case output != nil:
		raw, err := json.Marshal(output)
		if err != nil {
			return "", fmt.Errorf("agent: the agent output cannot be read: %w", err)
		}
		return string(raw), nil
	case strings.TrimSpace(finalText) != "":
		return finalText, nil
	default:
		return "", ErrNoAgentOutput
	}
}

// finalModelText returns the text of a final model turn: the plain, complete
// answer, as opposed to a partial event or a turn that asks to call a tool.
func finalModelText(event *session.Event) (string, bool) {
	if event.Partial || event.Content == nil || event.Content.Role != "model" {
		return "", false
	}
	var builder strings.Builder
	for _, part := range event.Content.Parts {
		if part == nil || part.Thought {
			continue
		}
		if part.FunctionCall != nil {
			return "", false
		}
		builder.WriteString(part.Text)
	}
	text := builder.String()
	return text, strings.TrimSpace(text) != ""
}

// NewModel builds the Gemini model from the environment.
//
// name defaults to ModelEnv and then to DefaultModel. A missing APIKeyEnv is
// reported as ErrMissingAPIKey and no call is attempted, so a machine without
// credentials fails at the boundary instead of inside a request.
func NewModel(ctx context.Context, name string) (model.LLM, error) {
	if strings.TrimSpace(name) == "" {
		name = strings.TrimSpace(os.Getenv(ModelEnv))
	}
	if name == "" {
		name = DefaultModel
	}

	key := strings.TrimSpace(os.Getenv(APIKeyEnv))
	if key == "" {
		return nil, fmt.Errorf("agent: %w", ErrMissingAPIKey)
	}

	built, err := gemini.NewModel(ctx, name, &genai.ClientConfig{APIKey: key})
	if err != nil {
		return nil, fmt.Errorf("agent: model %q: %w", name, err)
	}
	return built, nil
}
