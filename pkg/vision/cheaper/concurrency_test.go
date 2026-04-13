// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package cheaper

import (
	"context"
	"fmt"
	"image"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/vision/cheaper/cache"
)

// TestConcurrency_ExecutorParallel fires 100 goroutines at a ResilientExecutor
// configured with StrategyFirstSuccess. Every goroutine must receive a valid,
// non-nil result without any data race (verified by -race).
func TestConcurrency_ExecutorParallel(t *testing.T) {
	p1 := &delayProvider{name: "cp1", delay: 5 * time.Millisecond, response: "r1"}
	p2 := &delayProvider{name: "cp2", delay: 10 * time.Millisecond, response: "r2"}

	exec := NewResilientExecutor(ExecutorConfig{
		Strategy:  StrategyFirstSuccess,
		Providers: []VisionProvider{p1, p2},
		Timeout:   2 * time.Second,
	})

	img := image.NewRGBA(image.Rect(0, 0, 4, 4))

	const goroutines = 100
	results := make([]*VisionResult, goroutines)
	errs := make([]error, goroutines)

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			res, err := exec.Execute(context.Background(), img, "concurrent prompt")
			results[i] = res
			errs[i] = err
		}()
	}
	wg.Wait()

	for i := 0; i < goroutines; i++ {
		require.NoError(t, errs[i], "goroutine %d returned an error", i)
		require.NotNil(t, results[i], "goroutine %d returned nil result", i)
		assert.NotEmpty(t, results[i].Provider, "goroutine %d result has empty Provider", i)
	}
}

// TestConcurrency_RegistryHotPath runs 100 goroutines concurrently doing
// Register (with unique names), List, and IsRegistered to verify no data race
// occurs on the Registry's internal map.
func TestConcurrency_RegistryHotPath(t *testing.T) {
	reg := NewRegistry()
	// Pre-populate one shared entry that all readers can check.
	reg.Register("shared-hot", stubFactory("shared-hot"))

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			name := fmt.Sprintf("hot-%d", i)
			reg.Register(name, stubFactory(name))
			_ = reg.IsRegistered(name)
			_ = reg.List()
			_ = reg.IsRegistered("shared-hot")
		}()
	}
	wg.Wait()

	// All goroutine-registered entries must still be present.
	for i := 0; i < goroutines; i++ {
		assert.True(t, reg.IsRegistered(fmt.Sprintf("hot-%d", i)))
	}
	assert.True(t, reg.IsRegistered("shared-hot"))
}

// TestConcurrency_CacheContention runs 50 goroutines doing concurrent Put and
// Get on an ExactCache. The test verifies no data race occurs and the cache
// remains structurally valid after the storm.
func TestConcurrency_CacheContention(t *testing.T) {
	c := cache.NewExactCache(64)
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			prompt := fmt.Sprintf("prompt-%d", i%10) // intentional key collisions
			resp := &cache.CachedResponse{
				Text:      fmt.Sprintf("text-%d", i),
				Model:     "contention-model",
				Timestamp: time.Now(),
			}
			// Even goroutines write, odd goroutines read.
			if i%2 == 0 {
				c.Put(img, prompt, resp)
			} else {
				_, _ = c.Get(img, prompt)
			}
		}()
	}
	wg.Wait()

	// Cache size must be within declared bounds.
	assert.LessOrEqual(t, c.Size(), 64,
		"cache size must not exceed maxEntries after concurrent access")
}
