package ai

import (
	"context"
	"fmt"
	"strings"
)

// Healer recovers from a broken selector by asking the LLM to map a
// failed target back to a current-tree element.
type Healer struct {
	llm   LLMClient
	cost  *CostTracker
	model string
}

// NewHealer returns a Healer that uses llm + cost.
func NewHealer(llm LLMClient, cost *CostTracker, model string) *Healer {
	if model == "" {
		model = "claude-sonnet-4-6"
	}
	return &Healer{llm: llm, cost: cost, model: model}
}

const healerSystemPrompt = `You are a UI test healing assistant. Given a failed selector, a description of what the tester expected, and a fresh accessibility tree, return a single replacement selector as plain text.
Do not include explanations, code fences, or surrounding punctuation. Return an empty line if no safe replacement exists.`

// Heal returns a replacement selector or an empty string if none could
// be found with reasonable confidence.
func (h *Healer) Heal(ctx context.Context, failed, description, tree string) (string, error) {
	resp, err := h.llm.Chat(ctx, ChatRequest{
		Model:        h.model,
		SystemPrompt: healerSystemPrompt,
		UserPrompt: fmt.Sprintf(
			"FAILED SELECTOR: %s\nDESCRIPTION: %s\nTREE:\n%s",
			failed, description, trimTree(tree, 6000),
		),
		MaxTokens:   128,
		Temperature: 0,
	})
	if err != nil {
		return "", fmt.Errorf("healer chat: %w", err)
	}
	if h.cost != nil {
		if err := h.cost.Reserve(resp.CostUSD); err != nil {
			return "", err
		}
		h.cost.Record(Entry{
			Provider: resp.Provider, Model: resp.Model,
			TokensIn: resp.TokensIn, TokensOut: resp.TokensOut,
			CostCents: int(resp.CostUSD * 100),
			Outcome:   "heal",
		})
	}
	return strings.TrimSpace(resp.Text), nil
}
