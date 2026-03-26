// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOllamaProvider_Name(t *testing.T) {
	p := NewOllamaProvider(ProviderConfig{
		Name:    ProviderOllama,
		BaseURL: "http://localhost:11434",
		Model:   "qwen2.5",
	})
	assert.Equal(t, ProviderOllama, p.Name())
}

func TestOllamaProvider_SupportsVision(t *testing.T) {
	p := NewOllamaProvider(ProviderConfig{
		Name:    ProviderOllama,
		BaseURL: "http://localhost:11434",
		Model:   "qwen2.5",
	})
	assert.True(t, p.SupportsVision())
}

func TestOllamaProvider_Chat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/chat", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Verify stream: false is set
		var reqBody ollamaChatRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)
		assert.False(t, reqBody.Stream)
		assert.NotEmpty(t, reqBody.Messages)

		resp := ollamaChatResponse{
			Model: "qwen2.5",
			Message: ollamaMsg{
				Role:    RoleAssistant,
				Content: "QA analysis complete.",
			},
			PromptEvalCount: 42,
			EvalCount:       15,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := NewOllamaProvider(ProviderConfig{
		Name:    ProviderOllama,
		BaseURL: srv.URL,
		Model:   "qwen2.5",
	})

	messages := []Message{
		{Role: RoleSystem, Content: "You are a QA agent."},
		{Role: RoleUser, Content: "Analyze this test failure."},
	}

	resp, err := p.Chat(context.Background(), messages)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "QA analysis complete.", resp.Content)
	assert.Equal(t, "qwen2.5", resp.Model)
	assert.Equal(t, 42, resp.InputTokens)
	assert.Equal(t, 15, resp.OutputTokens)
}

func TestOllamaProvider_Chat_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"error":"model not found"}`)
	}))
	defer srv.Close()

	p := NewOllamaProvider(ProviderConfig{
		Name:    ProviderOllama,
		BaseURL: srv.URL,
		Model:   "qwen2.5",
	})

	messages := []Message{
		{Role: RoleUser, Content: "Hello"},
	}

	resp, err := p.Chat(context.Background(), messages)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "500")
}

func TestOllamaProvider_Vision(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/chat", r.URL.Path)

		// Verify the request body contains images array
		var reqBody ollamaChatRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)
		require.Len(t, reqBody.Messages, 1)
		assert.NotEmpty(t, reqBody.Messages[0].Images)

		resp := ollamaChatResponse{
			Model: "qwen2.5",
			Message: ollamaMsg{
				Role:    RoleAssistant,
				Content: "I see a test failure screenshot.",
			},
			PromptEvalCount: 100,
			EvalCount:       20,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := NewOllamaProvider(ProviderConfig{
		Name:    ProviderOllama,
		BaseURL: srv.URL,
		Model:   "qwen2.5",
	})

	imageBytes := []byte{0xFF, 0xD8, 0xFF, 0xE0} // fake JPEG header
	resp, err := p.Vision(context.Background(), imageBytes, "What do you see?")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "I see a test failure screenshot.", resp.Content)
	assert.Equal(t, 100, resp.InputTokens)
	assert.Equal(t, 20, resp.OutputTokens)
}
