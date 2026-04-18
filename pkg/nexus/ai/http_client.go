package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HTTPLLMClient is a concrete LLMClient that talks to any OpenAI-
// compatible chat-completions endpoint (OpenAI, Anthropic via proxy,
// LLMOrchestrator gateway, llama.cpp RPC wrapped behind an HTTP front-
// end, ...). Operators point it at their preferred endpoint; Nexus
// itself does not bake in an SDK dependency so the package stays
// portable.
type HTTPLLMClient struct {
	Endpoint     string // e.g. https://llm-orchestrator.local/v1/chat/completions
	APIKey       string // optional Bearer token
	DefaultModel string
	HTTP         *http.Client

	// SSRFGuard runs on every Chat() call before any bytes leave the
	// process. Zero value = private / loopback / link-local /
	// metadata endpoints are all rejected, matching the
	// "SSRF Defense" guidance from tldrsec/awesome-secure-defaults.
	// Operators running an internal-only LLM flip
	// AllowPrivateNetworks=true explicitly.
	SSRFGuard SSRFGuardConfig
}

// NewHTTPLLMClient returns a client with a sensible default timeout
// and JSON content-type.
func NewHTTPLLMClient(endpoint, apiKey, defaultModel string) *HTTPLLMClient {
	return &HTTPLLMClient{
		Endpoint:     strings.TrimRight(endpoint, "/"),
		APIKey:       apiKey,
		DefaultModel: defaultModel,
		HTTP:         &http.Client{Timeout: 60 * time.Second},
	}
}

// Chat implements LLMClient.
func (c *HTTPLLMClient) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	if c.Endpoint == "" {
		return ChatResponse{}, errors.New("http llm: endpoint is required")
	}
	model := req.Model
	if model == "" {
		model = c.DefaultModel
	}
	if model == "" {
		return ChatResponse{}, errors.New("http llm: model is required")
	}
	// SSRF gate runs after the cheap endpoint + model checks so
	// callers with misconfigured fields fast-fail without waiting
	// on DNS, but before any bytes leave the process.
	if err := ValidateURL(c.Endpoint, c.SSRFGuard); err != nil {
		return ChatResponse{}, fmt.Errorf("http llm: %w", err)
	}
	body := map[string]any{
		"model":       model,
		"messages":    buildMessages(req),
		"max_tokens":  maxTokensOrDefault(req.MaxTokens),
		"temperature": req.Temperature,
	}
	if req.JSONResponse {
		body["response_format"] = map[string]string{"type": "json_object"}
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("marshal request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint, bytes.NewReader(raw))
	if err != nil {
		return ChatResponse{}, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("llm call: %w", err)
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode >= 400 {
		return ChatResponse{}, fmt.Errorf("llm %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	var parsed chatAPIResponse
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return ChatResponse{}, fmt.Errorf("decode llm response: %w: body=%s", err, string(payload))
	}
	if len(parsed.Choices) == 0 {
		return ChatResponse{}, errors.New("llm response: no choices")
	}
	return ChatResponse{
		Text:      parsed.Choices[0].Message.Content,
		Provider:  parsed.Provider,
		Model:     parsed.Model,
		TokensIn:  parsed.Usage.PromptTokens,
		TokensOut: parsed.Usage.CompletionTokens,
		CostUSD:   parsed.Usage.CostUSD,
	}, nil
}

// ChatResponse already implements what we need; these internal types
// mirror the minimal OpenAI-compatible envelope.
type chatAPIResponse struct {
	Provider string
	Model    string `json:"model"`
	Choices  []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int     `json:"prompt_tokens"`
		CompletionTokens int     `json:"completion_tokens"`
		CostUSD          float64 `json:"cost_usd"`
	} `json:"usage"`
}

func buildMessages(req ChatRequest) []map[string]any {
	msgs := []map[string]any{}
	if req.SystemPrompt != "" {
		msgs = append(msgs, map[string]any{
			"role": "system", "content": req.SystemPrompt,
		})
	}
	user := map[string]any{"role": "user"}
	if len(req.ImageBase64) == 0 {
		user["content"] = req.UserPrompt
	} else {
		// Multi-modal: content is an array of parts (text + image_url).
		parts := []map[string]any{{"type": "text", "text": req.UserPrompt}}
		for _, b64 := range req.ImageBase64 {
			parts = append(parts, map[string]any{
				"type":      "image_url",
				"image_url": map[string]string{"url": "data:image/png;base64," + b64},
			})
		}
		user["content"] = parts
	}
	msgs = append(msgs, user)
	return msgs
}

func maxTokensOrDefault(n int) int {
	if n <= 0 {
		return 1024
	}
	return n
}
