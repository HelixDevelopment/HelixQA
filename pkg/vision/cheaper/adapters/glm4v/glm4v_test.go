// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package glm4v

import (
	"context"
	"encoding/json"
	"image"
	"image/color"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestImage creates a small solid-colour RGBA image for use in tests.
func newTestImage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 100, G: 149, B: 237, A: 255})
		}
	}
	return img
}

// chatCompletionResponse builds a minimal OpenAI-compatible chat completion
// JSON response for mock servers to return.
func chatCompletionResponse(content string) interface{} {
	return map[string]interface{}{
		"id":      "chatcmpl-test",
		"object":  "chat.completion",
		"created": 1700000000,
		"model":   "glm-4v-flash",
		"choices": []interface{}{
			map[string]interface{}{
				"index": 0,
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": content,
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]interface{}{
			"prompt_tokens":     100,
			"completion_tokens": 50,
			"total_tokens":      150,
		},
	}
}

// --- Constructor tests ---

func TestNewGLM4VProvider_MissingKey(t *testing.T) {
	provider, err := NewGLM4VProvider(map[string]interface{}{})

	assert.Nil(t, provider)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "api_key")
}

func TestNewGLM4VProvider_Defaults(t *testing.T) {
	provider, err := NewGLM4VProvider(map[string]interface{}{
		"api_key": "test-key",
	})

	require.NoError(t, err)
	require.NotNil(t, provider)

	p := provider.(*GLM4VProvider)
	assert.Equal(t, "test-key", p.apiKey)
	assert.Equal(t, "https://open.bigmodel.cn/api/paas/v4", p.baseURL)
	assert.Equal(t, "glm-4v-flash", p.model)
	assert.Equal(t, 60*time.Second, p.timeout)
}

func TestNewGLM4VProvider_CustomValues(t *testing.T) {
	provider, err := NewGLM4VProvider(map[string]interface{}{
		"api_key":  "custom-key",
		"base_url": "https://custom.endpoint.example.com/v1",
		"model":    "glm-4v",
		"timeout":  float64(30),
	})

	require.NoError(t, err)
	require.NotNil(t, provider)

	p := provider.(*GLM4VProvider)
	assert.Equal(t, "custom-key", p.apiKey)
	assert.Equal(t, "https://custom.endpoint.example.com/v1", p.baseURL)
	assert.Equal(t, "glm-4v", p.model)
	assert.Equal(t, 30*time.Second, p.timeout)
}

// --- Analyze tests ---

func TestGLM4V_Analyze_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(chatCompletionResponse("The screen shows a home page."))
	}))
	defer srv.Close()

	provider, err := NewGLM4VProvider(map[string]interface{}{
		"api_key":  "test-key",
		"base_url": srv.URL,
	})
	require.NoError(t, err)

	img := newTestImage(64, 64)
	result, err := provider.Analyze(context.Background(), img, "Describe the screen.")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "The screen shows a home page.", result.Text)
	assert.Equal(t, "glm-4v", result.Provider)
	assert.NotEmpty(t, result.Model)
	assert.False(t, result.Timestamp.IsZero())
	assert.Greater(t, result.Duration, time.Duration(0))
}

// TestGLM4V_Analyze_NoPrefix verifies that the request body sent to Zhipu AI
// contains the raw base64 string for the image URL value — specifically, it
// must NOT include the "data:image/png;base64," prefix that standard data URIs
// use. This is a Zhipu-specific API requirement.
func TestGLM4V_Analyze_NoPrefix(t *testing.T) {
	var capturedBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(chatCompletionResponse("ok"))
	}))
	defer srv.Close()

	provider, err := NewGLM4VProvider(map[string]interface{}{
		"api_key":  "test-key",
		"base_url": srv.URL,
	})
	require.NoError(t, err)

	img := newTestImage(8, 8)
	_, err = provider.Analyze(context.Background(), img, "check")
	require.NoError(t, err)

	// Walk the messages to find the image_url value.
	messages, ok := capturedBody["messages"].([]interface{})
	require.True(t, ok, "messages must be an array")

	found := false
	for _, m := range messages {
		msg, ok := m.(map[string]interface{})
		if !ok {
			continue
		}
		contents, ok := msg["content"].([]interface{})
		if !ok {
			continue
		}
		for _, c := range contents {
			part, ok := c.(map[string]interface{})
			if !ok || part["type"] != "image_url" {
				continue
			}
			imageURL, ok := part["image_url"].(map[string]interface{})
			require.True(t, ok, "image_url must be an object")
			urlVal, ok := imageURL["url"].(string)
			require.True(t, ok, "url must be a string")

			// The value must NOT start with the data URI prefix.
			assert.False(
				t,
				strings.HasPrefix(urlVal, "data:image/png;base64,"),
				"Zhipu GLM-4V expects raw base64 without data URI prefix, got: %s",
				urlVal[:min(len(urlVal), 40)],
			)
			found = true
		}
	}
	assert.True(t, found, "image_url part must be present in request body")
}

