package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"digital.vasic.helixqa/pkg/nexus"
)

// NavigationAction is the AI's next move in the UI.
type NavigationAction struct {
	Kind       string  `json:"kind"`        // click | type | scroll | wait | done
	Target     string  `json:"target"`      // ElementRef or free-text description
	Text       string  `json:"text,omitempty"`
	Reasoning  string  `json:"reasoning"`
	Confidence float64 `json:"confidence"`
}

// VisualContext carries the information a vision model needs to pick
// the next action. Screenshot is PNG bytes; Tree is the serialised UI
// tree (HTML for browser, XML for mobile, JSON for desktop).
type VisualContext struct {
	Screenshot []byte
	Tree       string
	Goal       string
	PreviousActions []NavigationAction
	URL        string
	Platform   nexus.Platform
}

// LLMClient is the narrow contract the navigator needs from the
// orchestrator. Implementations wrap the existing LLMProvider /
// LLMOrchestrator submodules.
type LLMClient interface {
	// Chat sends a prompt + attachments and returns raw text reply,
	// plus token usage for the CostTracker.
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
}

// ChatRequest is the vendor-agnostic request envelope.
type ChatRequest struct {
	Model       string
	SystemPrompt string
	UserPrompt   string
	ImageBase64  []string
	JSONResponse bool
	MaxTokens    int
	Temperature  float64
}

// ChatResponse carries the reply plus cost signals.
type ChatResponse struct {
	Text       string
	Provider   string
	Model      string
	TokensIn   int
	TokensOut  int
	CostUSD    float64
}

// Navigator decides the next UI action to progress toward a Goal.
type Navigator struct {
	llm   LLMClient
	cost  *CostTracker
	model string
}

// NewNavigator returns a Navigator that dispatches to llm and records
// cost on every decision.
func NewNavigator(llm LLMClient, cost *CostTracker, model string) *Navigator {
	if model == "" {
		model = "claude-opus-4-7"
	}
	return &Navigator{llm: llm, cost: cost, model: model}
}

const navigatorSystemPrompt = `You are an autonomous QA agent navigating a UI.
Return JSON of shape {"kind":"click|type|scroll|wait|done","target":"ref or description","text":"","reasoning":"why","confidence":0..1}.
Pick 'done' when the goal is satisfied. Do not invent element references that are not present in the tree.`

// Decide returns the next NavigationAction for vc.
func (n *Navigator) Decide(ctx context.Context, vc VisualContext) (*NavigationAction, error) {
	prompt := fmt.Sprintf(
		"GOAL: %s\nURL: %s\nTREE:\n%s\nPREVIOUS ACTIONS:\n%s",
		vc.Goal, vc.URL, trimTree(vc.Tree, 8000),
		serialisePrevious(vc.PreviousActions),
	)
	resp, err := n.llm.Chat(ctx, ChatRequest{
		Model:        n.model,
		SystemPrompt: navigatorSystemPrompt,
		UserPrompt:   prompt,
		JSONResponse: true,
		MaxTokens:    256,
	})
	if err != nil {
		return nil, fmt.Errorf("navigator chat: %w", err)
	}
	if n.cost != nil {
		if err := n.cost.Reserve(resp.CostUSD); err != nil {
			return nil, err
		}
		n.cost.Record(Entry{
			Provider: resp.Provider, Model: resp.Model,
			TokensIn: resp.TokensIn, TokensOut: resp.TokensOut,
			CostCents: int(resp.CostUSD * 100),
			Outcome:   "decision",
		})
	}
	action, err := parseAction(resp.Text)
	if err != nil {
		return nil, fmt.Errorf("navigator parse: %w (raw=%s)", err, resp.Text)
	}
	return action, nil
}

func parseAction(raw string) (*NavigationAction, error) {
	raw = strings.TrimSpace(raw)
	// Tolerate code fences.
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)
	var a NavigationAction
	if err := json.Unmarshal([]byte(raw), &a); err != nil {
		return nil, err
	}
	if a.Kind == "" {
		return nil, fmt.Errorf("missing kind field")
	}
	return &a, nil
}

func trimTree(tree string, cap int) string {
	if len(tree) <= cap {
		return tree
	}
	return tree[:cap] + "... (truncated)"
}

func serialisePrevious(prev []NavigationAction) string {
	if len(prev) == 0 {
		return "(none)"
	}
	var b strings.Builder
	for i, a := range prev {
		fmt.Fprintf(&b, "%d. %s %s %q\n", i+1, a.Kind, a.Target, a.Text)
	}
	return b.String()
}
