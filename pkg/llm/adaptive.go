// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package llm

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// adaptivePerProviderTimeout caps how long a single
// provider is allowed per call during adaptive fallback.
// This prevents N slow providers from compounding into
// N * timeout total latency.
const adaptivePerProviderTimeout = 60 * time.Second

// adaptiveVisionTimeout is longer than the chat timeout
// to allow native vision providers (Gemini, Anthropic)
// time for their internal retry/backoff on rate limits.
const adaptiveVisionTimeout = 90 * time.Second

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
		case ProviderGoogle:
			providers = append(providers, NewGoogleProvider(cfg))
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
// are returned in a single diagnostic message. Each provider call is
// capped at adaptivePerProviderTimeout.
func (a *AdaptiveProvider) Chat(
	ctx context.Context,
	messages []Message,
) (*Response, error) {
	if len(a.providers) == 0 {
		return nil, fmt.Errorf("llm: all providers failed: no providers configured")
	}
	var errs []string
	for _, p := range a.providers {
		pCtx, pCancel := context.WithTimeout(
			ctx, adaptivePerProviderTimeout,
		)
		resp, err := p.Chat(pCtx, messages)
		pCancel()
		if err == nil {
			return resp, nil
		}
		errs = append(errs, fmt.Sprintf("%s: %v", p.Name(), err))
		if ctx.Err() != nil {
			break
		}
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
//
// Each provider call is capped at adaptivePerProviderTimeout to
// prevent N slow providers from compounding into N * timeout
// total latency.
func (a *AdaptiveProvider) Vision(
	ctx context.Context,
	image []byte,
	prompt string,
) (*Response, error) {
	// Prioritize providers with native multimodal support
	// (Gemini, Anthropic) over OpenAI-compatible providers
	// whose Vision() may not actually work.
	var capable []Provider
	var secondary []Provider
	for _, p := range a.providers {
		if !p.SupportsVision() {
			continue
		}
		switch p.Name() {
		case "nvidia":
			// NVIDIA has Llama 3.2 90B Vision — the most
			// capable instruction-following vision model.
			// Produces perfect navigation JSON. Highest
			// priority for vision calls.
			capable = append([]Provider{p}, capable...)
		case ProviderOllama:
			// Local Ollama as fallback — free, always
			// available, but smaller models may produce
			// imprecise actions.
			capable = append(capable, p)
		case ProviderGoogle, ProviderAnthropic, ProviderOpenAI,
			"githubmodels":
			capable = append(capable, p)
		default:
			secondary = append(secondary, p)
		}
	}
	capable = append(capable, secondary...)
	if len(capable) == 0 {
		// List all providers to help debug configuration.
		var names []string
		for _, p := range a.providers {
			names = append(names, p.Name())
		}
		return nil, fmt.Errorf(
			"llm: no vision-capable providers among %v",
			names,
		)
	}
	// Log which providers will be attempted for observability.
	{
		var names []string
		for _, p := range capable {
			names = append(names, p.Name())
		}
		fmt.Printf("  [llm] vision providers: %v\n", names)
	}
	var errs []string
	for _, p := range capable {
		// Native vision providers (Gemini, Anthropic) get
		// more time for their internal retry/backoff logic.
		timeout := adaptivePerProviderTimeout
		switch p.Name() {
		case ProviderGoogle, ProviderAnthropic, ProviderOpenAI,
			ProviderOllama, "nvidia", "githubmodels":
			timeout = adaptiveVisionTimeout
		}
		pCtx, pCancel := context.WithTimeout(ctx, timeout)
		resp, err := p.Vision(pCtx, image, prompt)
		pCancel()
		if err == nil {
			return resp, nil
		}
		errs = append(errs, fmt.Sprintf("%s: %v", p.Name(), err))
		// If the parent context is done, stop trying.
		if ctx.Err() != nil {
			break
		}
	}
	return nil, fmt.Errorf(
		"llm: all providers failed: %s",
		strings.Join(errs, "; "),
	)
}
