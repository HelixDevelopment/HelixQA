// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAsticaProvider_Name(t *testing.T) {
	p := NewAsticaProvider(ProviderConfig{
		Name:   "astica",
		APIKey: "test-key",
	})
	assert.Equal(t, "astica", p.Name())
}

func TestAsticaProvider_SupportsVision(t *testing.T) {
	p := NewAsticaProvider(ProviderConfig{
		Name:   "astica",
		APIKey: "test-key",
	})
	assert.True(t, p.SupportsVision())
}

func TestAsticaProvider_Chat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		// No Authorization header — token is in the body
		assert.Empty(t, r.Header.Get("Authorization"))

		var reqBody asticaRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)
		assert.Equal(t, "test-api-key", reqBody.Token)
		assert.Equal(t, "2.5_full", reqBody.ModelVersion)
		assert.NotEmpty(t, reqBody.GPTPrompt)

		resp := asticaResponse{
			Status:     "success",
			CaptionGPT: "Analysis of the QA test scenario.",
		}
		resp.Caption.Text = "fallback caption"
		resp.Caption.Confidence = 0.90
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := NewAsticaProvider(ProviderConfig{
		Name:    "astica",
		APIKey:  "test-api-key",
		BaseURL: srv.URL,
	})

	messages := []Message{
		{Role: RoleUser, Content: "Analyze this test."},
	}

	resp, err := p.Chat(context.Background(), messages)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "Analysis of the QA test scenario.", resp.Content)
	assert.Equal(t, "2.5_full", resp.Model)
}

func TestAsticaProvider_Chat_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"rate limited"}`))
	}))
	defer srv.Close()

	p := NewAsticaProvider(ProviderConfig{
		Name:    "astica",
		APIKey:  "test-key",
		BaseURL: srv.URL,
	})

	messages := []Message{
		{Role: RoleUser, Content: "Hello"},
	}

	resp, err := p.Chat(context.Background(), messages)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "429")
}

func TestAsticaProvider_Chat_NoUserContent(t *testing.T) {
	p := NewAsticaProvider(ProviderConfig{
		Name:   "astica",
		APIKey: "test-key",
	})

	messages := []Message{
		{Role: RoleAssistant, Content: "I am an assistant."},
	}

	resp, err := p.Chat(context.Background(), messages)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "no user content")
}

func TestAsticaProvider_Vision(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var reqBody asticaRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)
		assert.Equal(t, "test-api-key", reqBody.Token)
		assert.Equal(t, "2.5_full", reqBody.ModelVersion)
		assert.Contains(t, reqBody.Input, "data:image/png;base64,")
		assert.Equal(t, "describe,objects,faces,text", reqBody.VisionParams)
		assert.NotEmpty(t, reqBody.GPTPrompt)

		resp := asticaResponse{
			Status:     "success",
			CaptionGPT: "I see a test failure screenshot with error logs.",
		}
		resp.Caption.Text = "A screenshot"
		resp.Caption.Confidence = 0.95
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := NewAsticaProvider(ProviderConfig{
		Name:    "astica",
		APIKey:  "test-api-key",
		BaseURL: srv.URL,
	})

	imageBytes := []byte{0xFF, 0xD8, 0xFF, 0xE0} // fake JPEG header
	resp, err := p.Vision(context.Background(), imageBytes, "What do you see?")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "I see a test failure screenshot with error logs.", resp.Content)
	assert.Equal(t, "2.5_full", resp.Model)
}

func TestAsticaProvider_Vision_FallbackCaption(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := asticaResponse{
			Status:     "success",
			CaptionGPT: "",
		}
		resp.Caption.Text = "Fallback standard caption"
		resp.Caption.Confidence = 0.88
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := NewAsticaProvider(ProviderConfig{
		Name:    "astica",
		APIKey:  "test-key",
		BaseURL: srv.URL,
	})

	resp, err := p.Vision(context.Background(), []byte("img"), "Describe")
	require.NoError(t, err)
	assert.Equal(t, "Fallback standard caption", resp.Content)
}

func TestAsticaProvider_Vision_FailedStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := asticaResponse{
			Status: "error",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := NewAsticaProvider(ProviderConfig{
		Name:    "astica",
		APIKey:  "test-key",
		BaseURL: srv.URL,
	})

	resp, err := p.Vision(context.Background(), []byte("img"), "Describe")
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "error")
}

func TestAsticaProvider_Vision_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not valid json"))
	}))
	defer srv.Close()

	p := NewAsticaProvider(ProviderConfig{
		Name:    "astica",
		APIKey:  "test-key",
		BaseURL: srv.URL,
	})

	resp, err := p.Vision(context.Background(), []byte("img"), "Describe")
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "decode response")
}

func TestAsticaProvider_CustomModel(t *testing.T) {
	var receivedBody asticaRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		resp := asticaResponse{
			Status:     "success",
			CaptionGPT: "ok",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := NewAsticaProvider(ProviderConfig{
		Name:    "astica",
		APIKey:  "test-key",
		BaseURL: srv.URL,
		Model:   "2.1_full",
	})

	_, _ = p.Vision(context.Background(), []byte("img"), "test")
	assert.Equal(t, "2.1_full", receivedBody.ModelVersion)
}

func TestAsticaProvider_IsNotOpenAICompatible(t *testing.T) {
	assert.False(t, IsOpenAICompatible("astica"))
}

func TestAsticaProvider_InterfaceCompliance(t *testing.T) {
	var _ Provider = (*asticaProvider)(nil)
}