// min returns the smaller of a and b.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestGLM4V_Analyze_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	}))
	defer srv.Close()

	provider, err := NewGLM4VProvider(map[string]interface{}{
		"api_key":  "bad-key",
		"base_url": srv.URL,
	})
	require.NoError(t, err)

	img := newTestImage(8, 8)
	result, err := provider.Analyze(context.Background(), img, "test")

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestGLM4V_Analyze_EmptyChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":      "chatcmpl-empty",
			"object":  "chat.completion",
			"choices": []interface{}{},
		})
	}))
	defer srv.Close()

	provider, err := NewGLM4VProvider(map[string]interface{}{
		"api_key":  "test-key",
		"base_url": srv.URL,
	})
	require.NoError(t, err)

	img := newTestImage(8, 8)
	result, err := provider.Analyze(context.Background(), img, "test")

	assert.Error(t, err)
	assert.Nil(t, result)
}

// --- HealthCheck tests ---

func TestGLM4V_HealthCheck(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/models", r.URL.Path)
			assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[]}`))
		}))
		defer srv.Close()

		provider, err := NewGLM4VProvider(map[string]interface{}{
			"api_key":  "test-key",
			"base_url": srv.URL,
		})
		require.NoError(t, err)

		err = provider.HealthCheck(context.Background())
		assert.NoError(t, err)
	})

	t.Run("failure", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer srv.Close()

		provider, err := NewGLM4VProvider(map[string]interface{}{
			"api_key":  "test-key",
			"base_url": srv.URL,
		})
		require.NoError(t, err)

		err = provider.HealthCheck(context.Background())
		assert.Error(t, err)
	})
}

// --- Name test ---

func TestGLM4V_Name(t *testing.T) {
	provider, err := NewGLM4VProvider(map[string]interface{}{
		"api_key": "test-key",
	})
	require.NoError(t, err)

	assert.Equal(t, "glm-4v", provider.Name())
}

// --- GetCapabilities test ---

func TestGLM4V_GetCapabilities(t *testing.T) {
	provider, err := NewGLM4VProvider(map[string]interface{}{
		"api_key": "test-key",
	})
	require.NoError(t, err)

	caps := provider.GetCapabilities()

	assert.Equal(t, 10*1024*1024, caps.MaxImageSize, "MaxImageSize should be 10 MB")
	assert.ElementsMatch(t, []string{"png", "jpg", "jpeg", "webp"}, caps.SupportedFormats)
	assert.Equal(t, 1*time.Second, caps.AverageLatency)
	assert.InDelta(t, 0.0, caps.CostPer1MTokens, 0.0001,
		"glm-4v-flash is a free tier model; CostPer1MTokens should be 0")
}

// --- GetCostEstimate tests ---

func TestGLM4V_GetCostEstimate_Flash(t *testing.T) {
	provider, err := NewGLM4VProvider(map[string]interface{}{
		"api_key": "test-key",
		"model":   "glm-4v-flash",
	})
	require.NoError(t, err)

	cost := provider.GetCostEstimate(1024*1024, 200)
	assert.InDelta(t, 0.0, cost, 0.0001, "glm-4v-flash should cost 0.0")
}

func TestGLM4V_GetCostEstimate_Paid(t *testing.T) {
	provider, err := NewGLM4VProvider(map[string]interface{}{
		"api_key": "test-key",
		"model":   "glm-4v",
	})
	require.NoError(t, err)

	cost := provider.GetCostEstimate(1024*1024, 200)
	assert.InDelta(t, 0.015, cost, 0.0001, "glm-4v paid model should cost 0.015")
}
