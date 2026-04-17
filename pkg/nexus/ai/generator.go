package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Generator converts a natural-language user story into a bank YAML
// block the HelixQA runtime can execute directly.
type Generator struct {
	llm   LLMClient
	cost  *CostTracker
	model string
}

// NewGenerator returns a Generator bound to llm.
func NewGenerator(llm LLMClient, cost *CostTracker, model string) *Generator {
	if model == "" {
		model = "claude-opus-4-7"
	}
	return &Generator{llm: llm, cost: cost, model: model}
}

const generatorSystemPrompt = `You are a test bank YAML generator. Convert a user story into a YAML test_case block with fields id, name, category, priority, platforms, steps (list of {name, action, expected}), expected_result, and tags.
Return only YAML — no code fences, no prose. Use stable ids of the form NX-GEN-<slug>.`

// Generate returns a bank YAML block for story.
func (g *Generator) Generate(ctx context.Context, story string, platform string) (string, error) {
	if strings.TrimSpace(story) == "" {
		return "", errors.New("generator: empty story")
	}
	resp, err := g.llm.Chat(ctx, ChatRequest{
		Model:        g.model,
		SystemPrompt: generatorSystemPrompt,
		UserPrompt:   fmt.Sprintf("PLATFORM: %s\nSTORY:\n%s", platform, story),
		MaxTokens:    800,
		Temperature:  0.2,
	})
	if err != nil {
		return "", fmt.Errorf("generator chat: %w", err)
	}
	if g.cost != nil {
		if err := g.cost.Reserve(resp.CostUSD); err != nil {
			return "", err
		}
		g.cost.Record(Entry{
			Provider: resp.Provider, Model: resp.Model,
			TokensIn: resp.TokensIn, TokensOut: resp.TokensOut,
			CostCents: int(resp.CostUSD * 100),
			Outcome:   "generate",
		})
	}
	raw := stripFences(resp.Text)
	if err := validateYAML(raw); err != nil {
		return "", fmt.Errorf("generator: invalid YAML: %w", err)
	}
	return raw, nil
}

func stripFences(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```yaml")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

func validateYAML(raw string) error {
	var node any
	if err := yaml.Unmarshal([]byte(raw), &node); err != nil {
		return err
	}
	m, ok := node.(map[string]any)
	if !ok {
		return errors.New("root must be a mapping")
	}
	for _, required := range []string{"id", "name", "steps"} {
		if _, ok := m[required]; !ok {
			return fmt.Errorf("missing field %q", required)
		}
	}
	stepsRaw, ok := m["steps"].([]any)
	if !ok {
		return errors.New("field \"steps\" must be a list")
	}
	if len(stepsRaw) == 0 {
		return errors.New("field \"steps\" must contain at least one entry")
	}
	for i, entry := range stepsRaw {
		step, ok := entry.(map[string]any)
		if !ok {
			return fmt.Errorf("steps[%d] must be a mapping", i)
		}
		for _, field := range []string{"name", "action", "expected"} {
			v, present := step[field]
			if !present {
				return fmt.Errorf("steps[%d] missing field %q", i, field)
			}
			s, ok := v.(string)
			if !ok {
				return fmt.Errorf("steps[%d].%s must be a string", i, field)
			}
			if strings.TrimSpace(s) == "" {
				return fmt.Errorf("steps[%d].%s is empty", i, field)
			}
		}
	}
	if idStr, ok := m["id"].(string); ok && !strings.HasPrefix(idStr, "NX-") {
		return fmt.Errorf("id %q must start with NX-", idStr)
	}
	return nil
}
