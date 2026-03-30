// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// defaultConsensusQuorum is the minimum number of providers
// that must agree on an action for consensus to be reached.
const defaultConsensusQuorum = 2

// defaultConsensusTimeout caps the total time allowed for
// all providers to respond during a consensus vote.
const defaultConsensusTimeout = 30 * time.Second

// ConsensusProvider sends the same vision request to N
// providers simultaneously and returns the majority-vote
// action. This reduces hallucination risk by requiring
// multiple models to independently agree on the next UI
// action.
type ConsensusProvider struct {
	providers []Provider
	quorum    int
	timeout   time.Duration
}

// NewConsensusProvider constructs a ConsensusProvider that
// queries all given providers concurrently and requires at
// least quorum providers to agree on the action. If quorum
// is less than 1 it defaults to 2. If quorum exceeds the
// number of providers it is clamped to len(providers).
func NewConsensusProvider(
	providers []Provider,
	quorum int,
) *ConsensusProvider {
	if quorum < 1 {
		quorum = defaultConsensusQuorum
	}
	if quorum > len(providers) {
		quorum = len(providers)
	}
	return &ConsensusProvider{
		providers: providers,
		quorum:    quorum,
		timeout:   defaultConsensusTimeout,
	}
}

// SetTimeout overrides the default consensus timeout.
func (cp *ConsensusProvider) SetTimeout(d time.Duration) {
	if d > 0 {
		cp.timeout = d
	}
}

// Name returns the canonical identifier for the consensus
// provider.
func (cp *ConsensusProvider) Name() string {
	return "consensus"
}

// SupportsVision reports true when at least one wrapped
// provider supports vision inputs.
func (cp *ConsensusProvider) SupportsVision() bool {
	for _, p := range cp.providers {
		if p.SupportsVision() {
			return true
		}
	}
	return false
}

// Chat delegates to the first provider that succeeds. Chat
// calls do not benefit from consensus voting since they are
// typically planning/reasoning tasks, not action selection.
func (cp *ConsensusProvider) Chat(
	ctx context.Context,
	messages []Message,
) (*Response, error) {
	if len(cp.providers) == 0 {
		return nil, fmt.Errorf(
			"llm: consensus: no providers configured",
		)
	}
	var errs []string
	for _, p := range cp.providers {
		resp, err := p.Chat(ctx, messages)
		if err == nil {
			return resp, nil
		}
		errs = append(
			errs,
			fmt.Sprintf("%s: %v", p.Name(), err),
		)
		if ctx.Err() != nil {
			break
		}
	}
	return nil, fmt.Errorf(
		"llm: consensus chat: all providers failed: %s",
		strings.Join(errs, "; "),
	)
}

// visionResult captures one provider's response.
type visionResult struct {
	provider string
	response *Response
	err      error
}

