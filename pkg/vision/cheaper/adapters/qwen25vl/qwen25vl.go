// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package qwen25vl provides a VisionProvider adapter for locally self-hosted
// Qwen2.5-VL models served via an OpenAI-compatible HTTP API (e.g. vLLM,
// llama.cpp server, or Ollama with OpenAI compatibility enabled). No API key
// is required; the server is assumed to be reachable on the local network.
package qwen25vl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"net/http"
	"time"

	"digital.vasic.helixqa/pkg/vision/cheaper"
	"digital.vasic.helixqa/pkg/vision/cheaper/adapters"
)

const (
	defaultBaseURL = "http://localhost:9192/v1"
	defaultModel   = "Qwen2.5-VL-7B-Instruct"
	defaultTimeout = 120 * time.Second
	providerName   = "qwen2.5-vl"
)

// Qwen25VLProvider is a VisionProvider that sends images to a locally
// self-hosted Qwen2.5-VL model via an OpenAI-compatible chat/completions
// endpoint. All fields are set at construction time and are read-only
// thereafter, making the struct safe for concurrent use.
type Qwen25VLProvider struct {
	client  *http.Client
	baseURL string
	model   string
	timeout time.Duration
}

// NewQwen25VLProvider creates a Qwen25VLProvider from a configuration map.
// Accepted keys:
//   - "base_url" (string)  – API base URL, default "http://localhost:9192/v1"
//   - "model"    (string)  – model identifier, default "Qwen2.5-VL-7B-Instruct"
//   - "timeout"  (float64) – request timeout in seconds, default 120
func NewQwen25VLProvider(config map[string]interface{}) (cheaper.VisionProvider, error) {
	baseURL := defaultBaseURL
	model := defaultModel
	timeout := defaultTimeout

	if config != nil {
		if v, ok := config["base_url"].(string); ok && v != "" {
			baseURL = v
		}
		if v, ok := config["model"].(string); ok && v != "" {
			model = v
		}
		if v, ok := config["timeout"].(float64); ok && v > 0 {
			timeout = time.Duration(v) * time.Second
		}
	}

	return &Qwen25VLProvider{
		client:  &http.Client{Timeout: timeout},
		baseURL: baseURL,
		model:   model,
		timeout: timeout,
	}, nil
}

// Name returns the registered identifier for this provider.
func (p *Qwen25VLProvider) Name() string {
	return providerName
}

// chatRequest is the OpenAI-compatible request body sent to /chat/completions.
type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string        `json:"role"`
	Content []contentPart `json:"content"`
}

type contentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL string `json:"url"`
}

// chatResponse is the relevant subset of an OpenAI-compatible response.
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// Analyze encodes img as a base64 PNG, sends it alongside prompt to the
// Qwen2.5-VL completions endpoint, and returns the structured result.
func (p *Qwen25VLProvider) Analyze(
	ctx context.Context,
	img image.Image,
	prompt string,
) (*cheaper.VisionResult, error) {
	start := time.Now()

	b64, err := adapters.ImageToBase64(img)
	if err != nil {
		return nil, fmt.Errorf("qwen2.5-vl: encode image: %w", err)
	}

	reqBody := chatRequest{
		Model: p.model,
		Messages: []chatMessage{
			{
				Role: "user",
				Content: []contentPart{
					{Type: "text", Text: prompt},
					{
						Type:     "image_url",
						ImageURL: &imageURL{URL: "data:image/png;base64," + b64},
					},
				},
			},
		},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("qwen2.5-vl: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		p.baseURL+"/chat/completions",
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, fmt.Errorf("qwen2.5-vl: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("qwen2.5-vl: HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("qwen2.5-vl: unexpected status %d", resp.StatusCode)
	}

	var raw chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("qwen2.5-vl: decode response: %w", err)
	}

	if len(raw.Choices) == 0 {
		return nil, fmt.Errorf("qwen2.5-vl: empty choices in response")
	}

	return &cheaper.VisionResult{
		Text:        raw.Choices[0].Message.Content,
		RawResponse: raw,
		Duration:    time.Since(start),
		Model:       p.model,
		Provider:    providerName,
		Timestamp:   start,
	}, nil
}

// HealthCheck verifies the provider is reachable by calling GET /models.
func (p *Qwen25VLProvider) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/models", nil)
	if err != nil {
		return fmt.Errorf("qwen2.5-vl: health check build request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("qwen2.5-vl: health check request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("qwen2.5-vl: health check got status %d", resp.StatusCode)
	}

	return nil
}

// GetCapabilities returns the static capability profile for Qwen2.5-VL.
func (p *Qwen25VLProvider) GetCapabilities() cheaper.ProviderCapabilities {
	return cheaper.ProviderCapabilities{
		MaxImageSize:     20 * 1024 * 1024, // 20 MB
		SupportedFormats: []string{"png", "jpg", "jpeg", "webp", "gif"},
		AverageLatency:   3 * time.Second,
		CostPer1MTokens:  0,
	}
}

// GetCostEstimate returns the estimated cost for a single Analyze call.
// Qwen2.5-VL is self-hosted and therefore effectively free; a nominal
// value of 0.0001 is returned to allow cost-aware selectors to rank it.
func (p *Qwen25VLProvider) GetCostEstimate(_ int, _ int) float64 {
	return 0.0001
}
