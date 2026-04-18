// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package autonomous

import (
	"context"
	"fmt"
	"testing"
	"time"

	"digital.vasic.docprocessor/pkg/llm"
	"digital.vasic.llmorchestrator/pkg/agent"
	"digital.vasic.llmorchestrator/pkg/parser"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// adapterTestAgent implements agent.Agent for adapter tests.
type adapterTestAgent struct {
	responses []agent.Response
	callIdx   int
	failErr   error
}

func (a *adapterTestAgent) ID() string                    { return "adapter-test" }
func (a *adapterTestAgent) Name() string                  { return "test" }
func (a *adapterTestAgent) Start(_ context.Context) error { return nil }
func (a *adapterTestAgent) Stop(_ context.Context) error  { return nil }
func (a *adapterTestAgent) IsRunning() bool               { return true }

func (a *adapterTestAgent) Health(_ context.Context) agent.HealthStatus {
	return agent.HealthStatus{Healthy: true, CheckedAt: time.Now()}
}

func (a *adapterTestAgent) Send(_ context.Context, _ string) (agent.Response, error) {
	if a.failErr != nil {
		return agent.Response{}, a.failErr
	}
	if a.callIdx < len(a.responses) {
		resp := a.responses[a.callIdx]
		a.callIdx++
		return resp, nil
	}
	return agent.Response{Content: "ok"}, nil
}

func (a *adapterTestAgent) SendStream(_ context.Context, _ string) (<-chan agent.StreamChunk, error) {
	ch := make(chan agent.StreamChunk, 1)
	ch <- agent.StreamChunk{Done: true}
	close(ch)
	return ch, nil
}

func (a *adapterTestAgent) SendWithAttachments(_ context.Context, _ string, _ []agent.Attachment) (agent.Response, error) {
	return agent.Response{}, nil
}

func (a *adapterTestAgent) OutputDir() string { return "/tmp" }

func (a *adapterTestAgent) Capabilities() agent.AgentCapabilities {
	return agent.AgentCapabilities{Vision: true}
}

func (a *adapterTestAgent) SupportsVision() bool { return true }

func (a *adapterTestAgent) ModelInfo() agent.ModelInfo {
	return agent.ModelInfo{ID: "test"}
}

func TestNewAgentLLMAdapter(t *testing.T) {
	ag := &adapterTestAgent{}
	p := parser.NewParser()
	adapter := NewAgentLLMAdapter(ag, p)
	assert.NotNil(t, adapter)
}

func TestAgentLLMAdapter_Summarize(t *testing.T) {
	ag := &adapterTestAgent{
		responses: []agent.Response{
			{Content: "This is a summary of the document."},
		},
	}
	adapter := NewAgentLLMAdapter(ag, parser.NewParser())

	result, err := adapter.Summarize(context.Background(), "long document text")
	require.NoError(t, err)
	assert.Equal(t, "This is a summary of the document.", result)
}

func TestAgentLLMAdapter_Summarize_Error(t *testing.T) {
	ag := &adapterTestAgent{failErr: fmt.Errorf("timeout")}
	adapter := NewAgentLLMAdapter(ag, parser.NewParser())

	_, err := adapter.Summarize(context.Background(), "text")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "summarize")
}

func TestAgentLLMAdapter_ExtractFeatures_Error(t *testing.T) {
	ag := &adapterTestAgent{failErr: fmt.Errorf("agent down")}
	adapter := NewAgentLLMAdapter(ag, parser.NewParser())

	_, err := adapter.ExtractFeatures(context.Background(), "text")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "extract features")
}

func TestAgentLLMAdapter_ClassifyFeature_Error(t *testing.T) {
	ag := &adapterTestAgent{failErr: fmt.Errorf("timeout")}
	adapter := NewAgentLLMAdapter(ag, parser.NewParser())

	_, err := adapter.ClassifyFeature(context.Background(), llm.RawFeature{Name: "test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "classify feature")
}

func TestAgentLLMAdapter_InferScreens_Error(t *testing.T) {
	ag := &adapterTestAgent{failErr: fmt.Errorf("timeout")}
	adapter := NewAgentLLMAdapter(ag, parser.NewParser())

	_, err := adapter.InferScreens(context.Background(), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "infer screens")
}

func TestAgentLLMAdapter_GenerateTestSteps_Error(t *testing.T) {
	ag := &adapterTestAgent{failErr: fmt.Errorf("timeout")}
	adapter := NewAgentLLMAdapter(ag, parser.NewParser())

	_, err := adapter.GenerateTestSteps(context.Background(), llm.Feature{Name: "test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "generate steps")
}
