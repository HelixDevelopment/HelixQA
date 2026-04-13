// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package learning provides few-shot prompt augmentation for the HelixQA
// vision pipeline. It retrieves semantically similar past interactions from
// the vector memory store and prepends them to the current prompt so that
// the vision model has high-quality examples to reason from.
package learning

import (
	"context"
	"fmt"
	"strings"
	"time"

	"digital.vasic.helixqa/pkg/vision/cheaper/memory"
)

// FewShotBuilder augments a base prompt with successful examples retrieved
// from a VectorMemoryStore. Only examples whose similarity score meets or
// exceeds minScore are included.
type FewShotBuilder struct {
	memoryStore *memory.VectorMemoryStore
	maxExamples int
	minScore    float64
}

// NewFewShotBuilder creates a FewShotBuilder backed by store. Up to
// maxExamples will be prepended to any prompt. The minimum cosine-similarity
// threshold is fixed at 0.7; use the struct literal directly if a different
// threshold is required.
func NewFewShotBuilder(store *memory.VectorMemoryStore, maxExamples int) *FewShotBuilder {
	return &FewShotBuilder{
		memoryStore: store,
		maxExamples: maxExamples,
		minScore:    0.7,
	}
}

// BuildPrompt retrieves few-shot examples from the memory store that are
// semantically similar to basePrompt, filters them by minScore, and formats
// them as an augmented prompt. If no examples pass the threshold, basePrompt
// is returned unchanged.
func (b *FewShotBuilder) BuildPrompt(ctx context.Context, basePrompt string) (string, error) {
	examples, err := b.memoryStore.GetFewShotExamples(ctx, basePrompt, b.maxExamples)
	if err != nil {
		return "", fmt.Errorf("learning: get few-shot examples: %w", err)
	}

	// Filter by minimum similarity score.
	filtered := examples[:0]
	for _, ex := range examples {
		if ex.SimilarityScore >= b.minScore {
			filtered = append(filtered, ex)
		}
	}

	if len(filtered) == 0 {
		return basePrompt, nil
	}

	var sb strings.Builder
	sb.WriteString("Here are examples of successful UI element identification:\n")

	for i, ex := range filtered {
		fmt.Fprintf(&sb, "\nExample %d:\nQuery: %s\nCorrect Response: %s\n", i+1, ex.Prompt, ex.Response)
	}

	sb.WriteString("\nNow, using these examples as reference, please respond to:\n")
	sb.WriteString(basePrompt)

	return sb.String(), nil
}

// LearnFromSuccess stores a successful vision interaction in the memory store
// so that it can be retrieved as a few-shot example in future prompts.
func (b *FewShotBuilder) LearnFromSuccess(ctx context.Context, prompt, response string, confidence float64) error {
	mem := &memory.VisionMemory{
		Prompt:          prompt,
		Response:        response,
		Success:         true,
		ConfidenceScore: confidence,
		Timestamp:       time.Now(),
	}

	if err := b.memoryStore.Store(ctx, mem); err != nil {
		return fmt.Errorf("learning: store success memory: %w", err)
	}

	return nil
}
