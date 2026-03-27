// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	openaiDefaultBaseURL = "https://api.openai.com"
	openaiDefaultModel   = "gpt-4o"
	openaiHTTPTimeout    = 45 * time.Second
)

// openaiProvider implements Provider for the OpenAI API.
type openaiProvider struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

// openaiRequest is the JSON body sent to /v1/chat/completions.
type openaiRequest struct {
	Model     string         `json:"model"`
	MaxTokens int            `json:"max_tokens"`
	Messages  []openaiReqMsg `json:"messages"`
}

// openaiReqMsg is a single outbound message whose content is an
// array of typed content parts (text or image_url).
type openaiReqMsg struct {
	Role    string              `json:"role"`
	Content []openaiContentPart `json:"content"`
}

// openaiContentPart is one element of a request message content
// array.
type openaiContentPart struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	ImageURL *openaiImageURL `json:"image_url,omitempty"`
}

// openaiImageURL holds the URL (or base64 data URI) for an
// image_url content part.
type openaiImageURL struct {
	URL string `json:"url"`
}

// openaiMsg is a single inbound message as returned by the API.
// The OpenAI response delivers content as a plain string.
type openaiMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openaiResponse is the JSON body returned by /v1/chat/completions.
type openaiResponse struct {
	Model   string         `json:"model"`
	Choices []openaiChoice `json:"choices"`
	Usage   openaiUsage    `json:"usage"`
}

// openaiChoice is a single completion choice in the response.
type openaiChoice struct {
	Message openaiMsg `json:"message"`
}

// openaiUsage holds token counts from the API response.
type openaiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// NewOpenAIProvider constructs a Provider backed by the OpenAI API.
// BaseURL defaults to https://api.openai.com; Model defaults to
// gpt-4o.
func NewOpenAIProvider(cfg ProviderConfig) Provider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = openaiDefaultBaseURL
	}
	model := cfg.Model
	if model == "" {
		model = openaiDefaultModel
	}
	return &openaiProvider{
		apiKey:  cfg.APIKey,
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{Timeout: openaiHTTPTimeout},
	}
}

// Name returns the canonical provider identifier.
func (p *openaiProvider) Name() string {
	return ProviderOpenAI
}

// SupportsVision reports that OpenAI GPT-4o supports image inputs.
func (p *openaiProvider) SupportsVision() bool {
	return true
}

// Chat sends a multi-turn conversation to the OpenAI chat
// completions API and returns the assistant reply.
func (p *openaiProvider) Chat(
	ctx context.Context,
	messages []Message,
) (*Response, error) {
	var msgs []openaiReqMsg
	for _, m := range messages {
		msgs = append(msgs, openaiReqMsg{
			Role: m.Role,
			Content: []openaiContentPart{
				{Type: "text", Text: m.Content},
			},
		})
	}
	req := openaiRequest{
		Model:     p.model,
		MaxTokens: 4096,
		Messages:  msgs,
	}
	return p.doRequest(ctx, req)
}

// Vision sends a screenshot with a text prompt to the OpenAI chat
// completions API using an image_url content part carrying a
// base64 data URI.
func (p *openaiProvider) Vision(
	ctx context.Context,
	image []byte,
	prompt string,
) (*Response, error) {
	encoded := base64.StdEncoding.EncodeToString(image)
	dataURI := "data:image/png;base64," + encoded

	req := openaiRequest{
		Model:     p.model,
		MaxTokens: 4096,
		Messages: []openaiReqMsg{
			{
				Role: RoleUser,
				Content: []openaiContentPart{
					{
						Type:     "image_url",
						ImageURL: &openaiImageURL{URL: dataURI},
					},
					{
						Type: "text",
						Text: prompt,
					},
				},
			},
		},
	}
	return p.doRequest(ctx, req)
}

// doRequest serialises req, POSTs to /v1/chat/completions, and
// parses the response into a *Response.
func (p *openaiProvider) doRequest(
	ctx context.Context,
	req openaiRequest,
) (*Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("openai: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		p.baseURL+"/v1/chat/completions",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("openai: create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai: send request: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("openai: read response body: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"openai: API error %d: %s",
			httpResp.StatusCode,
			string(respBody),
		)
	}

	var apiResp openaiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("openai: decode response: %w", err)
	}

	var content string
	if len(apiResp.Choices) > 0 {
		content = apiResp.Choices[0].Message.Content
	}

	return &Response{
		Content:      content,
		Model:        apiResp.Model,
		InputTokens:  apiResp.Usage.PromptTokens,
		OutputTokens: apiResp.Usage.CompletionTokens,
	}, nil
}
