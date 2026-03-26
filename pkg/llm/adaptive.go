// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package llm

import (
	"context"
	"fmt"
	"strings"
)

// AdaptiveProvider wraps a slice of Provider implementations and
// tries them in order, falling back to the next on failure. It
// satisfies the Provider interface itself, so it can be used
// anywhere a Provider is expected.
type AdaptiveProvider struct {
	providers []Provider
}

// NewAdaptiveProvider constructs an AdaptiveProvider from an
// explicit list of already-constructed Provider instances. The
// providers are tried in the order supplied.
func NewAdaptiveProvider(providers ...Provider) *AdaptiveProvider {
	return &AdaptiveProvider{providers: providers}
}

// NewAdaptiveFromConfigs constructs an AdaptiveProvider by
// instantiating providers from the supplied ProviderConfig slice.
// Configs that fail validation or reference an unknown provider
// type are silently skipped. An error is returned only when zero
// valid providers are produced.
func NewAdaptiveFromConfigs(
	configs []ProviderConfig,
) (*AdaptiveProvider, error) {
	var providers []Provider
	for _, cfg := range configs {
		if err := cfg.Validate(); err != nil {
			continue
		}
		switch cfg.Name {
		case ProviderAnthropic:
			providers = append(providers, NewAnthropicProvider(cfg))
		case ProviderOllama, ProviderUITars:
			providers = append(providers, NewOllamaProvider(cfg))
		default:
			// Check registry for OpenAI-compatible providers
			if defaults, ok := providerDefaults[cfg.Name]; ok {
				if cfg.BaseURL == "" {
					cfg.BaseURL = defaults.BaseURL
				}
				if cfg.Model == "" && defaults.Model != "" {
					cfg.Model = defaults.Model
				}
				providers = append(providers, NewOpenAIProvider(cfg))
			} else if cfg.Name == ProviderOpenAI {
				providers = append(providers, NewOpenAIProvider(cfg))
			}
			// truly unknown — skip silently
		}
	}
	if len(providers) == 0 {
		return nil, fmt.Errorf(
			"llm: NewAdaptiveFromConfigs: no valid providers produced",
		)
	}
	return &AdaptiveProvider{providers: providers}, nil
}

// Name returns the canonical identifier for the adaptive provider.
func (a *AdaptiveProvider) Name() string {
	return "adaptive"
}

// SupportsVision reports true when at least one wrapped provider
// supports vision inputs.
func (a *AdaptiveProvider) SupportsVision() bool {
	for _, p := range a.providers {
		if p.SupportsVision() {
			return true
		}
	}
	return false
}

// Chat tries each provider in order and returns the first successful
// response. If every provider returns an error the combined errors
// are returned in a single diagnostic message.
func (a *AdaptiveProvider) Chat(
	ctx context.Context,
	messages []Message,
) (*Response, error) {
	if len(a.providers) == 0 {
		return nil, fmt.Errorf("llm: all providers failed: no providers configured")
	}
	var errs []string
	for _, p := range a.providers {
		resp, err := p.Chat(ctx, messages)
		if err == nil {
			return resp, nil
		}
		errs = append(errs, fmt.Sprintf("%s: %v", p.Name(), err))
	}
	return nil, fmt.Errorf(
		"llm: all providers failed: %s",
		strings.Join(errs, "; "),
	)
}

// Vision tries each vision-capable provider in order and returns
// the first successful response. Providers that do not support
// vision are skipped entirely. If no vision-capable provider is
// registered a descriptive error is returned immediately.
func (a *AdaptiveProvider) Vision(
	ctx context.Context,
	image []byte,
	prompt string,
) (*Response, error) {
	var capable []Provider
	for _, p := range a.providers {
		if p.SupportsVision() {
			capable = append(capable, p)
		}
	}
	if len(capable) == 0 {
		return nil, fmt.Errorf("llm: no vision-capable providers available")
	}
	var errs []string
	for _, p := range capable {
		resp, err := p.Vision(ctx, image, prompt)
		if err == nil {
			return resp, nil
		}
		errs = append(errs, fmt.Sprintf("%s: %v", p.Name(), err))
	}
	return nil, fmt.Errorf(
		"llm: all providers failed: %s",
		strings.Join(errs, "; "),
	)
}
