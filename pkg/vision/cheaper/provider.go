// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package cheaper provides access to cost-effective vision providers for
// HelixQA autonomous QA sessions. It defines the VisionProvider interface,
// core result and capability types, and a factory mechanism so that multiple
// cheaper vision backends (UI-TARS, ShowUI, GLM-4V, Qwen2.5-VL, OmniParser,
// etc.) can be registered, selected, and swapped at runtime without changing
// call-sites.
package cheaper

import (
	"context"
	"image"
	"time"
)

// VisionResult holds the structured output of a single vision analysis call.
// Every field is populated by the provider implementation; callers should
// treat zero values as "not reported" rather than errors.
type VisionResult struct {
	// Text is the primary textual interpretation returned by the provider
	// (e.g. an action description, OCR output, or scene caption).
	Text string `json:"text"`

	// RawResponse is the unmodified payload received from the provider,
	// preserved for debugging and audit purposes.
	RawResponse interface{} `json:"raw_response,omitempty"`

	// Metadata carries provider-specific auxiliary data (e.g. bounding boxes,
	// token counts, finish reason).
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// Duration is the wall-clock time the provider took to produce this result.
	Duration time.Duration `json:"duration_ms"`

	// Model is the exact model identifier used for this call.
	Model string `json:"model"`

	// Provider is the registered name of the VisionProvider that produced
	// this result.
	Provider string `json:"provider"`

	// Timestamp records when the analysis call was initiated.
	Timestamp time.Time `json:"timestamp"`

	// CacheHit is true when the result was served from cache rather than
	// making a live provider call.
	CacheHit bool `json:"cache_hit"`

	// Confidence is a normalised [0, 1] score indicating how certain the
	// provider is about the returned interpretation. Providers that do not
	// expose confidence should leave this as 0.
	Confidence float64 `json:"confidence"`
}

// ProviderCapabilities describes the static capabilities and cost profile of
// a VisionProvider. It is returned by GetCapabilities and used by the
// provider optimizer to select the best provider for each phase.
type ProviderCapabilities struct {
	// SupportsStreaming indicates whether the provider can stream partial
	// results back incrementally.
	SupportsStreaming bool `json:"supports_streaming"`

	// MaxImageSize is the maximum image payload the provider accepts, in
	// bytes. A value of 0 means the limit is unknown or unlimited.
	MaxImageSize int `json:"max_image_size"`

	// SupportedFormats lists the MIME sub-types or file extensions the
	// provider can process (e.g. "png", "jpeg", "webp").
	SupportedFormats []string `json:"supported_formats"`

	// AverageLatency is the expected round-trip time for a typical request.
	// Used for SLA-aware provider selection.
	AverageLatency time.Duration `json:"average_latency_ms"`

	// SupportsBatch indicates whether the provider can process multiple
	// images in a single API call.
	SupportsBatch bool `json:"supports_batch"`

	// CostPer1MTokens is the US-dollar cost per one million tokens (input +
	// output combined). A value of 0 indicates a free or locally-hosted
	// provider.
	CostPer1MTokens float64 `json:"cost_per_1m_tokens"`
}

// VisionProvider is the interface that all cheaper vision backends must
// implement. Implementations are expected to be safe for concurrent use.
type VisionProvider interface {
	// Analyze sends img to the provider together with an optional text prompt
	// and returns a structured VisionResult. The caller owns img and may
	// reuse it after the call returns.
	Analyze(ctx context.Context, img image.Image, prompt string) (*VisionResult, error)

	// Name returns the unique registered identifier for this provider
	// (e.g. "ui-tars", "showui", "glm4v").
	Name() string

	// HealthCheck verifies that the provider is reachable and ready to serve
	// requests. Implementations should complete within a short timeout
	// (typically 5 seconds) to avoid blocking session startup.
	HealthCheck(ctx context.Context) error

	// GetCapabilities returns the static capability and cost profile for this
	// provider. The returned value must not be mutated by callers.
	GetCapabilities() ProviderCapabilities

	// GetCostEstimate returns the estimated US-dollar cost for a single
	// Analyze call given the raw image size in bytes and the prompt length
	// in characters. Returns 0 for free/local providers.
	GetCostEstimate(imageSize int, promptLength int) float64
}

// ProviderFactory is a constructor function that creates a new VisionProvider
// from a provider-specific configuration map. It is used by the provider
// registry to instantiate providers at runtime without compile-time coupling.
type ProviderFactory func(config map[string]interface{}) (VisionProvider, error)

// ProviderConfig holds the declarative configuration for a single vision
// provider entry, as loaded from the HelixQA configuration file or
// environment. It is used both for provider registration and for generating
// default configurations in documentation.
type ProviderConfig struct {
	// Name must match the value returned by VisionProvider.Name().
	Name string `yaml:"name" json:"name"`

	// Enabled controls whether the provider participates in selection.
	// Disabled providers are registered but never selected.
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Priority is an integer hint for selection ordering. Higher values
	// indicate higher preference when all other scores are equal.
	Priority int `yaml:"priority" json:"priority"`

	// Config is a provider-specific key/value map forwarded verbatim to the
	// ProviderFactory. Each provider documents its accepted keys.
	Config map[string]interface{} `yaml:"config,omitempty" json:"config,omitempty"`

	// FallbackTo lists the names of providers to try in order when this
	// provider fails or is unavailable.
	FallbackTo []string `yaml:"fallback_to,omitempty" json:"fallback_to,omitempty"`
}
