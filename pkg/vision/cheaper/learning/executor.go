// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package learning provides a 5-layer vision execution pipeline that reduces
// live provider calls by checking progressively richer caches and memory
// stores before falling back to the underlying ResilientExecutor.
package learning

import (
	"context"
	"image"
	"strings"
	"time"

	"digital.vasic.helixqa/pkg/vision/cheaper"
	"digital.vasic.helixqa/pkg/vision/cheaper/cache"
	"digital.vasic.helixqa/pkg/vision/cheaper/memory"
)

// LearningConfig controls which pipeline layers are active.
type LearningConfig struct {
	// EnableExactCache enables L1: exact (image+prompt) hash cache lookup.
	EnableExactCache bool

	// EnableDifferential enables L2: differential (visually similar frame) cache.
	EnableDifferential bool

	// EnableVectorMemory enables L3: semantic prompt similarity search in vector
	// memory.
	EnableVectorMemory bool

	// EnableFewShot enables L4: few-shot prompt augmentation from past
	// successful interactions.
	EnableFewShot bool

	// EnableOptimization enables recording results in the provider optimizer so
	// that the best provider is selected over time.
	EnableOptimization bool

	// SimilarityThreshold is the minimum cosine-similarity score required for an
	// L3 vector-memory hit to be accepted. Defaults to 0.85 when zero.
	SimilarityThreshold float64
}

// LearningVisionExecutor wraps a ResilientExecutor with a 5-layer caching and
// learning pipeline. Each layer is independently enabled via LearningConfig.
// All methods are safe for concurrent use.
type LearningVisionExecutor struct {
	executor          *cheaper.ResilientExecutor
	vectorMemory      *memory.VectorMemoryStore
	diffCache         *cache.DifferentialCache
	fewShotBuilder    *FewShotBuilder
	providerOptimizer *ProviderOptimizer
	exactCache        *cache.ExactCache
	config            LearningConfig
}

// NewLearningVisionExecutor creates a LearningVisionExecutor that wraps
// executor and memoryStore. All cache/optimizer sub-components are
// constructed internally. If config.SimilarityThreshold is zero it is set to
// the recommended default of 0.85.
func NewLearningVisionExecutor(
	executor *cheaper.ResilientExecutor,
	memoryStore *memory.VectorMemoryStore,
	config LearningConfig,
) *LearningVisionExecutor {
	if config.SimilarityThreshold == 0 {
		config.SimilarityThreshold = 0.85
	}

	return &LearningVisionExecutor{
		executor:          executor,
		vectorMemory:      memoryStore,
		diffCache:         cache.NewDifferentialCache(0.1),
		fewShotBuilder:    NewFewShotBuilder(memoryStore, 3),
		providerOptimizer: NewProviderOptimizer(),
		exactCache:        cache.NewExactCache(256),
		config:            config,
	}
}

