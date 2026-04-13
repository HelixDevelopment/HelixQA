// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package learning

import (
	"context"
	"image"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/vision/cheaper"
	"digital.vasic.helixqa/pkg/vision/cheaper/cache"
	"digital.vasic.helixqa/pkg/vision/cheaper/memory"
)

// ── Stub provider ────────────────────────────────────────────────────────────

// stubProvider is a minimal VisionProvider that always returns a fixed result.
type stubProvider struct {
	name     string
	response string
	callCount int
}

func (s *stubProvider) Name() string { return s.name }

func (s *stubProvider) Analyze(
	_ context.Context,
	_ image.Image,
	_ string,
) (*cheaper.VisionResult, error) {
	s.callCount++
	return &cheaper.VisionResult{
		Text:      s.response,
		Provider:  s.name,
		Model:     "stub-model",
		Timestamp: time.Now(),
	}, nil
}

func (s *stubProvider) HealthCheck(_ context.Context) error { return nil }

func (s *stubProvider) GetCapabilities() cheaper.ProviderCapabilities {
	return cheaper.ProviderCapabilities{}
}

func (s *stubProvider) GetCostEstimate(_, _ int) float64 { return 0 }

// ── Helpers ───────────────────────────────────────────────────────────────────

// newTestImg returns a small unique RGBA image. Passing a non-zero seed value
// fills the single pixel with a recognisable colour so different calls produce
// different image hashes.
func newTestImg(r, g, b uint8) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			img.SetRGBA(x, y, struct{ R, G, B, A uint8 }{r, g, b, 255})
		}
	}
	return img
}

// mockEmbedderLearning returns a deterministic L2-normalised 384-dim vector
// derived from the input text. Reuses the same approach as few_shot_test.go so
// the two helper functions stay in sync without conflicting.
func mockEmbedderLearning(_ context.Context, text string) ([]float32, error) {
	const dims = 384
	vec := make([]float32, dims)
	for i, ch := range text {
		vec[i%dims] += float32(ch)
	}
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

// newTestMemoryStore creates an in-memory VectorMemoryStore backed by the
// deterministic mock embedder.
func newTestMemoryStore(t *testing.T) *memory.VectorMemoryStore {
	t.Helper()
	store, err := memory.NewVectorMemoryStore("", mockEmbedderLearning)
	require.NoError(t, err)
	return store
}

// newTestExecutor wraps a single stubProvider in a ResilientExecutor using
// StrategyFallback (simplest strategy for unit tests).
func newTestExecutor(prov *stubProvider) *cheaper.ResilientExecutor {
	return cheaper.NewResilientExecutor(cheaper.ExecutorConfig{
		Strategy:  cheaper.StrategyFallback,
		Providers: []cheaper.VisionProvider{prov},
	})
}

// newAllEnabledConfig returns a LearningConfig with every layer turned on.
func newAllEnabledConfig() LearningConfig {
	return LearningConfig{
		EnableExactCache:    true,
		EnableDifferential:  true,
		EnableVectorMemory:  true,
		EnableFewShot:       true,
		EnableOptimization:  true,
		SimilarityThreshold: 0.85,
	}
}

// waitForLearn gives the background goroutine launched by Execute time to
// write to the cache. It polls exactCache.Size() up to maxWait.
func waitForLearn(t *testing.T, ec *cache.ExactCache, wantSize int, maxWait time.Duration) {
	t.Helper()
	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		if ec.Size() >= wantSize {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// ── Tests ─────────────────────────────────────────────────────────────────────

// TestLearningExecutor_L1_ExactCacheHit verifies that a pre-populated exact
// cache entry is returned immediately with CacheHit=true and
// Provider="exact-cache", without calling the underlying executor.
func TestLearningExecutor_L1_ExactCacheHit(t *testing.T) {
	prov := &stubProvider{name: "stub", response: "live-result"}
	exec := newTestExecutor(prov)
	store := newTestMemoryStore(t)
	cfg := newAllEnabledConfig()

	lve := NewLearningVisionExecutor(exec, store, cfg)

	img := newTestImg(10, 20, 30)
	prompt := "tap the submit button"

	// Pre-populate the exact cache.
	lve.exactCache.Put(img, prompt, &cache.CachedResponse{
		Text:      "cached-result",
		Model:     "cached-model",
		Duration:  5 * time.Millisecond,
		Timestamp: time.Now(),
	})

	result, err := lve.Execute(context.Background(), img, prompt)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.True(t, result.CacheHit, "expected CacheHit=true for L1 hit")
	assert.Equal(t, "exact-cache", result.Provider)
	assert.Equal(t, "cached-result", result.Text)
	assert.Equal(t, 0, prov.callCount, "underlying executor must not be called on L1 hit")
}

// TestLearningExecutor_L2_DiffCacheHit verifies that when the differential
// cache has a stored frame that is identical to the query image, the response
// is returned with Provider="diff-cache" without calling the executor.
func TestLearningExecutor_L2_DiffCacheHit(t *testing.T) {
	prov := &stubProvider{name: "stub", response: "live-result"}
	exec := newTestExecutor(prov)
	store := newTestMemoryStore(t)

	// Disable L1 so we exercise L2.
	cfg := newAllEnabledConfig()
	cfg.EnableExactCache = false

	lve := NewLearningVisionExecutor(exec, store, cfg)

	img := newTestImg(50, 60, 70)
	prompt := "navigate to home screen"

	// Pre-populate the differential cache with the same image.
	lve.diffCache.StoreFrame(img, &cache.CachedResponse{
		Text:      "diff-cached-result",
		Model:     "diff-model",
		Duration:  3 * time.Millisecond,
		Timestamp: time.Now(),
	})

	result, err := lve.Execute(context.Background(), img, prompt)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.True(t, result.CacheHit, "expected CacheHit=true for L2 hit")
	assert.Equal(t, "diff-cache", result.Provider)
	assert.Equal(t, "diff-cached-result", result.Text)
	assert.Equal(t, 0, prov.callCount, "underlying executor must not be called on L2 hit")
}

// TestLearningExecutor_L5_FallsThrough verifies that with empty caches the
// pipeline falls through all layers and the real executor is called.
func TestLearningExecutor_L5_FallsThrough(t *testing.T) {
	prov := &stubProvider{name: "stub", response: "live-result"}
	exec := newTestExecutor(prov)
	store := newTestMemoryStore(t)
	cfg := newAllEnabledConfig()

	lve := NewLearningVisionExecutor(exec, store, cfg)

	img := newTestImg(100, 110, 120)
	prompt := "describe what is on screen"

	result, err := lve.Execute(context.Background(), img, prompt)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "live-result", result.Text)
	assert.Equal(t, "stub", result.Provider)
	assert.False(t, result.CacheHit)
	assert.Equal(t, 1, prov.callCount, "executor must be called exactly once on full fall-through")
}

// TestLearningExecutor_DisabledLayers verifies that when all Enable* flags are
// false the pipeline skips every caching layer and calls the executor directly.
func TestLearningExecutor_DisabledLayers(t *testing.T) {
	prov := &stubProvider{name: "stub", response: "direct-result"}
	exec := newTestExecutor(prov)
	store := newTestMemoryStore(t)

	cfg := LearningConfig{
		EnableExactCache:    false,
		EnableDifferential:  false,
		EnableVectorMemory:  false,
		EnableFewShot:       false,
		EnableOptimization:  false,
		SimilarityThreshold: 0.85,
	}

	lve := NewLearningVisionExecutor(exec, store, cfg)

	img := newTestImg(200, 210, 220)
	prompt := "is there a login button visible?"

	// Pre-populate caches — they must be ignored because layers are disabled.
	lve.exactCache.Put(img, prompt, &cache.CachedResponse{Text: "should-not-return"})
	lve.diffCache.StoreFrame(img, &cache.CachedResponse{Text: "should-not-return"})

	result, err := lve.Execute(context.Background(), img, prompt)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "direct-result", result.Text)
	assert.False(t, result.CacheHit)
	assert.Equal(t, 1, prov.callCount)
}

// TestLearningExecutor_PostExecution_Learns verifies that after a successful
// L5 call the exact cache is populated with the result, so a subsequent
// identical call is served from the cache.
func TestLearningExecutor_PostExecution_Learns(t *testing.T) {
	prov := &stubProvider{name: "stub", response: "learned-result"}
	exec := newTestExecutor(prov)
	store := newTestMemoryStore(t)
	cfg := newAllEnabledConfig()

	lve := NewLearningVisionExecutor(exec, store, cfg)

	img := newTestImg(30, 40, 50)
	prompt := "click the search icon"

	// First call — should hit the executor.
	result, err := lve.Execute(context.Background(), img, prompt)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, prov.callCount)

	// Wait for the background goroutine to write the exact-cache entry.
	waitForLearn(t, lve.exactCache, 1, 500*time.Millisecond)

	assert.Equal(t, 1, lve.exactCache.Size(), "exact cache must have one entry after learning")

	// Second call with the same image+prompt must be served from L1.
	result2, err := lve.Execute(context.Background(), img, prompt)
	require.NoError(t, err)
	require.NotNil(t, result2)

	assert.True(t, result2.CacheHit, "second call should be a cache hit")
	assert.Equal(t, "exact-cache", result2.Provider)
	// Executor should still have been called only once.
	assert.Equal(t, 1, prov.callCount)
}

