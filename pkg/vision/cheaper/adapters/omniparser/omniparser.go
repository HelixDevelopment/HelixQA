// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package omniparser provides a VisionProvider adapter for the OmniParser V2
// model served locally via a Gradio HTTP API. OmniParser returns structured
// UI element data — bounding boxes, element types, and captions — which makes
// it well-suited for autonomous UI navigation tasks.
package omniparser

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
	defaultAPIURL  = "http://localhost:7861/api/predict"
	defaultTimeout = 60 * time.Second
	providerName   = "omniparser-v2"
	modelName      = "OmniParser-V2"
)

// OmniParserProvider implements cheaper.VisionProvider using a locally-hosted
// OmniParser V2 Gradio endpoint. No API key is required because the service
// runs on the local network.
type OmniParserProvider struct {
	client  *http.Client
	apiURL  string
	timeout time.Duration
}

// gradioRequest is the JSON body sent to the Gradio /api/predict endpoint.
type gradioRequest struct {
	Data []string `json:"data"`
}

// gradioResponse is the JSON body returned by the Gradio endpoint. The first
// element of Data is a JSON string that encodes the parsed UI elements.
type gradioResponse struct {
	Data []string `json:"data"`
}

// NewOmniParserProvider constructs an OmniParserProvider from a configuration
// map.
//
// Recognised keys:
//
//	"api_url" (string, optional) — Gradio predict endpoint URL
//	                               (default: "http://localhost:7861/api/predict").
//	"timeout" (time.Duration or float64 seconds, optional) — Per-request
//	          HTTP timeout (default: 60s).
func NewOmniParserProvider(config map[string]interface{}) (cheaper.VisionProvider, error) {
	apiURL := defaultAPIURL
	if v, ok := config["api_url"].(string); ok && v != "" {
		apiURL = v
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

	return &OmniParserProvider{
		client:  &http.Client{Timeout: timeout},
		apiURL:  apiURL,
		timeout: timeout,
	}, nil
}

// Analyze encodes img to a base64 PNG string, submits it together with prompt
// to the OmniParser Gradio endpoint using the Gradio predict format, and
// returns a structured VisionResult. The response text contains JSON-encoded
// UI element data (bounding boxes, element types, captions).
func (p *OmniParserProvider) Analyze(
	ctx context.Context,
	img image.Image,
	prompt string,
) (*VisionResult, error) {
	start := time.Now()

	b64, err := adapters.ImageToBase64(img)
	if err != nil {
		return nil, fmt.Errorf("omniparser: image encoding failed: %w", err)
	}

	reqPayload := gradioRequest{
		Data: []string{b64, prompt},
	}

	bodyBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("omniparser: failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		p.apiURL,
		bytes.NewReader(bodyBytes),
	)
	if err != nil {
		return nil, fmt.Errorf("omniparser: failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("omniparser: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("omniparser: failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"omniparser: API returned status %d: %s",
			resp.StatusCode,
			string(respBytes),
		)
	}

	var gradioResp gradioResponse
	if err := json.Unmarshal(respBytes, &gradioResp); err != nil {
		return nil, fmt.Errorf("omniparser: failed to decode response: %w", err)
	}

	if len(gradioResp.Data) == 0 {
		return nil, fmt.Errorf("omniparser: API returned empty data array")
	}

	text := gradioResp.Data[0]
	duration := time.Since(start)

	return &VisionResult{
		Text:        text,
		RawResponse: gradioResp,
		Duration:    duration,
		Model:       modelName,
		Provider:    providerName,
		Timestamp:   start,
	}, nil
}

// Name returns the unique identifier for this provider.
func (p *OmniParserProvider) Name() string {
	return providerName
}

// HealthCheck performs a GET request to the base URL (the apiURL with the
// "/api/predict" suffix stripped) to verify that the Gradio service is
// reachable and returns a 2xx response.
func (p *OmniParserProvider) HealthCheck(ctx context.Context) error {
	baseURL := strings.TrimSuffix(p.apiURL, "/api/predict")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL, nil)
	if err != nil {
		return fmt.Errorf("omniparser: failed to create health-check request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("omniparser: health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("omniparser: health check returned status %d", resp.StatusCode)
	}
	return nil
}

// GetCapabilities returns the static capability and cost profile for
// OmniParser V2. The service is local so there is no monetary cost.
func (p *OmniParserProvider) GetCapabilities() cheaper.ProviderCapabilities {
	return cheaper.ProviderCapabilities{
		MaxImageSize:     15 * 1024 * 1024, // 15 MB
		SupportedFormats: []string{"png", "jpg", "jpeg"},
		AverageLatency:   800 * time.Millisecond,
		CostPer1MTokens:  0,
	}
}

// GetCostEstimate returns the estimated cost for a single Analyze call.
// OmniParser V2 is a locally-hosted model, so the monetary cost is effectively
// zero; a nominal value of 0.0019 USD is returned to reflect hosting overhead.
func (p *OmniParserProvider) GetCostEstimate(imageSize int, promptLength int) float64 {
	return 0.0019
}

// VisionResult re-exports cheaper.VisionResult so test code in this package
// can reference it without an additional import alias.
type VisionResult = cheaper.VisionResult