// Execute runs the 5-layer pipeline for the given image and prompt.
//
//   - L1 (ExactCache):      return cached result on identical image+prompt.
//   - L2 (Differential):    return cached result for a visually similar frame.
//   - L3 (VectorMemory):    return a past result with high prompt similarity.
//   - L4 (FewShot):         augment the prompt with successful examples.
//   - L5 (Executor):        call the underlying ResilientExecutor.
//
// After a live L5 call the result is stored in all enabled cache/memory layers
// asynchronously so as not to add latency to the caller.
func (l *LearningVisionExecutor) Execute(
	ctx context.Context,
	img image.Image,
	prompt string,
) (*cheaper.VisionResult, error) {
	// ── L1: Exact cache ──────────────────────────────────────────────────────
	if l.config.EnableExactCache {
		imageHash := memory.ComputeImageHash(img)
		if cached, ok := l.exactCache.Get(img, prompt); ok {
			return &cheaper.VisionResult{
				Text:      cached.Text,
				Model:     cached.Model,
				Provider:  "exact-cache",
				Duration:  cached.Duration,
				Timestamp: cached.Timestamp,
				CacheHit:  true,
			}, nil
		}
		// Store the image hash for use in learnFromResult.
		_ = imageHash
	}

	// ── L2: Differential cache ───────────────────────────────────────────────
	if l.config.EnableDifferential {
		if cached, ok := l.diffCache.GetCachedResponse(ctx, img); ok {
			return &cheaper.VisionResult{
				Text:      cached.Text,
				Model:     cached.Model,
				Provider:  "diff-cache",
				Duration:  cached.Duration,
				Timestamp: cached.Timestamp,
				CacheHit:  true,
			}, nil
		}
	}

	// ── L3: Vector memory ─────────────────────────────────────────────────────
	if l.config.EnableVectorMemory && l.vectorMemory != nil {
		results, err := l.vectorMemory.Search(ctx, prompt, 1)
		if err == nil && len(results) > 0 {
			top := results[0]
			if top.SimilarityScore > l.config.SimilarityThreshold {
				return &cheaper.VisionResult{
					Text:       top.Response,
					Model:      top.ProviderModel,
					Provider:   "vector-memory",
					Duration:   top.Latency,
					Timestamp:  top.Timestamp,
					CacheHit:   true,
					Confidence: top.ConfidenceScore,
				}, nil
			}
		}
	}

	// ── L4: Few-shot prompt augmentation ──────────────────────────────────────
	activePrompt := prompt
	if l.config.EnableFewShot && l.fewShotBuilder != nil {
		augmented, err := l.fewShotBuilder.BuildPrompt(ctx, prompt)
		if err == nil {
			activePrompt = augmented
		}
	}

	// ── L5: Live executor call ────────────────────────────────────────────────
	result, err := l.executor.Execute(ctx, img, activePrompt)
	if err != nil {
		if l.config.EnableOptimization && result != nil {
			uiType := detectUIType(prompt)
			l.providerOptimizer.RecordFailure(result.Provider, uiType)
		}
		return nil, err
	}

	// Asynchronously persist the result in all enabled layers.
	go l.learnFromResult(context.Background(), img, prompt, result)

	return result, nil
}

// learnFromResult stores result in every enabled cache/memory layer and
// updates the provider optimizer. It is called asynchronously after a
// successful L5 execution so it never delays the caller.
func (l *LearningVisionExecutor) learnFromResult(
	ctx context.Context,
	img image.Image,
	prompt string,
	result *cheaper.VisionResult,
) {
	cached := &cache.CachedResponse{
		Text:      result.Text,
		Model:     result.Model,
		Duration:  result.Duration,
		Timestamp: result.Timestamp,
	}

	if l.config.EnableExactCache {
		l.exactCache.Put(img, prompt, cached)
	}

	if l.config.EnableDifferential {
		l.diffCache.StoreFrame(img, cached)
	}

	if l.config.EnableVectorMemory && l.vectorMemory != nil {
		uiType := detectUIType(prompt)
		mem := &memory.VisionMemory{
			ImageHash:       memory.ComputeImageHash(img),
			Prompt:          prompt,
			Response:        result.Text,
			ProviderModel:   result.Provider + "/" + result.Model,
			Success:         true,
			Latency:         result.Duration,
			UIElementType:   uiType,
			ConfidenceScore: result.Confidence,
			Timestamp:       time.Now(),
		}
		_ = l.vectorMemory.Store(ctx, mem)
	}

	if l.config.EnableOptimization {
		uiType := detectUIType(prompt)
		l.providerOptimizer.RecordSuccess(result.Provider, result.Duration, uiType)
	}
}

// detectUIType infers a UI element type from the prompt text by checking for
// well-known keywords. The returned value matches the types understood by
// ProviderOptimizer: "button", "text", "image", "link", "navigation", or
// "general".
func detectUIType(prompt string) string {
	lower := strings.ToLower(prompt)

	switch {
	case containsAny(lower, "button", "click", "press", "tap"):
		return "button"
	case containsAny(lower, "text", "input", "field", "type", "enter"):
		return "text"
	case containsAny(lower, "image", "picture", "photo", "icon"):
		return "image"
	case containsAny(lower, "link", "url", "href"):
		return "link"
	case containsAny(lower, "menu", "nav", "navigation"):
		return "navigation"
	default:
		return "general"
	}
}

// containsAny returns true when s contains at least one of the given
// substrings.
func containsAny(s string, substrings ...string) bool {
	for _, sub := range substrings {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
