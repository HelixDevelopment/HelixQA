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

func TestAnthropicProvider_Name(t *testing.T) {
	p := NewAnthropicProvider(ProviderConfig{
		Name:   ProviderAnthropic,
		APIKey: "sk-ant-test",
		Model:  "claude-sonnet-4-20250514",
	})
	assert.Equal(t, ProviderAnthropic, p.Name())
}

func TestAnthropicProvider_SupportsVision(t *testing.T) {
	p := NewAnthropicProvider(ProviderConfig{
		Name:   ProviderAnthropic,
		APIKey: "sk-ant-test",
		Model:  "claude-sonnet-4-20250514",
	})
	assert.True(t, p.SupportsVision())
}

func TestAnthropicProvider_Chat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/messages", r.URL.Path)
		assert.Equal(t, "sk-ant-test-key", r.Header.Get("x-api-key"))
		assert.Equal(t, "2023-06-01", r.Header.Get("anthropic-version"))
		assert.Equal(t, "application/json", r.Header.Get("content-type"))

		resp := anthropicResponse{
			Model: "claude-sonnet-4-20250514",
			Content: []anthropicContent{
				{Type: "text", Text: "QA analysis complete."},
			},
			Usage: anthropicUsage{
				InputTokens:  42,
				OutputTokens: 15,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := NewAnthropicProvider(ProviderConfig{
		Name:    ProviderAnthropic,
		APIKey:  "sk-ant-test-key",
		Model:   "claude-sonnet-4-20250514",
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
	assert.Equal(t, "claude-sonnet-4-20250514", resp.Model)
	assert.Equal(t, 42, resp.InputTokens)
	assert.Equal(t, 15, resp.OutputTokens)
}

func TestAnthropicProvider_Chat_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = fmt.Fprint(w, `{"error":{"type":"rate_limit_error","message":"Too many requests"}}`)
	}))
	defer srv.Close()

	p := NewAnthropicProvider(ProviderConfig{
		Name:    ProviderAnthropic,
		APIKey:  "sk-ant-test-key",
		Model:   "claude-sonnet-4-20250514",
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

func TestAnthropicProvider_Vision(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/messages", r.URL.Path)
		assert.Equal(t, "sk-ant-test-key", r.Header.Get("x-api-key"))
		assert.Equal(t, "2023-06-01", r.Header.Get("anthropic-version"))

		// Verify the request body contains image content
		var reqBody anthropicRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)
		require.Len(t, reqBody.Messages, 1)
		require.Len(t, reqBody.Messages[0].Content, 2)
		assert.Equal(t, "image", reqBody.Messages[0].Content[0].Type)
		assert.Equal(t, "text", reqBody.Messages[0].Content[1].Type)

		resp := anthropicResponse{
			Model: "claude-sonnet-4-20250514",
			Content: []anthropicContent{
				{Type: "text", Text: "I see a test failure screenshot."},
			},
			Usage: anthropicUsage{
				InputTokens:  100,
				OutputTokens: 20,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := NewAnthropicProvider(ProviderConfig{
		Name:    ProviderAnthropic,
		APIKey:  "sk-ant-test-key",
		Model:   "claude-sonnet-4-20250514",
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
