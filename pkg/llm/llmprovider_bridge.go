// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package llm

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	lpmodels "digital.vasic.llmprovider/pkg/models"
	lpprovider "digital.vasic.llmprovider/pkg/provider"
)

// LLMProviderBridge adapts any implementation of the
// digital.vasic.llmprovider/pkg/provider.LLMProvider interface to the
// HelixQA llm.Provider interface.
//
// This is the real bridge from HelixQA's autonomous QA pipeline to the
// shared LLMProvider Go module, so HelixQA inherits that module's
// production provider stack (circuit breakers, health monitoring,
// retry/backoff, multi-provider discovery) instead of re-implementing
// transport per provider.
//
// The bridge is project-agnostic: the concrete LLMProvider and the
// model name are injected by the caller. No provider keys are read or
// stored here — credential handling stays in the LLMProvider module
// per §11.4.10.
type LLMProviderBridge struct {
	// impl is the wrapped LLMProvider implementation.
	impl lpprovider.LLMProvider

	// name is the canonical HelixQA provider identifier surfaced via
	// Name(). When empty, "llmprovider-bridge" is used.
	name string

	// model is the model identifier passed in every request's
	// ModelParameters. May be empty if the provider infers it.
	model string

	// timeout caps a single Complete invocation. Zero means no
	// bridge-level timeout (the provider's own timeout applies).
	timeout time.Duration

	// supportsVision is resolved from the provider's
	// GetCapabilities() at construction.
	supportsVision bool
}

// LLMProviderBridgeOption configures an LLMProviderBridge.
type LLMProviderBridgeOption func(*LLMProviderBridge)

// WithBridgeName sets the canonical provider name reported by Name().
func WithBridgeName(name string) LLMProviderBridgeOption {
	return func(b *LLMProviderBridge) { b.name = name }
}

// WithBridgeModel sets the model identifier sent in every request.
func WithBridgeModel(model string) LLMProviderBridgeOption {
	return func(b *LLMProviderBridge) { b.model = model }
}

// WithBridgeTimeout caps a single Complete invocation.
func WithBridgeTimeout(d time.Duration) LLMProviderBridgeOption {
	return func(b *LLMProviderBridge) { b.timeout = d }
}

// NewLLMProviderBridge wraps an LLMProvider implementation as a
// HelixQA llm.Provider. It queries GetCapabilities() once to learn
// whether the provider supports vision input.
func NewLLMProviderBridge(impl lpprovider.LLMProvider, opts ...LLMProviderBridgeOption) (*LLMProviderBridge, error) {
	if impl == nil {
		return nil, fmt.Errorf("llm: NewLLMProviderBridge requires a non-nil LLMProvider")
	}
	b := &LLMProviderBridge{
		impl:    impl,
		name:    "llmprovider-bridge",
		timeout: defaultBridgeTimeout,
	}
	for _, o := range opts {
		o(b)
	}
	if caps := impl.GetCapabilities(); caps != nil {
		b.supportsVision = caps.SupportsVision
	}
	return b, nil
}

// Name implements llm.Provider.
func (b *LLMProviderBridge) Name() string { return b.name }

// SupportsVision implements llm.Provider.
func (b *LLMProviderBridge) SupportsVision() bool { return b.supportsVision }

// Chat implements llm.Provider by translating the HelixQA message
// slice into an LLMProvider request, calling Complete, and mapping the
// response back.
func (b *LLMProviderBridge) Chat(ctx context.Context, messages []Message) (*Response, error) {
	ctx, cancel := b.withTimeout(ctx)
	defer cancel()

	req := &lpmodels.LLMRequest{
		Messages:    toLPMessages(messages),
		ModelParams: lpmodels.ModelParameters{Model: b.model},
		RequestType: "chat",
		CreatedAt:   time.Now(),
	}
	// Some providers prefer a flat Prompt; populate it from the last
	// user message for compatibility while keeping Messages intact.
	req.Prompt = lastUserContent(messages)

	resp, err := b.impl.Complete(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("llm: llmprovider bridge Chat: %w", err)
	}
	if resp == nil {
		return nil, fmt.Errorf("llm: llmprovider bridge Chat: nil response")
	}
	return b.toResponse(resp), nil
}

// Vision implements llm.Provider. The image is base64-encoded and
// attached to the user prompt in a provider-neutral way; providers
// that advertise SupportsVision interpret it. Callers SHOULD check
// SupportsVision first.
func (b *LLMProviderBridge) Vision(ctx context.Context, image []byte, prompt string) (*Response, error) {
	if !b.supportsVision {
		return nil, fmt.Errorf("llm: provider %q does not support vision", b.name)
	}
	ctx, cancel := b.withTimeout(ctx)
	defer cancel()

	encoded := base64.StdEncoding.EncodeToString(image)
	req := &lpmodels.LLMRequest{
		Prompt: prompt,
		Messages: []lpmodels.Message{
			{Role: "user", Content: prompt},
		},
		ModelParams: lpmodels.ModelParameters{
			Model: b.model,
			ProviderSpecific: map[string]interface{}{
				"image_base64": encoded,
				"image_bytes":  len(image),
			},
		},
		RequestType: "vision",
		CreatedAt:   time.Now(),
	}
	resp, err := b.impl.Complete(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("llm: llmprovider bridge Vision: %w", err)
	}
	if resp == nil {
		return nil, fmt.Errorf("llm: llmprovider bridge Vision: nil response")
	}
	return b.toResponse(resp), nil
}

// HealthCheck delegates to the underlying provider's health check. It
// is exposed so callers can verify the bridge is live before a
// session (not part of llm.Provider, but available for pre-flight).
func (b *LLMProviderBridge) HealthCheck() error {
	return b.impl.HealthCheck()
}

func (b *LLMProviderBridge) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if b.timeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, b.timeout)
}

func (b *LLMProviderBridge) toResponse(resp *lpmodels.LLMResponse) *Response {
	model := b.model
	if model == "" {
		model = resp.ProviderName
	}
	return &Response{
		Content:      resp.Content,
		Model:        model,
		OutputTokens: resp.TokensUsed,
	}
}

func toLPMessages(in []Message) []lpmodels.Message {
	out := make([]lpmodels.Message, 0, len(in))
	for _, m := range in {
		out = append(out, lpmodels.Message{Role: m.Role, Content: m.Content})
	}
	return out
}

func lastUserContent(messages []Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if strings.EqualFold(messages[i].Role, RoleUser) {
			return messages[i].Content
		}
	}
	if len(messages) > 0 {
		return messages[len(messages)-1].Content
	}
	return ""
}

// Ensure the bridge satisfies the HelixQA Provider interface at
// compile time.
var _ Provider = (*LLMProviderBridge)(nil)
