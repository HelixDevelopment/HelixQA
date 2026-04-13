// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package learning

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/vision/cheaper/memory"
)

// mockEmbedder returns a deterministic 384-dimensional vector based on the
// input text. Each dimension is derived from the character values of the text
// so that different strings produce distinct (though not semantically
// meaningful) embeddings. The resulting vector is L2-normalized so it satisfies
// chromem-go's requirement for normalized embeddings.
func mockEmbedder(_ context.Context, text string) ([]float32, error) {
	const dims = 384
	vec := make([]float32, dims)

	for i, ch := range text {
		idx := i % dims
		vec[idx] += float32(ch)
	}

	// L2 normalize.
	var sum float64
	for _, v := range vec {
		sum += float64(v) * float64(v)
	}
	if sum > 0 {
		norm := float32(math.Sqrt(sum))
		for i := range vec {
			vec[i] /= norm
		}
	}

	return vec, nil
}

// newTestStore creates an in-memory VectorMemoryStore using mockEmbedder.
func newTestStore(t *testing.T) *memory.VectorMemoryStore {
	t.Helper()
	store, err := memory.NewVectorMemoryStore("", mockEmbedder)
	require.NoError(t, err)
	require.NotNil(t, store)
	return store
}

// sampleMemory builds a VisionMemory with sensible defaults for testing.
func sampleMemory(id, prompt, response string, success bool) *memory.VisionMemory {
	return &memory.VisionMemory{
		ID:              id,
		ImageHash:       "hash-" + id,
		Prompt:          prompt,
		Response:        response,
		ProviderModel:   "test-model",
		Success:         success,
		Latency:         100 * time.Millisecond,
		UIElementType:   "button",
		ConfidenceScore: 0.9,
		Timestamp:       time.Now(),
	}
}

// ---------------------------------------------------------------------------
// TestFewShotBuilder_NoExamples
// ---------------------------------------------------------------------------

func TestFewShotBuilder_NoExamples(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	builder := NewFewShotBuilder(store, 5)

	basePrompt := "What is shown on screen?"
	result, err := builder.BuildPrompt(ctx, basePrompt)
	require.NoError(t, err)

	// Empty store must return the original prompt unchanged.
	assert.Equal(t, basePrompt, result)
}

// ---------------------------------------------------------------------------
// TestFewShotBuilder_WithExamples
// ---------------------------------------------------------------------------

func TestFewShotBuilder_WithExamples(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	// Store successful memories. The mock embedder produces cosine similarities
	// in the 0.4–0.5 range, so we set minScore=0.0 to ensure examples are
	// included regardless of the deterministic-but-low mock scores.
	require.NoError(t, store.Store(ctx, sampleMemory("ex-1", "click the login button", "login button clicked successfully", true)))
	require.NoError(t, store.Store(ctx, sampleMemory("ex-2", "navigate to home screen", "navigated to home screen", true)))

	builder := &FewShotBuilder{
		memoryStore: store,
		maxExamples: 5,
		minScore:    0.0,
	}
	basePrompt := "click login"
	result, err := builder.BuildPrompt(ctx, basePrompt)
	require.NoError(t, err)

	// Augmented prompt must contain the few-shot header.
	assert.Contains(t, result, "Here are examples of successful UI element identification:")
	// Must contain at least one "Example" label.
	assert.Contains(t, result, "Example 1:")
	// Must contain the instruction before the original prompt.
	assert.Contains(t, result, "Now, using these examples as reference, please respond to:")
	// Must end with (or contain) the original basePrompt.
	assert.True(t, strings.HasSuffix(strings.TrimSpace(result), basePrompt),
		"augmented prompt must end with the original base prompt")
}

// ---------------------------------------------------------------------------
// TestFewShotBuilder_BelowMinScore
// ---------------------------------------------------------------------------

func TestFewShotBuilder_BelowMinScore(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	// Store memories so the store is non-empty.
	require.NoError(t, store.Store(ctx, sampleMemory("s-1", "find the settings icon", "settings icon found", true)))
	require.NoError(t, store.Store(ctx, sampleMemory("s-2", "open the menu drawer", "menu drawer opened", true)))

	// Set minScore impossibly high — all results should be filtered out.
	builder := &FewShotBuilder{
		memoryStore: store,
		maxExamples: 5,
		minScore:    0.99,
	}

	basePrompt := "open menu"
	result, err := builder.BuildPrompt(ctx, basePrompt)
	require.NoError(t, err)

	// All examples below minScore → original prompt returned unchanged.
	assert.Equal(t, basePrompt, result)
}

// ---------------------------------------------------------------------------
// TestFewShotBuilder_LearnFromSuccess
// ---------------------------------------------------------------------------

func TestFewShotBuilder_LearnFromSuccess(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	builder := NewFewShotBuilder(store, 5)

	// Store is empty initially.
	assert.Equal(t, 0, store.Size())

	prompt := "tap the play button"
	response := `{"action":"tap","element":"play_button","confidence":0.95}`
	err := builder.LearnFromSuccess(ctx, prompt, response, 0.95)
	require.NoError(t, err)

	// Store should now have exactly one entry.
	assert.Equal(t, 1, store.Size())

	// Searching for the learned prompt should return the stored memory.
	results, err := store.Search(ctx, "play button", 1)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, prompt, results[0].Prompt)
	assert.Equal(t, response, results[0].Response)
	assert.True(t, results[0].Success)
	assert.InDelta(t, 0.95, results[0].ConfidenceScore, 0.001)
}
