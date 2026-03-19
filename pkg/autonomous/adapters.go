// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package autonomous

import (
	"context"
	"encoding/json"
	"fmt"

	"digital.vasic.docprocessor/pkg/llm"
	"digital.vasic.llmorchestrator/pkg/agent"
	"digital.vasic.llmorchestrator/pkg/parser"
)

// agentLLMAdapter bridges LLMOrchestrator's Agent interface to
// DocProcessor's LLMAgent interface. This allows DocProcessor
// to use LLM agents without a direct module dependency on
// LLMOrchestrator.
type agentLLMAdapter struct {
	agent  agent.Agent
	parser parser.ResponseParser
}

// NewAgentLLMAdapter creates an adapter that bridges an
// LLMOrchestrator Agent to the DocProcessor LLMAgent interface.
func NewAgentLLMAdapter(
	ag agent.Agent,
	p parser.ResponseParser,
) llm.LLMAgent {
	return &agentLLMAdapter{
		agent:  ag,
		parser: p,
	}
}

// Summarize sends a summarization prompt to the agent.
func (a *agentLLMAdapter) Summarize(
	ctx context.Context, text string,
) (string, error) {
	prompt := fmt.Sprintf(
		"Summarize the following document concisely:\n\n%s", text,
	)
	resp, err := a.agent.Send(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("summarize: %w", err)
	}
	return resp.Content, nil
}

// ExtractFeatures sends a feature extraction prompt and parses
// the structured JSON response.
func (a *agentLLMAdapter) ExtractFeatures(
	ctx context.Context, text string,
) ([]llm.RawFeature, error) {
	prompt := fmt.Sprintf(
		"Extract all software features from this documentation. "+
			"Return a JSON array of objects with fields: "+
			"name, description, category, platforms, priority.\n\n%s",
		text,
	)
	resp, err := a.agent.Send(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("extract features: %w", err)
	}

	jsonData, err := a.parser.ExtractJSON(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("parse features: %w", err)
	}

	// Marshal back to JSON and unmarshal into RawFeature slice.
	featuresData, ok := jsonData["features"]
	if !ok {
		// Try the response as a direct array.
		return nil, fmt.Errorf("no features key in response")
	}

	raw, err := json.Marshal(featuresData)
	if err != nil {
		return nil, fmt.Errorf("marshal features: %w", err)
	}

	var features []llm.RawFeature
	if err := json.Unmarshal(raw, &features); err != nil {
		return nil, fmt.Errorf("unmarshal features: %w", err)
	}
	return features, nil
}

// ClassifyFeature sends a classification prompt.
func (a *agentLLMAdapter) ClassifyFeature(
	ctx context.Context, feature llm.RawFeature,
) (llm.FeatureCategory, error) {
	prompt := fmt.Sprintf(
		"Classify this feature into one category "+
			"(format, ui, network, settings, storage, auth, "+
			"editor, other):\n\nName: %s\nDescription: %s",
		feature.Name, feature.Description,
	)
	resp, err := a.agent.Send(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("classify feature: %w", err)
	}
	return llm.FeatureCategory(resp.Content), nil
}

// InferScreens sends a screen inference prompt.
func (a *agentLLMAdapter) InferScreens(
	ctx context.Context, features []llm.Feature,
) ([]llm.ExpectedScreen, error) {
	prompt := fmt.Sprintf(
		"Given %d features, infer the expected application "+
			"screens. Return a JSON array of objects with fields: "+
			"id, name, description, features, platforms.",
		len(features),
	)
	resp, err := a.agent.Send(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("infer screens: %w", err)
	}

	jsonData, err := a.parser.ExtractJSON(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("parse screens: %w", err)
	}

	screensData, ok := jsonData["screens"]
	if !ok {
		return nil, fmt.Errorf("no screens key in response")
	}

	raw, err := json.Marshal(screensData)
	if err != nil {
		return nil, fmt.Errorf("marshal screens: %w", err)
	}

	var screens []llm.ExpectedScreen
	if err := json.Unmarshal(raw, &screens); err != nil {
		return nil, fmt.Errorf("unmarshal screens: %w", err)
	}
	return screens, nil
}

// GenerateTestSteps generates verification steps for a feature.
func (a *agentLLMAdapter) GenerateTestSteps(
	ctx context.Context, feature llm.Feature,
) ([]llm.TestStep, error) {
	prompt := fmt.Sprintf(
		"Generate test steps to verify this feature:\n\n"+
			"Name: %s\nDescription: %s\nPlatforms: %v",
		feature.Name, feature.Description, feature.Platforms,
	)
	resp, err := a.agent.Send(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("generate steps: %w", err)
	}

	jsonData, err := a.parser.ExtractJSON(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("parse steps: %w", err)
	}

	stepsData, ok := jsonData["steps"]
	if !ok {
		return nil, fmt.Errorf("no steps key in response")
	}

	raw, err := json.Marshal(stepsData)
	if err != nil {
		return nil, fmt.Errorf("marshal steps: %w", err)
	}

	var steps []llm.TestStep
	if err := json.Unmarshal(raw, &steps); err != nil {
		return nil, fmt.Errorf("unmarshal steps: %w", err)
	}
	return steps, nil
}