// TestDetectUIType exercises detectUIType with a table of representative
// prompts.
func TestDetectUIType(t *testing.T) {
	cases := []struct {
		prompt string
		want   string
	}{
		// button keywords
		{"tap the submit button", "button"},
		{"click the OK button", "button"},
		{"press the back key", "button"},
		{"tap here to continue", "button"},
		// text keywords
		{"enter your username in the text field", "text"},
		{"type the search query into the input", "text"},
		{"fill in the password field", "text"},
		// image keywords
		{"describe the image on screen", "image"},
		{"what does the icon look like?", "image"},
		{"is there a picture in the header?", "image"},
		// link keywords
		{"follow the link to the homepage", "link"},
		{"what is the url shown?", "link"},
		{"open the href attribute", "link"},
		// navigation keywords
		{"navigate to the menu", "navigation"},
		{"open the nav drawer", "navigation"},
		{"go to navigation settings", "navigation"},
		// general fallback
		{"describe what is visible on screen", "general"},
		{"is there any content loaded?", "general"},
		{"what color is the background?", "general"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.prompt, func(t *testing.T) {
			got := detectUIType(tc.prompt)
			assert.Equal(t, tc.want, got, "prompt: %q", tc.prompt)
		})
	}
}

// TestLearningExecutor_DefaultSimilarityThreshold verifies that when
// SimilarityThreshold is left at zero, NewLearningVisionExecutor sets it to
// 0.85.
func TestLearningExecutor_DefaultSimilarityThreshold(t *testing.T) {
	prov := &stubProvider{name: "stub", response: "r"}
	exec := newTestExecutor(prov)
	store := newTestMemoryStore(t)

	lve := NewLearningVisionExecutor(exec, store, LearningConfig{})

	assert.Equal(t, 0.85, lve.config.SimilarityThreshold)
}
