// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package memory

import (
	"context"
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockEmbedder returns a deterministic 384-dimensional vector based on the
// input text.  Each dimension is derived from the character values of the
// text so that different strings produce distinct (though not semantically
// meaningful) embeddings.  The resulting vector is L2-normalized so it
// satisfies chromem-go's requirement for normalized embeddings.
func mockEmbedder(_ context.Context, text string) ([]float32, error) {
	const dims = 384
	vec := make([]float32, dims)

	for i, ch := range text {
		idx := i % dims
		vec[idx] += float32(ch)
	}

	// L2 normalize
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
func newTestStore(t *testing.T) *VectorMemoryStore {
	t.Helper()
	store, err := NewVectorMemoryStore("", mockEmbedder)
	require.NoError(t, err)
	require.NotNil(t, store)
	return store
}

// sampleMemory builds a VisionMemory with sensible defaults for testing.
func sampleMemory(id, prompt, response string, success bool) *VisionMemory {
	return &VisionMemory{
		ID:              id,
		ImageHash:       fmt.Sprintf("hash-%s", id),
		Prompt:          prompt,
		Response:        response,
		ProviderModel:   "test-model",
		Success:         success,
		Latency:         100 * time.Millisecond,
		UIElementType:   "button",
		ConfidenceScore: 0.9,
		Metadata:        map[string]interface{}{"key": "value"},
		Timestamp:       time.Now(),
	}
}

// ---------------------------------------------------------------------------
// TestVectorStore_StoreAndSearch
// ---------------------------------------------------------------------------

func TestVectorStore_StoreAndSearch(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	mem1 := sampleMemory("id-1", "click the login button", "login button clicked", true)
	mem2 := sampleMemory("id-2", "navigate to home screen", "navigated to home", true)

	require.NoError(t, store.Store(ctx, mem1))
	require.NoError(t, store.Store(ctx, mem2))

	results, err := store.Search(ctx, "click login", 1)
	require.NoError(t, err)
	require.Len(t, results, 1)

	// Exactly one result must be returned, and its ID must be one of the stored ones.
	assert.Contains(t, []string{"id-1", "id-2"}, results[0].ID)
	// SimilarityScore must be a valid cosine similarity in [-1, 1].
	assert.GreaterOrEqual(t, results[0].SimilarityScore, float64(-1.0))
	assert.LessOrEqual(t, results[0].SimilarityScore, float64(1.0))
	// Response field must have been round-tripped through metadata.
	assert.NotEmpty(t, results[0].Response)
}

// ---------------------------------------------------------------------------
// TestVectorStore_GetFewShotExamples_OnlySuccess
// ---------------------------------------------------------------------------

func TestVectorStore_GetFewShotExamples_OnlySuccess(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	successful := sampleMemory("ok-1", "tap search icon", "search opened", true)
	failed := sampleMemory("fail-1", "tap broken button", "nothing happened", false)

	require.NoError(t, store.Store(ctx, successful))
	require.NoError(t, store.Store(ctx, failed))

	examples, err := store.GetFewShotExamples(ctx, "search icon", 5)
	require.NoError(t, err)

	// Only the successful memory should be returned.
	for _, ex := range examples {
		assert.True(t, ex.Success, "GetFewShotExamples must only return successful memories")
	}
}

// ---------------------------------------------------------------------------
// TestVectorStore_EmptySearch
// ---------------------------------------------------------------------------

func TestVectorStore_EmptySearch(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	// Search on an empty store must return an empty slice without error.
	results, err := store.Search(ctx, "anything", 5)
	require.NoError(t, err)
	assert.Empty(t, results)
}

// ---------------------------------------------------------------------------
// TestVectorStore_Size
// ---------------------------------------------------------------------------

func TestVectorStore_Size(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	assert.Equal(t, 0, store.Size())

	require.NoError(t, store.Store(ctx, sampleMemory("a", "alpha prompt", "alpha response", true)))
	assert.Equal(t, 1, store.Size())

	require.NoError(t, store.Store(ctx, sampleMemory("b", "beta prompt", "beta response", false)))
	assert.Equal(t, 2, store.Size())
}

// ---------------------------------------------------------------------------
// TestVectorStore_Clear
// ---------------------------------------------------------------------------

func TestVectorStore_Clear(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	require.NoError(t, store.Store(ctx, sampleMemory("c1", "clear test one", "result one", true)))
	require.NoError(t, store.Store(ctx, sampleMemory("c2", "clear test two", "result two", true)))
	require.Equal(t, 2, store.Size())

	require.NoError(t, store.Clear(ctx))
	assert.Equal(t, 0, store.Size())

	// Store should still be usable after Clear.
	require.NoError(t, store.Store(ctx, sampleMemory("c3", "post clear", "post clear response", true)))
	assert.Equal(t, 1, store.Size())
}

// ---------------------------------------------------------------------------
// TestVectorStore_ConcurrentAccess
// ---------------------------------------------------------------------------

func TestVectorStore_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	const workers = 8
	const perWorker = 5

	var wg sync.WaitGroup
	errCh := make(chan error, workers*perWorker)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				id := fmt.Sprintf("w%d-i%d", workerID, i)
				prompt := fmt.Sprintf("worker %d item %d prompt text", workerID, i)
				response := fmt.Sprintf("worker %d item %d response text", workerID, i)
				mem := sampleMemory(id, prompt, response, i%2 == 0)
				if err := store.Store(ctx, mem); err != nil {
					errCh <- fmt.Errorf("Store(%s): %w", id, err)
				}
			}
		}(w)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent store error: %v", err)
	}

	total := workers * perWorker
	assert.Equal(t, total, store.Size())

	// Concurrent reads must not panic or deadlock.
	var readWg sync.WaitGroup
	for r := 0; r < workers; r++ {
		readWg.Add(1)
		go func() {
			defer readWg.Done()
			_, _ = store.Search(ctx, "worker prompt text", 3)
		}()
	}
	readWg.Wait()
}
