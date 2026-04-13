// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package uitars provides a VisionProvider adapter for the ByteDance UI-TARS
// 1.5-7B model served via the Hugging Face Inference API using an
// OpenAI-compatible chat-completions endpoint.
package uitars

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"net/http"
	"time"

	"digital.vasic.helixqa/pkg/vision/cheaper"
	"digital.vasic.helixqa/pkg/vision/cheaper/adapters"
)

const (
	defaultBaseURL = "https://api-inference.huggingface.co"
	defaultModel   = "ByteDance-Seed/UI-TARS-1.5-7B"
	defaultTimeout = 60 * time.Second
	providerName   = "ui-tars-1.5"
)

// UITARSProvider implements cheaper.VisionProvider using the Hugging Face
// Inference API with an OpenAI-compatible chat-completions protocol.
type UITARSProvider struct {
	client  *http.Client
	baseURL string
	apiKey  string
	model   string
	timeout time.Duration
}

// chatMessage mirrors the OpenAI messages array element.
type chatMessage struct {
	Role    string        `json:"role"`
	Content []interface{} `json:"content"`
}

// textContent is a plain-text content part.
type textContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// imageURLContent is the image_url content part expected by the model.
type imageURLContent struct {
	Type     string         `json:"type"`
	ImageURL imageURLDetail `json:"image_url"`
}

type imageURLDetail struct {
	URL string `json:"url"`
}

// chatRequest is the JSON body sent to the completions endpoint.
type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

// chatResponse is a minimal representation of the API response we parse.
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// NewUITARSProvider constructs a UITARSProvider from a configuration map.
//
// Recognised keys:
//
//	"api_key"  (string, required) — Hugging Face API token.
//	"base_url" (string, optional) — API base URL (default: https://api-inference.huggingface.co).
//	"model"    (string, optional) — Model identifier (default: ByteDance-Seed/UI-TARS-1.5-7B).
//	"timeout"  (time.Duration or float64 seconds, optional) — Per-request timeout (default: 60s).
func NewUITARSProvider(config map[string]interface{}) (cheaper.VisionProvider, error) {
	apiKey, ok := config["api_key"].(string)
	if !ok || apiKey == "" {
		return nil, fmt.Errorf("uitars: \"api_key\" is required and must be a non-empty string")
	}

	baseURL := defaultBaseURL
	if v, ok := config["base_url"].(string); ok && v != "" {
		baseURL = v
	}

	model := defaultModel
	if v, ok := config["model"].(string); ok && v != "" {
		model = v
	}

	timeout := defaultTimeout
	switch v := config["timeout"].(type) {
	case time.Duration:
		if v > 0 {
			timeout = v
		}
	case float64:
		if v > 0 {
			timeout = time.Duration(v * float64(time.Second))
		}
	}

	return &UITARSProvider{
		client:  &http.Client{Timeout: timeout},
		baseURL: baseURL,
		apiKey:  apiKey,
		model:   model,
		timeout: timeout,
	}, nil
}

// Analyze encodes img to base64 PNG, submits it together with prompt to the
// UI-TARS chat-completions endpoint, and returns a structured VisionResult.
func (p *UITARSProvider) Analyze(ctx context.Context, img image.Image, prompt string) (*VisionResult, error) {
	start := time.Now()

	b64, err := adapters.ImageToBase64(img)
	if err != nil {
		return nil, fmt.Errorf("uitars: image encoding failed: %w", err)
	}

	dataURI := "data:image/png;base64," + b64

	msg := chatMessage{
		Role: "user",
		Content: []interface{}{
			textContent{Type: "text", Text: prompt},
			imageURLContent{
				Type:     "image_url",
				ImageURL: imageURLDetail{URL: dataURI},
			},
		},
	}

	reqBody := chatRequest{
		Model:    p.model,
		Messages: []chatMessage{msg},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("uitars: failed to marshal request: %w", err)
	}

	url := p.baseURL + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("uitars: failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("uitars: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("uitars: failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("uitars: API returned status %d: %s", resp.StatusCode, string(respBytes))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBytes, &chatResp); err != nil {
		return nil, fmt.Errorf("uitars: failed to decode response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("uitars: API returned no choices")
	}

	text := chatResp.Choices[0].Message.Content
	duration := time.Since(start)

	return &VisionResult{
		Text:        text,
		RawResponse: chatResp,
		Duration:    duration,
		Model:       p.model,
		Provider:    providerName,
		Timestamp:   start,
	}, nil
}

// Name returns the unique identifier for this provider.
func (p *UITARSProvider) Name() string {
	return providerName
}

// HealthCheck performs a GET request to {baseURL}/health to verify the service
// is reachable and returns a 2xx response.
func (p *UITARSProvider) HealthCheck(ctx context.Context) error {
	url := p.baseURL + "/health"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("uitars: failed to create health-check request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("uitars: health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("uitars: health check returned status %d", resp.StatusCode)
	}
	return nil
}

// GetCapabilities returns the static capability and cost profile of the
// UI-TARS 1.5-7B provider.
func (p *UITARSProvider) GetCapabilities() cheaper.ProviderCapabilities {
	return cheaper.ProviderCapabilities{
		MaxImageSize:     20 * 1024 * 1024, // 20 MB
		SupportedFormats: []string{"png", "jpg", "jpeg", "webp"},
		AverageLatency:   2 * time.Second,
		CostPer1MTokens:  0,
	}
}

// GetCostEstimate returns the estimated cost for a single Analyze call.
// UI-TARS on Hugging Face Inference API is effectively free for the caller
// at this tier, so we return a nominal value of 0.0001 USD.
func (p *UITARSProvider) GetCostEstimate(imageSize int, promptLength int) float64 {
	return 0.0001
}

// VisionResult re-exports cheaper.VisionResult so test code in this package
// can use it without an additional import alias.
type VisionResult = cheaper.VisionResult
