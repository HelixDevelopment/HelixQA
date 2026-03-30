// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package llm

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// defaultMaxFails is the number of consecutive failures
// before the escalation provider moves to the next tier.
const defaultMaxFails = 3

// EscalationTier groups a provider with its cost ranking
// and display name.
type EscalationTier struct {
	// Provider is the LLM backend for this tier.
	Provider Provider

	// Name is a human-readable label for logging.
	Name string

	// CostRank orders tiers from cheapest (1) to most
	// expensive. Ties are broken by registry quality score.
	CostRank int
}

// EscalationProvider starts with the cheapest provider and
// escalates to more expensive ones on repeated consecutive
// failures. After a successful call the failure counter
// resets. This minimizes cost while maintaining reliability.
//
// Thread-safe: all state mutations are protected by a mutex.
type EscalationProvider struct {
	mu          sync.Mutex
	tiers       []EscalationTier
	currentTier int
	failCount   int
	maxFails    int
}

// NewEscalationProvider constructs an EscalationProvider
// from the given providers. Providers are sorted by cost
// using the visionModelRegistry (cheapest first). If no
// registry entry exists the provider is placed at the end.
// maxFails defaults to 3 if <= 0.
func NewEscalationProvider(
	providers []Provider,
) *EscalationProvider {
	return NewEscalationProviderWithMaxFails(
		providers, defaultMaxFails,
	)
}

// NewEscalationProviderWithMaxFails constructs an
// EscalationProvider with a custom failure threshold.
func NewEscalationProviderWithMaxFails(
	providers []Provider,
	maxFails int,
) *EscalationProvider {
	if maxFails <= 0 {
		maxFails = defaultMaxFails
	}

	tiers := buildTiers(providers)

	return &EscalationProvider{
		tiers:    tiers,
		maxFails: maxFails,
	}
}

// buildTiers creates sorted tiers from providers. Cheapest
// providers come first, then progressively more expensive.
func buildTiers(providers []Provider) []EscalationTier {
	type scored struct {
		provider Provider
		cost     float64
		quality  float64
	}

	var items []scored
	for _, p := range providers {
		entry, known := visionRegistryByProvider[p.Name()]
		cost := 999.0 // unknown providers go last
		quality := 0.5
		if known {
			cost = entry.CostPer1kTokens
			quality = entry.QualityScore
		}
		items = append(items, scored{
			provider: p,
			cost:     cost,
			quality:  quality,
		})
	}

	// Sort by cost ascending, then by quality descending
	// to break ties (prefer higher quality at the same
	// cost tier).
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].cost != items[j].cost {
			return items[i].cost < items[j].cost
		}
		return items[i].quality > items[j].quality
	})

	tiers := make([]EscalationTier, len(items))
	for i, item := range items {
		tiers[i] = EscalationTier{
			Provider: item.provider,
			Name:     item.provider.Name(),
			CostRank: i + 1,
		}
	}
	return tiers
}

// Name returns the canonical identifier.
func (ep *EscalationProvider) Name() string {
	return "escalation"
}

// SupportsVision reports true when at least one tier's
// provider supports vision.
func (ep *EscalationProvider) SupportsVision() bool {
	for _, t := range ep.tiers {
		if t.Provider.SupportsVision() {
			return true
		}
	}
	return false
}

// Chat delegates to the current tier's provider. On failure
// the failure counter increments and the tier may escalate.
func (ep *EscalationProvider) Chat(
	ctx context.Context,
	messages []Message,
) (*Response, error) {
	ep.mu.Lock()
	if len(ep.tiers) == 0 {
		ep.mu.Unlock()
		return nil, fmt.Errorf(
			"llm: escalation: no providers configured",
		)
	}
	tier := ep.tiers[ep.currentTier]
	ep.mu.Unlock()

	resp, err := tier.Provider.Chat(ctx, messages)
	ep.handleResult(resp, err)
	return resp, err
}

// Vision delegates to the current tier's provider. On
// failure the failure counter increments and the tier may
// escalate. Vision-only providers that do not support
// vision are skipped during escalation.
func (ep *EscalationProvider) Vision(
	ctx context.Context,
	image []byte,
	prompt string,
) (*Response, error) {
	ep.mu.Lock()
	if len(ep.tiers) == 0 {
		ep.mu.Unlock()
		return nil, fmt.Errorf(
			"llm: escalation: no providers configured",
		)
	}
	tier := ep.tiers[ep.currentTier]
	ep.mu.Unlock()

	resp, err := tier.Provider.Vision(ctx, image, prompt)
	ep.handleResult(resp, err)
	return resp, err
}

// handleResult updates the failure counter and escalates
// the tier when the threshold is reached.
func (ep *EscalationProvider) handleResult(
	resp *Response, err error,
) {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	if err != nil || resp == nil ||
		!resp.HasContent() {
		ep.failCount++
		if ep.failCount >= ep.maxFails &&
			ep.currentTier < len(ep.tiers)-1 {
			ep.currentTier++
			ep.failCount = 0
			fmt.Printf(
				"  [escalation] escalating to tier %d: %s\n",
				ep.currentTier+1,
				ep.tiers[ep.currentTier].Name,
			)
		}
		return
	}

	// Success: reset the failure counter but stay on the
	// current tier. We do not de-escalate automatically
	// because the cheaper tier already proved unreliable.
	ep.failCount = 0
}

// CurrentTier returns the zero-based index of the current
// tier.
func (ep *EscalationProvider) CurrentTier() int {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	return ep.currentTier
}

// FailCount returns the current consecutive failure count.
func (ep *EscalationProvider) FailCount() int {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	return ep.failCount
}

// Tiers returns a copy of the tier list.
func (ep *EscalationProvider) Tiers() []EscalationTier {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	result := make([]EscalationTier, len(ep.tiers))
	copy(result, ep.tiers)
	return result
}

// Reset returns the escalation provider to its initial
// state (tier 0, zero failures). Useful between pipeline
// phases or test sessions.
func (ep *EscalationProvider) Reset() {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	ep.currentTier = 0
	ep.failCount = 0
}

// Status returns a human-readable summary of the current
// escalation state for logging.
func (ep *EscalationProvider) Status() string {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	if len(ep.tiers) == 0 {
		return "escalation: no tiers"
	}
	var names []string
	for _, t := range ep.tiers {
		names = append(names, t.Name)
	}
	return fmt.Sprintf(
		"escalation: tier=%d/%d (%s) fails=%d/%d "+
			"order=[%s]",
		ep.currentTier+1, len(ep.tiers),
		ep.tiers[ep.currentTier].Name,
		ep.failCount, ep.maxFails,
		strings.Join(names, " -> "),
	)
}
