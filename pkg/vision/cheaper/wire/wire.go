// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package wire composes the cheaper vision stack (providers, executor,
// learning pipeline) and returns an llm.Provider-compatible bridge.
package wire

import (
	"context"
	"fmt"
	"image"
	"os"
	"strconv"
	"strings"
	"time"

	"digital.vasic.helixqa/pkg/vision/cheaper"
	"digital.vasic.helixqa/pkg/vision/cheaper/learning"
	"digital.vasic.helixqa/pkg/vision/cheaper/memory"

	"github.com/philippgille/chromem-go"
)

// Config holds configuration for building the full cheaper vision stack.
type Config struct {
	// FallbackChain is the ordered list of provider names for fallback strategy.
	FallbackChain []string

	// Strategy is the execution strategy (default: cheaper.StrategyFallback).
	Strategy cheaper.ExecutionStrategy

	// Timeout per-provider (default: 30s).
	Timeout time.Duration

	// RetryAttempts per provider (default: 2).
	RetryAttempts int

	// CircuitBreaker enables per-provider circuit breakers (default: true).
	CircuitBreaker bool

	// Learning enables the 5-layer learning system.
	Learning bool

	// PersistPath for vector memory (empty = in-memory).
	PersistPath string

	// MaxCacheEntries for L1 exact cache (default: 10000).
	MaxCacheEntries int

	// ChangeThreshold for L2 differential cache (default: 0.05).
	ChangeThreshold float64

	// SimilarityThreshold for L3 vector memory (default: 0.85).
	SimilarityThreshold float64

	// EmbeddingFunc for vector memory (nil = use default).
	EmbeddingFunc chromem.EmbeddingFunc
}

// DefaultConfig returns a Config populated from HELIX_VISION_*
// environment variables with sensible defaults.
func DefaultConfig() Config {
	chain := []string{"qwen25vl", "glm4v", "uitars", "showui"}
	if v := os.Getenv("HELIX_VISION_FALLBACK_CHAIN"); v != "" {
		chain = strings.Split(v, ",")
	}

	timeout := 30 * time.Second
	if v := os.Getenv("HELIX_VISION_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			timeout = d
		}
	}

	maxCache := 10000
	if v := os.Getenv("HELIX_VISION_MAX_MEMORIES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxCache = n
		}
	}

	changeThresh := 0.05
	if v := os.Getenv("HELIX_VISION_CHANGE_THRESHOLD"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			changeThresh = f
		}
	}

	learningEnabled := true
	if v := os.Getenv("HELIX_VISION_LEARNING_ENABLED"); v == "false" {
		learningEnabled = false
	}

	return Config{
		FallbackChain:       chain,
		Strategy:            cheaper.StrategyFallback,
		Timeout:             timeout,
		RetryAttempts:       2,
		CircuitBreaker:      true,
		Learning:            learningEnabled,
		PersistPath:         os.Getenv("HELIX_VISION_PERSIST_PATH"),
		MaxCacheEntries:     maxCache,
		ChangeThreshold:     changeThresh,
		SimilarityThreshold: 0.85,
	}
}

// Build constructs the complete cheaper vision stack from the given
// providers and config. Returns a BridgeProvider satisfying llm.Provider.
//
// Usage with the autonomous pipeline:
//
//	bp, err := wire.Build(providers, cfg)
//	pipeline.WithVisionProvider(bp)
func Build(providers []cheaper.VisionProvider, cfg Config) (*cheaper.BridgeProvider, error) {
	if len(providers) == 0 {
		return nil, fmt.Errorf("wire.Build: no providers given")
	}

	execCfg := cheaper.ExecutorConfig{
		Strategy:           cfg.Strategy,
		Providers:          providers,
		Timeout:            cfg.Timeout,
		RetryAttempts:      cfg.RetryAttempts,
		RetryDelay:         500 * time.Millisecond,
		CircuitBreaker:     cfg.CircuitBreaker,
		CBFailureThreshold: 5,
		CBSuccessThreshold: 3,
		CBTimeout:          30 * time.Second,
		FallbackChain:      cfg.FallbackChain,
	}
	executor := cheaper.NewResilientExecutor(execCfg)

	if !cfg.Learning {
		direct := &executorAdapter{executor: executor}
		return cheaper.NewBridgeProvider(direct), nil
	}

	embedFunc := cfg.EmbeddingFunc
	if embedFunc == nil {
		embedFunc = defaultEmbedder()
	}
	memStore, err := memory.NewVectorMemoryStore(cfg.PersistPath, embedFunc)
	if err != nil {
		return nil, fmt.Errorf("wire.Build: vector memory: %w", err)
	}

	learnCfg := learning.LearningConfig{
		EnableExactCache:    true,
		EnableDifferential:  true,
		EnableVectorMemory:  true,
		EnableFewShot:       true,
		EnableOptimization:  true,
		SimilarityThreshold: cfg.SimilarityThreshold,
	}

	learningExec := learning.NewLearningVisionExecutor(executor, memStore, learnCfg)
	learningProv := &learningAdapter{executor: learningExec}
	return cheaper.NewBridgeProvider(learningProv), nil
}

// Enabled returns true if HELIX_VISION_CHEAPER_ENABLED is "true" or "1".
func Enabled() bool {
	v := os.Getenv("HELIX_VISION_CHEAPER_ENABLED")
	return v == "true" || v == "1"
}

// executorAdapter adapts ResilientExecutor to cheaper.VisionProvider.
type executorAdapter struct {
	executor *cheaper.ResilientExecutor
}

func (a *executorAdapter) Analyze(ctx context.Context, img image.Image, prompt string) (*cheaper.VisionResult, error) {
	return a.executor.Execute(ctx, img, prompt)
}
func (a *executorAdapter) Name() string                        { return "cheaper-executor" }
func (a *executorAdapter) HealthCheck(_ context.Context) error { return nil }
func (a *executorAdapter) GetCapabilities() cheaper.ProviderCapabilities {
	return cheaper.ProviderCapabilities{}
}
func (a *executorAdapter) GetCostEstimate(_ int, _ int) float64 { return 0 }

// learningAdapter adapts LearningVisionExecutor to cheaper.VisionProvider.
type learningAdapter struct {
	executor *learning.LearningVisionExecutor
}

func (a *learningAdapter) Analyze(ctx context.Context, img image.Image, prompt string) (*cheaper.VisionResult, error) {
	return a.executor.Execute(ctx, img, prompt)
}
func (a *learningAdapter) Name() string                        { return "cheaper-learning" }
func (a *learningAdapter) HealthCheck(_ context.Context) error { return nil }
func (a *learningAdapter) GetCapabilities() cheaper.ProviderCapabilities {
	return cheaper.ProviderCapabilities{}
}
func (a *learningAdapter) GetCostEstimate(_ int, _ int) float64 { return 0 }

// defaultEmbedder returns a simple character-hash embedder for vector memory.
// In production, replace with a real embedding model via Ollama.
func defaultEmbedder() chromem.EmbeddingFunc {
	return func(_ context.Context, text string) ([]float32, error) {
		const dim = 384
		vec := make([]float32, dim)
		for i, c := range text {
			vec[i%dim] += float32(c) / 1000.0
		}
		var norm float32
		for _, v := range vec {
			norm += v * v
		}
		if norm > 0 {
			invNorm := 1.0 / float32(len(text)+1)
			for i := range vec {
				vec[i] *= invNorm
			}
		}
		return vec, nil
	}
}
