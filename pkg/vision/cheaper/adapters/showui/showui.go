// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package showui provides a VisionProvider adapter for ShowUI-2B, a
// locally-hosted GUI-understanding model served via Gradio's /api/predict
// endpoint. No API key is required; the model runs entirely on-premises.
package showui

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
	defaultAPIURL = "http://localhost:7860/api/predict"
	defaultTimeout = 30 * time.Second

	providerName = "showui-2b"
	modelName    = "ShowUI-2B"

	maxImageSize   = 10 * 1024 * 1024 // 10 MB
	avgLatency     = 500 * time.Millisecond
)

// ShowUIProvider sends screenshots to a locally-hosted ShowUI-2B model via
// the Gradio HTTP API and returns structured VisionResult values.
// All exported methods are safe for concurrent use.
type ShowUIProvider struct {
	client  *http.Client
	apiURL  string
	timeout time.Duration
}

// NewShowUIProvider creates a new ShowUIProvider from the supplied config map.
//
// Accepted keys:
//   - "api_url"  (string)        — Gradio predict endpoint
//                                  (default: "http://localhost:7860/api/predict")
//   - "timeout"  (time.Duration) — per-request HTTP timeout (default: 30s)
//
// No API key is required because ShowUI-2B runs locally.
func NewShowUIProvider(config map[string]interface{}) (cheaper.VisionProvider, error) {
	apiURL := defaultAPIURL
	if v, ok := config["api_url"]; ok {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("showui: 'api_url' must be a string")
		}
		if s != "" {
			apiURL = s
		}
	}

	timeout := defaultTimeout
	if v, ok := config["timeout"]; ok {
		d, ok := v.(time.Duration)
		if !ok {
			return nil, fmt.Errorf("showui: 'timeout' must be a time.Duration")
		}
		if d > 0 {
			timeout = d
		}
	}

	return &ShowUIProvider{
		client:  &http.Client{Timeout: timeout},
		apiURL:  apiURL,
		timeout: timeout,
	}, nil
}

// gradioRequest is the JSON body sent to the Gradio /api/predict endpoint.
type gradioRequest struct {
	Data []string `json:"data"`
}

// gradioResponse is the JSON body received from the Gradio /api/predict
// endpoint on success.
type gradioResponse struct {
	Data []string `json:"data"`
}

// Analyze encodes img to base64 PNG, posts it together with prompt to the
// ShowUI-2B Gradio endpoint, and returns a VisionResult.
func (p *ShowUIProvider) Analyze(
	ctx context.Context,
	img image.Image,
	prompt string,
) (*cheaper.VisionResult, error) {
	b64, err := adapters.ImageToBase64(img)
	if err != nil {
		return nil, fmt.Errorf("showui: encode image: %w", err)
	}

	payload := gradioRequest{Data: []string{b64, prompt}}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("showui: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("showui: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("showui: HTTP POST: %w", err)
	}
	defer resp.Body.Close()

	elapsed := time.Since(start)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("showui: API returned status %d: %s", resp.StatusCode, string(raw))
	}

	var gr gradioResponse
	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		return nil, fmt.Errorf("showui: decode response: %w", err)
	}

	if len(gr.Data) == 0 {
		return nil, fmt.Errorf("showui: response 'data' array is empty")
	}

	return &cheaper.VisionResult{
		Text:        gr.Data[0],
		RawResponse: gr,
		Duration:    elapsed,
		Model:       modelName,
		Provider:    providerName,
		Timestamp:   start,
	}, nil
}

// Name returns the unique registered identifier for this provider.
func (p *ShowUIProvider) Name() string {
	return providerName
}

// HealthCheck performs a GET request to the ShowUI base URL (the Gradio web
// UI) to verify the service is reachable and responding. It returns an error
// for any non-2xx status code.
func (p *ShowUIProvider) HealthCheck(ctx context.Context) error {
	baseURL := strings.TrimSuffix(p.apiURL, "/api/predict")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL, nil)
	if err != nil {
		return fmt.Errorf("showui: health-check build request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("showui: health-check HTTP GET: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("showui: health-check returned status %d", resp.StatusCode)
	}

	return nil
}

// GetCapabilities returns the static capability and cost profile for
// ShowUI-2B. The returned value must not be mutated by callers.
func (p *ShowUIProvider) GetCapabilities() cheaper.ProviderCapabilities {
	return cheaper.ProviderCapabilities{
		MaxImageSize:     maxImageSize,
		SupportedFormats: []string{"png", "jpg", "jpeg"},
		AverageLatency:   avgLatency,
		CostPer1MTokens:  0,
	}
}

// GetCostEstimate always returns 0.0 because ShowUI-2B is a free,
// locally-hosted model with no per-token billing.
func (p *ShowUIProvider) GetCostEstimate(imageSize int, promptLength int) float64 {
	return 0.0
}