// Vision sends the image to all providers concurrently,
// parses each response's actions, and returns the action
// that appears most frequently across responses. If no
// consensus is reached the response from the highest-ranked
// provider (first in the list) is used as a fallback.
func (cp *ConsensusProvider) Vision(
	ctx context.Context,
	image []byte,
	prompt string,
) (*Response, error) {
	if len(cp.providers) == 0 {
		return nil, fmt.Errorf(
			"llm: consensus: no providers configured",
		)
	}

	// Single provider: no voting needed.
	if len(cp.providers) == 1 {
		return cp.providers[0].Vision(ctx, image, prompt)
	}

	vCtx, vCancel := context.WithTimeout(ctx, cp.timeout)
	defer vCancel()

	var mu sync.Mutex
	results := make([]visionResult, 0, len(cp.providers))

	var wg sync.WaitGroup
	for _, p := range cp.providers {
		if !p.SupportsVision() {
			continue
		}
		wg.Add(1)
		go func(prov Provider) {
			defer wg.Done()
			resp, err := prov.Vision(vCtx, image, prompt)
			mu.Lock()
			results = append(results, visionResult{
				provider: prov.Name(),
				response: resp,
				err:      err,
			})
			mu.Unlock()
		}(p)
	}
	wg.Wait()

	// Collect successful responses.
	var successes []visionResult
	var errs []string
	for _, r := range results {
		if r.err != nil {
			errs = append(
				errs,
				fmt.Sprintf("%s: %v", r.provider, r.err),
			)
			continue
		}
		if r.response == nil || !r.response.HasContent() {
			errs = append(
				errs,
				fmt.Sprintf(
					"%s: empty response", r.provider,
				),
			)
			continue
		}
		successes = append(successes, r)
	}

	if len(successes) == 0 {
		return nil, fmt.Errorf(
			"llm: consensus vision: all providers failed: %s",
			strings.Join(errs, "; "),
		)
	}

	// If only one succeeded, return it directly.
	if len(successes) == 1 {
		return successes[0].response, nil
	}

	// Extract the primary action type from each response.
	votes := make(map[string][]visionResult)
	for _, s := range successes {
		action := extractActionType(s.response.Content)
		votes[action] = append(votes[action], s)
	}

	// Find the action with the most votes.
	var bestAction string
	var bestCount int
	for action, voters := range votes {
		if len(voters) > bestCount {
			bestCount = len(voters)
			bestAction = action
		}
	}

	// Check if quorum is met.
	if bestCount >= cp.quorum {
		// Return the first response with the winning action.
		winner := votes[bestAction][0]
		fmt.Printf(
			"  [consensus] action=%q votes=%d/%d "+
				"(quorum=%d) winner=%s\n",
			bestAction, bestCount, len(successes),
			cp.quorum, winner.provider,
		)
		return winner.response, nil
	}

	// No consensus: fall back to the first successful
	// provider (highest-ranked by original order).
	fmt.Printf(
		"  [consensus] no quorum (best=%q votes=%d "+
			"quorum=%d) — fallback to %s\n",
		bestAction, bestCount, cp.quorum,
		successes[0].provider,
	)
	return successes[0].response, nil
}

// Providers returns the underlying provider slice.
func (cp *ConsensusProvider) Providers() []Provider {
	return cp.providers
}

// Quorum returns the configured quorum value.
func (cp *ConsensusProvider) Quorum() int {
	return cp.quorum
}

// actionPayload is a minimal struct for extracting the
// action type from a JSON response. The autonomous pipeline
// produces JSON with an "action" field.
type actionPayload struct {
	Action string `json:"action"`
	Type   string `json:"type"`
}

// extractActionType attempts to parse the action type from
// a provider response. It tries JSON parsing first, then
// falls back to keyword extraction from the raw text.
func extractActionType(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return "unknown"
	}

	// Try direct JSON parse.
	var payload actionPayload
	if err := json.Unmarshal(
		[]byte(content), &payload,
	); err == nil {
		if payload.Action != "" {
			return strings.ToLower(payload.Action)
		}
		if payload.Type != "" {
			return strings.ToLower(payload.Type)
		}
	}

	// Try to find JSON within markdown code fences.
	if idx := strings.Index(content, "```json"); idx >= 0 {
		start := idx + 7
		end := strings.Index(content[start:], "```")
		if end > 0 {
			jsonStr := strings.TrimSpace(
				content[start : start+end],
			)
			if err := json.Unmarshal(
				[]byte(jsonStr), &payload,
			); err == nil {
				if payload.Action != "" {
					return strings.ToLower(payload.Action)
				}
				if payload.Type != "" {
					return strings.ToLower(payload.Type)
				}
			}
		}
	}

	// Try to find a JSON array and extract first element.
	if idx := strings.Index(content, "["); idx >= 0 {
		end := strings.LastIndex(content, "]")
		if end > idx {
			var arr []actionPayload
			jsonStr := content[idx : end+1]
			if err := json.Unmarshal(
				[]byte(jsonStr), &arr,
			); err == nil && len(arr) > 0 {
				if arr[0].Action != "" {
					return strings.ToLower(arr[0].Action)
				}
				if arr[0].Type != "" {
					return strings.ToLower(arr[0].Type)
				}
			}
		}
	}

	// Keyword extraction fallback: look for common action
	// verbs in the response text.
	lower := strings.ToLower(content)
	actionKeywords := []string{
		"tap", "click", "swipe", "scroll", "type",
		"press", "back", "home", "dpad",
	}
	for _, kw := range actionKeywords {
		if strings.Contains(lower, kw) {
			return kw
		}
	}

	return "unknown"
}
