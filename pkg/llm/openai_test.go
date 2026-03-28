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

func TestOpenAIProvider_Name(t *testing.T) {
	p := NewOpenAIProvider(ProviderConfig{
		Name:   ProviderOpenAI,
		APIKey: "sk-test",
		Model:  "gpt-4o",
	})
	assert.Equal(t, ProviderOpenAI, p.Name())
}

func TestOpenAIProvider_SupportsVision(t *testing.T) {
	p := NewOpenAIProvider(ProviderConfig{
		Name:   ProviderOpenAI,
		APIKey: "sk-test",
		Model:  "gpt-4o",
	})
	assert.True(t, p.SupportsVision())
}

func TestOpenAIProvider_SupportsVision_TextOnly(t *testing.T) {
	// DeepSeek and Groq are text-only and should NOT report
	// vision support, even though they use the OpenAI API format.
	for _, name := range []string{ProviderDeepSeek, ProviderGroq, "cerebras", "siliconflow"} {
		p := NewOpenAIProvider(ProviderConfig{
			Name:   name,
			APIKey: "sk-test",
		})
		assert.False(t, p.SupportsVision(), "provider %q should not support vision", name)
		assert.Equal(t, name, p.Name(), "provider should preserve its name")
	}
}

func TestOpenAIProvider_Chat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer sk-openai-test-key", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		resp := openaiResponse{
			Model: "gpt-4o",
			Choices: []openaiChoice{
				{
					Message: openaiMsg{
						Role:    RoleAssistant,
						Content: "QA analysis complete.",
					},
				},
			},
			Usage: openaiUsage{
				PromptTokens:     42,
				CompletionTokens: 15,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := NewOpenAIProvider(ProviderConfig{
		Name:    ProviderOpenAI,
		APIKey:  "sk-openai-test-key",
		Model:   "gpt-4o",
		BaseURL: srv.URL,
	})

	messages := []Message{
		{Role: RoleSystem, Content: "You are a QA agent."},
		{Role: RoleUser, Content: "Analyze this test failure."},
	}

	resp, err := p.Chat(context.Background(), messages)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "QA analysis complete.", resp.Content)
	assert.Equal(t, "gpt-4o", resp.Model)
	assert.Equal(t, 42, resp.InputTokens)
	assert.Equal(t, 15, resp.OutputTokens)
}

func TestOpenAIProvider_Chat_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = fmt.Fprint(w, `{"error":{"type":"rate_limit_error","message":"Too many requests"}}`)
	}))
	defer srv.Close()

	p := NewOpenAIProvider(ProviderConfig{
		Name:    ProviderOpenAI,
		APIKey:  "sk-openai-test-key",
		Model:   "gpt-4o",
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

func TestOpenAIProvider_Vision(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer sk-openai-test-key", r.Header.Get("Authorization"))

		// Verify the request body contains image_url content part
		var reqBody openaiRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)
		require.Len(t, reqBody.Messages, 1)
		require.Len(t, reqBody.Messages[0].Content, 2)
		assert.Equal(t, "image_url", reqBody.Messages[0].Content[0].Type)
		assert.Equal(t, "text", reqBody.Messages[0].Content[1].Type)
		assert.NotEmpty(t, reqBody.Messages[0].Content[0].ImageURL)

		resp := openaiResponse{
			Model: "gpt-4o",
			Choices: []openaiChoice{
				{
					Message: openaiMsg{
						Role:    RoleAssistant,
						Content: "I see a test failure screenshot.",
					},
				},
			},
			Usage: openaiUsage{
				PromptTokens:     100,
				CompletionTokens: 20,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := NewOpenAIProvider(ProviderConfig{
		Name:    ProviderOpenAI,
		APIKey:  "sk-openai-test-key",
		Model:   "gpt-4o",
		BaseURL: srv.URL,
	})

	imageBytes := []byte{0xFF, 0xD8, 0xFF, 0xE0} // fake JPEG header
	resp, err := p.Vision(context.Background(), imageBytes, "What do you see?")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "I see a test failure screenshot.", resp.Content)
	assert.Equal(t, 100, resp.InputTokens)
	assert.Equal(t, 20, resp.OutputTokens)
}
