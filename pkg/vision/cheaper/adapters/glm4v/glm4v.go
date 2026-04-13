// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package glm4v implements a VisionProvider backed by Zhipu AI's GLM-4V
// family of vision-language models. The default model is glm-4v-flash which
// is available on the free tier. The API is OpenAI-compatible with one
// notable difference: the image_url value must be a raw base64 string — the
// standard "data:image/png;base64," prefix must be omitted.
package glm4v

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"net/http"
	"strings"
	"time"

	"digital.vasic.helixqa/pkg/vision/cheaper"
	"digital.vasic.helixqa/pkg/vision/cheaper/adapters"
)

const (
	defaultBaseURL = "https://open.bigmodel.cn/api/paas/v4"
	defaultModel   = "glm-4v-flash"
	defaultTimeout = 60 * time.Second

	providerName     = "glm-4v"
	paidModel        = "glm-4v"
	paidModelCost    = 0.015
	flashModelCost   = 0.0
	maxImageSize     = 10 * 1024 * 1024 // 10 MB
	avgLatency       = 1 * time.Second
)

// GLM4VProvider is a VisionProvider implementation that calls the Zhipu AI
// GLM-4V API. It is safe for concurrent use.
type GLM4VProvider struct {
	client  *http.Client
	baseURL string
	apiKey  string
	model   string
	timeout time.Duration
}

// NewGLM4VProvider constructs a GLM4VProvider from the supplied configuration
// map. The only required key is "api_key". Optional keys:
//   - "base_url" (string, default "https://open.bigmodel.cn/api/paas/v4")
//   - "model"    (string, default "glm-4v-flash")
//   - "timeout"  (numeric seconds, default 60)
func NewGLM4VProvider(config map[string]interface{}) (cheaper.VisionProvider, error) {
	apiKey, ok := config["api_key"].(string)
	if !ok || strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("glm4v: api_key is required and must be a non-empty string")
	}

	baseURL := defaultBaseURL
	if v, ok := config["base_url"].(string); ok && strings.TrimSpace(v) != "" {
		baseURL = strings.TrimRight(v, "/")
	}

	model := defaultModel
	if v, ok := config["model"].(string); ok && strings.TrimSpace(v) != "" {
		model = v
	}

	timeout := defaultTimeout
	if v, ok := config["timeout"].(float64); ok && v > 0 {
		timeout = time.Duration(v) * time.Second
	}

	p := &GLM4VProvider{
		client:  &http.Client{Timeout: timeout},
		baseURL: baseURL,
		apiKey:  apiKey,
		model:   model,
		timeout: timeout,
	}
	return p, nil
}

// Name returns the unique provider identifier "glm-4v".
func (p *GLM4VProvider) Name() string {
	return providerName
}

// chatRequest is the JSON body sent to the /chat/completions endpoint.
type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string        `json:"role"`
	Content []contentPart `json:"content"`
}

type contentPart struct {
	Type     string        `json:"type"`
	Text     string        `json:"text,omitempty"`
	ImageURL *imageURLPart `json:"image_url,omitempty"`
}

// imageURLPart holds the Zhipu-specific image_url object. Unlike standard
// data URIs, Zhipu expects the raw base64 string without the
// "data:image/png;base64," prefix.
type imageURLPart struct {
	URL string `json:"url"`
}

// chatResponse is the subset of the OpenAI-compatible response we need.
type chatResponse struct {
	ID      string       `json:"id"`
	Model   string       `json:"model"`
	Choices []chatChoice `json:"choices"`
}

type chatChoice struct {
	Message chatMessageResponse `json:"message"`
}

type chatMessageResponse struct {
	Content string `json:"content"`
}

// Analyze encodes img as PNG, base64-encodes the bytes (without a data URI
// prefix), and sends a chat completion request to the GLM-4V API together
// with the supplied prompt. It returns a VisionResult on success.
func (p *GLM4VProvider) Analyze(
	ctx context.Context,
	img image.Image,
	prompt string,
) (*VisionResult, error) {
	started := time.Now()

	b64, err := adapters.ImageToBase64(img)
	if err != nil {
		return nil, fmt.Errorf("glm4v: encode image: %w", err)
	}

	body := chatRequest{
		Model: p.model,
		Messages: []chatMessage{
			{
				Role: "user",
				Content: []contentPart{
					{
						Type: "image_url",
						// Zhipu AI: raw base64 string, NO "data:image/png;base64," prefix.
						ImageURL: &imageURLPart{URL: b64},
					},
					{
						Type: "text",
						Text: prompt,
					},
				},
			},
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("glm4v: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		p.baseURL+"/chat/completions",
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, fmt.Errorf("glm4v: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("glm4v: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	rawBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("glm4v: read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("glm4v: API returned status %d: %s", resp.StatusCode, rawBytes)
	}

	var cr chatResponse
	if err := json.Unmarshal(rawBytes, &cr); err != nil {
		return nil, fmt.Errorf("glm4v: decode response: %w", err)
	}

	if len(cr.Choices) == 0 {
		return nil, fmt.Errorf("glm4v: API returned empty choices list")
	}

	var rawParsed interface{}
	_ = json.Unmarshal(rawBytes, &rawParsed)

	return &VisionResult{
		Text:        cr.Choices[0].Message.Content,
		RawResponse: rawParsed,
		Model:       p.model,
		Provider:    providerName,
		Timestamp:   started,
		Duration:    time.Since(started),
	}, nil
}

// HealthCheck performs a GET /models request to verify the provider is
// reachable and the API key is accepted.
func (p *GLM4VProvider) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		p.baseURL+"/models",
		nil,
	)
	if err != nil {
		return fmt.Errorf("glm4v: health check request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("glm4v: health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("glm4v: health check returned status %d: %s", resp.StatusCode, body)
	}
	return nil
}

// GetCapabilities returns the static capability and cost profile for GLM-4V.
// The flash model is free tier so CostPer1MTokens is 0.
func (p *GLM4VProvider) GetCapabilities() cheaper.ProviderCapabilities {
	return cheaper.ProviderCapabilities{
		MaxImageSize:    maxImageSize,
		SupportedFormats: []string{"png", "jpg", "jpeg", "webp"},
		AverageLatency:  avgLatency,
		CostPer1MTokens: 0,
	}
}

// GetCostEstimate returns 0.0 for the free glm-4v-flash model and 0.015 for
// the paid glm-4v model. imageSize and promptLength are accepted to satisfy
// the interface but are not used in the current pricing model.
func (p *GLM4VProvider) GetCostEstimate(imageSize int, promptLength int) float64 {
	if p.model == paidModel {
		return paidModelCost
	}
	return flashModelCost
}

// VisionResult is re-exported from the cheaper package for use in tests
// within this package without an import cycle.
type VisionResult = cheaper.VisionResult
