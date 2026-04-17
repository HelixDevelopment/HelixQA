package ai

import (
	"context"
	"errors"
	"fmt"

	"digital.vasic.llmorchestrator/pkg/agent"
)

// OrchestratorClient adapts an LLMOrchestrator agent.Agent to the
// Nexus ai.LLMClient contract. Operators construct one from any
// already-running agent (ClaudeCodeAgent, GeminiAgent, OpenCodeAgent,
// JunieAgent, QwenCodeAgent) so Nexus AI features reuse the shared
// orchestrator rather than opening direct vendor SDK connections.
//
// The adapter does not start or stop the underlying agent — its
// lifecycle is owned by the orchestrator runtime. Call Send through
// the adapter in any goroutine; the agent.Agent implementations are
// responsible for their own concurrency safety.
type OrchestratorClient struct {
	agent        agent.Agent
	defaultModel string
}

// NewOrchestratorClient wraps a running agent.Agent for Nexus use.
func NewOrchestratorClient(ag agent.Agent, defaultModel string) (*OrchestratorClient, error) {
	if ag == nil {
		return nil, errors.New("orchestrator client: nil agent")
	}
	return &OrchestratorClient{agent: ag, defaultModel: defaultModel}, nil
}

// Chat satisfies LLMClient. The prompt shape is system + user so the
// adapter concatenates with a blank-line separator when both are
// provided — matching how every LLMOrchestrator adapter renders its
// prompts today.
func (c *OrchestratorClient) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	if c.agent == nil {
		return ChatResponse{}, errors.New("orchestrator client: agent is nil")
	}
	prompt := renderPrompt(req)
	var (
		resp agent.Response
		err  error
	)
	if len(req.ImageBase64) > 0 {
		attachments := make([]agent.Attachment, 0, len(req.ImageBase64))
		for i, raw := range req.ImageBase64 {
			_ = raw // agents take file attachments by path; inlined base64 is wrapped elsewhere
			attachments = append(attachments, agent.Attachment{
				Path:     fmt.Sprintf("helix-nexus-image-%d.png", i),
				MimeType: "image/png",
			})
		}
		resp, err = c.agent.SendWithAttachments(ctx, prompt, attachments)
	} else {
		resp, err = c.agent.Send(ctx, prompt)
	}
	if err != nil {
		return ChatResponse{}, fmt.Errorf("orchestrator send: %w", err)
	}
	model := req.Model
	if model == "" {
		model = c.defaultModel
	}
	return ChatResponse{
		Text:     resp.Content,
		Provider: c.agent.Name(),
		Model:    model,
	}, nil
}

func renderPrompt(req ChatRequest) string {
	if req.SystemPrompt == "" {
		return req.UserPrompt
	}
	if req.UserPrompt == "" {
		return req.SystemPrompt
	}
	return req.SystemPrompt + "\n\n" + req.UserPrompt
}
