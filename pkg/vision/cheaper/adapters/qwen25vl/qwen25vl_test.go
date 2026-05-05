// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package qwen25vl

import (
	"context"
	"encoding/json"
	"image"
	"image/color"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestImage creates a small RGBA image for use in tests.
func newTestImage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 128, A: 255})
		}
	}
	return img
}

func TestNewQwen25VLProvider_CustomConfig(t *testing.T) {
	cfg := map[string]interface{}{
		"base_url": "http://myhost:8080/v1",
		"model":    "Qwen2.5-VL-72B-Instruct",
		"timeout":  float64(60),
	}

	provider, err := NewQwen25VLProvider(cfg)

	require.NoError(t, err)
	p, ok := provider.(*Qwen25VLProvider)
	require.True(t, ok)
	assert.Equal(t, "http://myhost:8080/v1", p.baseURL)
	assert.Equal(t, "Qwen2.5-VL-72B-Instruct", p.model)
	assert.Equal(t, 60*time.Second, p.timeout)
}

func TestNewQwen25VLProvider_EmptyConfig(t *testing.T) {
	provider, err := NewQwen25VLProvider(map[string]interface{}{})

	require.NoError(t, err)
	p, ok := provider.(*Qwen25VLProvider)
	require.True(t, ok)
	assert.Equal(t, "http://localhost:9192/v1", p.baseURL)
	assert.Equal(t, "Qwen2.5-VL-7B-Instruct", p.model)
	assert.Equal(t, 120*time.Second, p.timeout)
}

func TestQwen25VL_Name(t *testing.T) {
	provider, err := NewQwen25VLProvider(nil)
	require.NoError(t, err)

	assert.Equal(t, "qwen2.5-vl", provider.Name())
}

func TestQwen25VL_GetCapabilities(t *testing.T) {
	provider, err := NewQwen25VLProvider(nil)
	require.NoError(t, err)

	caps := provider.GetCapabilities()

	assert.Equal(t, 20*1024*1024, caps.MaxImageSize)
	assert.Equal(t, []string{"png", "jpg", "jpeg", "webp", "gif"}, caps.SupportedFormats)
	assert.Equal(t, 3*time.Second, caps.AverageLatency)
	assert.InDelta(t, 0.0, caps.CostPer1MTokens, 0.0001)
	assert.False(t, caps.SupportsStreaming)
	assert.False(t, caps.SupportsBatch)
}

func TestQwen25VL_GetCostEstimate(t *testing.T) {
	provider, err := NewQwen25VLProvider(nil)
	require.NoError(t, err)

	cost := provider.GetCostEstimate(1024*1024, 100)

	assert.InDelta(t, 0.0001, cost, 0.000001)
}

func TestQwen25VL_Analyze_Success(t *testing.T) {
	responseBody := map[string]interface{}{
		"choices": []interface{}{
			map[string]interface{}{
				"message": map[string]interface{}{
					"content": "A login screen with username and password fields.",
				},
			},
		},
	}

	var capturedRequest map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/chat/completions", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		// No Authorization header expected for local model
		assert.Empty(t, r.Header.Get("Authorization"))

		require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedRequest))

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(responseBody))
	}))
	defer srv.Close()

	cfg := map[string]interface{}{"base_url": srv.URL}
	provider, err := NewQwen25VLProvider(cfg)
	require.NoError(t, err)

	img := newTestImage(4, 4)
	result, err := provider.Analyze(context.Background(), img, "Describe the screen")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "A login screen with username and password fields.", result.Text)
	assert.Equal(t, "qwen2.5-vl", result.Provider)
	assert.Equal(t, "Qwen2.5-VL-7B-Instruct", result.Model)
	assert.NotNil(t, result.RawResponse)
	assert.False(t, result.Timestamp.IsZero())
	assert.GreaterOrEqual(t, result.Duration, time.Duration(0))

	// Verify request body structure
	require.NotNil(t, capturedRequest)
	messages, ok := capturedRequest["messages"].([]interface{})
	require.True(t, ok)
	require.Len(t, messages, 1)
	msg := messages[0].(map[string]interface{})
	assert.Equal(t, "user", msg["role"])
	content := msg["content"].([]interface{})
	require.Len(t, content, 2)
	// First element: text
	textPart := content[0].(map[string]interface{})
	assert.Equal(t, "text", textPart["type"])
	assert.Equal(t, "Describe the screen", textPart["text"])
	// Second element: image_url
	imgPart := content[1].(map[string]interface{})
	assert.Equal(t, "image_url", imgPart["type"])
	imgURL := imgPart["image_url"].(map[string]interface{})
	urlVal, ok := imgURL["url"].(string)
	require.True(t, ok)
	assert.Contains(t, urlVal, "data:image/png;base64,")
}

func TestQwen25VL_Analyze_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer srv.Close()

	cfg := map[string]interface{}{"base_url": srv.URL}
	provider, err := NewQwen25VLProvider(cfg)
	require.NoError(t, err)

	img := newTestImage(4, 4)
	result, err := provider.Analyze(context.Background(), img, "Describe")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "500")
}

func TestQwen25VL_Analyze_EmptyChoices(t *testing.T) {
	responseBody := map[string]interface{}{
		"choices": []interface{}{},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(responseBody))
	}))
	defer srv.Close()

	cfg := map[string]interface{}{"base_url": srv.URL}
	provider, err := NewQwen25VLProvider(cfg)
	require.NoError(t, err)

	img := newTestImage(4, 4)
	result, err := provider.Analyze(context.Background(), img, "Describe")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "empty choices")
}

func TestQwen25VL_HealthCheck_Healthy(t *testing.T) {
	responseBody := map[string]interface{}{
		"data": []interface{}{
			map[string]interface{}{"id": "Qwen2.5-VL-7B-Instruct"},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/models", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(responseBody))
	}))
	defer srv.Close()

	cfg := map[string]interface{}{"base_url": srv.URL}
	provider, err := NewQwen25VLProvider(cfg)
	require.NoError(t, err)

	err = provider.HealthCheck(context.Background())

	assert.NoError(t, err)
}

func TestQwen25VL_HealthCheck_Unhealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	cfg := map[string]interface{}{"base_url": srv.URL}
	provider, err := NewQwen25VLProvider(cfg)
	require.NoError(t, err)

	err = provider.HealthCheck(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "503")
}
