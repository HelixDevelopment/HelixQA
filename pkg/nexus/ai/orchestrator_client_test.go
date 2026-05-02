package ai

import (
	"context"
	"errors"
	"strings"
	"testing"

	"digital.vasic.llmorchestrator/pkg/agent"
)

// stubAgent implements agent.Agent with canned responses + call tracking.
type stubAgent struct {
	name     string
	resp     agent.Response
	err      error
	sendArgs string
	attach   []agent.Attachment
}

func (s *stubAgent) ID() string                    { return "stub" }
func (s *stubAgent) Name() string                  { return s.name }
func (s *stubAgent) Start(_ context.Context) error { return nil }
func (s *stubAgent) Stop(_ context.Context) error  { return nil }
func (s *stubAgent) IsRunning() bool               { return true }
func (s *stubAgent) Send(_ context.Context, prompt string) (agent.Response, error) {
	s.sendArgs = prompt
	return s.resp, s.err
}
func (s *stubAgent) SendStream(_ context.Context, _ string) (<-chan agent.StreamChunk, error) {
	return nil, errors.New("stream not supported in stub")
}
func (s *stubAgent) SendWithAttachments(_ context.Context, prompt string, a []agent.Attachment) (agent.Response, error) {
	s.sendArgs = prompt
	s.attach = a
	return s.resp, s.err
}
func (s *stubAgent) OutputDir() string { return "/tmp" }
func (s *stubAgent) Capabilities() agent.AgentCapabilities {
	return agent.AgentCapabilities{}
}
func (s *stubAgent) ModelInfo() agent.ModelInfo { return agent.ModelInfo{} }
func (s *stubAgent) Health(_ context.Context) agent.HealthStatus {
	return agent.HealthStatus{Healthy: true}
}
func (s *stubAgent) Requirements() agent.AgentRequirements {
	return agent.AgentRequirements{}
}
func (s *stubAgent) SupportsVision() bool { return true }

func TestOrchestratorClient_ChatHappyPath(t *testing.T) {
	s := &stubAgent{name: "gemini", resp: agent.Response{Content: "hello world"}}
	c, _ := NewOrchestratorClient(s, "default-model")
	resp, err := c.Chat(context.Background(), ChatRequest{
		SystemPrompt: "Be terse.",
		UserPrompt:   "Say hi.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "hello world" {
		t.Errorf("text = %q", resp.Text)
	}
	if resp.Provider != "gemini" {
		t.Errorf("provider = %q", resp.Provider)
	}
	if resp.Model != "default-model" {
		t.Errorf("model fallback not applied: %q", resp.Model)
	}
	if !strings.HasPrefix(s.sendArgs, "Be terse.") {
		t.Errorf("system prompt not composed: %q", s.sendArgs)
	}
}

func TestOrchestratorClient_ChatPropagatesError(t *testing.T) {
	s := &stubAgent{err: errors.New("agent crashed")}
	c, _ := NewOrchestratorClient(s, "m")
	if _, err := c.Chat(context.Background(), ChatRequest{UserPrompt: "x"}); err == nil {
		t.Fatal("expected error propagation")
	}
}

func TestOrchestratorClient_ChatWithAttachments(t *testing.T) {
	s := &stubAgent{resp: agent.Response{Content: "ok"}}
	c, _ := NewOrchestratorClient(s, "m")
	_, err := c.Chat(context.Background(), ChatRequest{
		UserPrompt:  "describe",
		ImageBase64: []string{"AAAA", "BBBB"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.attach) != 2 {
		t.Errorf("attachments count = %d", len(s.attach))
	}
	if s.attach[0].MimeType != "image/png" {
		t.Errorf("mime = %q", s.attach[0].MimeType)
	}
}

func TestOrchestratorClient_UsesRequestedModel(t *testing.T) {
	s := &stubAgent{resp: agent.Response{Content: "ok"}}
	c, _ := NewOrchestratorClient(s, "fallback")
	resp, _ := c.Chat(context.Background(), ChatRequest{Model: "claude-opus-4-7", UserPrompt: "x"})
	if resp.Model != "claude-opus-4-7" {
		t.Errorf("requested model ignored, got %q", resp.Model)
	}
}

func TestRenderPrompt(t *testing.T) {
	cases := []struct {
		sys, user, want string
	}{
		{"", "u", "u"},
		{"s", "", "s"},
		{"s", "u", "s\n\nu"},
		{"", "", ""},
	}
	for _, c := range cases {
		if got := renderPrompt(ChatRequest{SystemPrompt: c.sys, UserPrompt: c.user}); got != c.want {
			t.Errorf("renderPrompt(%q,%q) = %q, want %q", c.sys, c.user, got, c.want)
		}
	}
}

func TestOrchestratorClient_NilAgentReceiverSafe(t *testing.T) {
	c := &OrchestratorClient{}
	if _, err := c.Chat(context.Background(), ChatRequest{UserPrompt: "x"}); err == nil {
		t.Fatal("nil agent receiver must error")
	}
}
