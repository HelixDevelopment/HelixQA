// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package llm

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	lpmodels "digital.vasic.llmprovider/pkg/models"
)

// echoLLMProvider is a real, deterministic implementation of the
// digital.vasic.llmprovider/pkg/provider.LLMProvider interface used to
// prove the bridge's request/response translation end-to-end. It is
// NOT a mock framework — it is a concrete provider that computes a
// real response from the request, exercising the exact code paths a
// network provider would drive through the bridge.
type echoLLMProvider struct {
	vision  bool
	failErr error
	lastReq *lpmodels.LLMRequest
}

func (e *echoLLMProvider) Complete(_ context.Context, req *lpmodels.LLMRequest) (*lpmodels.LLMResponse, error) {
	e.lastReq = req
	if e.failErr != nil {
		return nil, e.failErr
	}
	// Echo the last message content back, prefixed, with a real
	// token count derived from the input.
	var content string
	if len(req.Messages) > 0 {
		content = req.Messages[len(req.Messages)-1].Content
	} else {
		content = req.Prompt
	}
	return &lpmodels.LLMResponse{
		Content:      "echo:" + content,
		ProviderName: "echo",
		TokensUsed:   len(strings.Fields(content)),
		FinishReason: "stop",
	}, nil
}

func (e *echoLLMProvider) CompleteStream(_ context.Context, _ *lpmodels.LLMRequest) (<-chan *lpmodels.LLMResponse, error) {
	ch := make(chan *lpmodels.LLMResponse)
	close(ch)
	return ch, nil
}

func (e *echoLLMProvider) HealthCheck() error { return e.failErr }

func (e *echoLLMProvider) GetCapabilities() *lpmodels.ProviderCapabilities {
	return &lpmodels.ProviderCapabilities{SupportsVision: e.vision}
}

func (e *echoLLMProvider) ValidateConfig(_ map[string]interface{}) (bool, []string) {
	return true, nil
}

// TestLLMProviderBridge_Chat proves the bridge translates a HelixQA
// message slice into an LLMProvider request, invokes the real
// Complete path, and maps the response back — a true end-to-end
// exercise of the bridge.
func TestLLMProviderBridge_Chat(t *testing.T) {
	impl := &echoLLMProvider{}
	b, err := NewLLMProviderBridge(impl, WithBridgeName("test-bridge"), WithBridgeModel("m1"))
	if err != nil {
		t.Fatalf("NewLLMProviderBridge: %v", err)
	}
	if b.Name() != "test-bridge" {
		t.Errorf("Name=%q want test-bridge", b.Name())
	}

	resp, err := b.Chat(context.Background(), []Message{
		{Role: RoleSystem, Content: "be terse"},
		{Role: RoleUser, Content: "hello world"},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Content != "echo:hello world" {
		t.Errorf("Content=%q want echo:hello world", resp.Content)
	}
	if resp.Model != "m1" {
		t.Errorf("Model=%q want m1", resp.Model)
	}
	if resp.OutputTokens != 2 {
		t.Errorf("OutputTokens=%d want 2", resp.OutputTokens)
	}
	// Verify the request the bridge built carried both messages and a
	// flattened prompt from the last user message.
	if len(impl.lastReq.Messages) != 2 {
		t.Errorf("bridge sent %d messages, want 2", len(impl.lastReq.Messages))
	}
	if impl.lastReq.Prompt != "hello world" {
		t.Errorf("flattened prompt=%q want hello world", impl.lastReq.Prompt)
	}
	if impl.lastReq.ModelParams.Model != "m1" {
		t.Errorf("model param=%q want m1", impl.lastReq.ModelParams.Model)
	}
}

// TestLLMProviderBridge_VisionGating proves vision is gated on
// capability and that, when supported, the image is base64-attached.
func TestLLMProviderBridge_VisionGating(t *testing.T) {
	// Provider without vision: bridge must refuse.
	noVision, _ := NewLLMProviderBridge(&echoLLMProvider{vision: false})
	if noVision.SupportsVision() {
		t.Fatal("SupportsVision should be false")
	}
	if _, err := noVision.Vision(context.Background(), []byte("img"), "describe"); err == nil {
		t.Fatal("Vision should error when provider lacks vision support")
	}

	// Provider with vision: bridge must encode and pass the image.
	impl := &echoLLMProvider{vision: true}
	withVision, _ := NewLLMProviderBridge(impl, WithBridgeModel("v1"))
	if !withVision.SupportsVision() {
		t.Fatal("SupportsVision should be true")
	}
	resp, err := withVision.Vision(context.Background(), []byte{1, 2, 3, 4}, "what is this")
	if err != nil {
		t.Fatalf("Vision: %v", err)
	}
	if resp.Content != "echo:what is this" {
		t.Errorf("Content=%q", resp.Content)
	}
	ps := impl.lastReq.ModelParams.ProviderSpecific
	enc, ok := ps["image_base64"].(string)
	if !ok {
		t.Fatal("image_base64 not attached")
	}
	decoded, _ := base64.StdEncoding.DecodeString(enc)
	if string(decoded) != string([]byte{1, 2, 3, 4}) {
		t.Errorf("decoded image mismatch: %v", decoded)
	}
}

// TestLLMProviderBridge_ErrorPropagation proves a provider failure is
// surfaced (not swallowed) — a §11.4.1 FAIL-bluff guard.
func TestLLMProviderBridge_ErrorPropagation(t *testing.T) {
	impl := &echoLLMProvider{failErr: errors.New("upstream down")}
	b, _ := NewLLMProviderBridge(impl)
	_, err := b.Chat(context.Background(), []Message{{Role: RoleUser, Content: "x"}})
	if err == nil || !strings.Contains(err.Error(), "upstream down") {
		t.Fatalf("expected upstream error, got %v", err)
	}
	if hc := b.HealthCheck(); hc == nil {
		t.Fatal("HealthCheck should surface the upstream error")
	}
}

// TestLLMProviderBridge_NilImpl proves construction rejects a nil
// provider.
func TestLLMProviderBridge_NilImpl(t *testing.T) {
	if _, err := NewLLMProviderBridge(nil); err == nil {
		t.Fatal("NewLLMProviderBridge(nil) should error")
	}
}

// TestLLMProviderBridge_SatisfiesProvider proves the bridge is usable
// anywhere an llm.Provider is expected.
func TestLLMProviderBridge_SatisfiesProvider(t *testing.T) {
	var p Provider
	b, _ := NewLLMProviderBridge(&echoLLMProvider{})
	p = b
	if p.Name() == "" {
		t.Fatal("provider name empty")
	}
}
