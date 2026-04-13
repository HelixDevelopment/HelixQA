// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package learning

import (
	"sync"
	"testing"
	"time"
)

func TestOptimizer_RecordSuccess(t *testing.T) {
	opt := NewProviderOptimizer()

	opt.RecordSuccess("providerA", 100*time.Millisecond, "button")
	opt.RecordSuccess("providerA", 200*time.Millisecond, "button")
	opt.RecordSuccess("providerA", 150*time.Millisecond, "text")

	m := opt.GetMetrics("providerA")
	if m == nil {
		t.Fatal("expected metrics for providerA, got nil")
	}
	if m.TotalRequests != 3 {
		t.Errorf("expected TotalRequests=3, got %d", m.TotalRequests)
	}
	if m.SuccessfulRequests != 3 {
		t.Errorf("expected SuccessfulRequests=3, got %d", m.SuccessfulRequests)
	}
	if m.FailedRequests != 0 {
		t.Errorf("expected FailedRequests=0, got %d", m.FailedRequests)
	}
	if m.AvgLatency == 0 {
		t.Error("expected non-zero AvgLatency")
	}
	if m.TotalLatency == 0 {
		t.Error("expected non-zero TotalLatency")
	}
	if m.ButtonAccuracy <= 0 {
		t.Errorf("expected positive ButtonAccuracy, got %f", m.ButtonAccuracy)
	}
}

func TestOptimizer_RecordFailure(t *testing.T) {
	opt := NewProviderOptimizer()

	opt.RecordFailure("providerB", "image")
	opt.RecordFailure("providerB", "general")
	opt.RecordFailure("providerB", "text")

	m := opt.GetMetrics("providerB")
	if m == nil {
		t.Fatal("expected metrics for providerB, got nil")
	}
	if m.TotalRequests != 3 {
		t.Errorf("expected TotalRequests=3, got %d", m.TotalRequests)
	}
	if m.FailedRequests != 3 {
		t.Errorf("expected FailedRequests=3, got %d", m.FailedRequests)
	}
	if m.SuccessfulRequests != 0 {
		t.Errorf("expected SuccessfulRequests=0, got %d", m.SuccessfulRequests)
	}
	// After 3 failures starting from 0.5 initial:
	// After 1: 0.9*0.5 + 0.1*0.0 = 0.45
	// After 2: 0.9*0.45 + 0.1*0.0 = 0.405
	// After 3: 0.9*0.405 + 0.1*0.0 = 0.3645
	if m.ImageAccuracy >= 0.5 {
		t.Errorf("expected ImageAccuracy < 0.5 after failures, got %f", m.ImageAccuracy)
	}
	if m.TextFieldAccuracy >= 0.5 {
		t.Errorf("expected TextFieldAccuracy < 0.5 after failures, got %f", m.TextFieldAccuracy)
	}
	if m.GeneralAccuracy >= 0.5 {
		t.Errorf("expected GeneralAccuracy < 0.5 after failures, got %f", m.GeneralAccuracy)
	}
}

func TestOptimizer_GetBestProvider(t *testing.T) {
	opt := NewProviderOptimizer()

	// providerGood: 9 successes, 1 failure — high success rate
	for i := 0; i < 9; i++ {
		opt.RecordSuccess("providerGood", 100*time.Millisecond, "general")
	}
	opt.RecordFailure("providerGood", "general")

	// providerBad: 1 success, 9 failures — low success rate
	opt.RecordSuccess("providerBad", 100*time.Millisecond, "general")
	for i := 0; i < 9; i++ {
		opt.RecordFailure("providerBad", "general")
	}

	best := opt.GetBestProvider("general", false)
	if best != "providerGood" {
		t.Errorf("expected providerGood as best provider, got %q", best)
	}
}

func TestOptimizer_GetBestProvider_UIType(t *testing.T) {
	opt := NewProviderOptimizer()

	// buttonSpecialist: high button accuracy, lower text accuracy
	for i := 0; i < 10; i++ {
		opt.RecordSuccess("buttonSpecialist", 100*time.Millisecond, "button")
	}
	for i := 0; i < 5; i++ {
		opt.RecordFailure("buttonSpecialist", "text")
	}

	// textSpecialist: high text accuracy, lower button accuracy
	for i := 0; i < 10; i++ {
		opt.RecordSuccess("textSpecialist", 100*time.Millisecond, "text")
	}
	for i := 0; i < 5; i++ {
		opt.RecordFailure("textSpecialist", "button")
	}

	bestForButton := opt.GetBestProvider("button", false)
	if bestForButton != "buttonSpecialist" {
		t.Errorf("expected buttonSpecialist for button uiType, got %q", bestForButton)
	}

	bestForText := opt.GetBestProvider("text", false)
	if bestForText != "textSpecialist" {
		t.Errorf("expected textSpecialist for text uiType, got %q", bestForText)
	}
}

func TestOptimizer_StaleProvider(t *testing.T) {
	opt := NewProviderOptimizer()

	// staleProvider: good metrics but LastUsed is 11 minutes ago
	for i := 0; i < 10; i++ {
		opt.RecordSuccess("staleProvider", 50*time.Millisecond, "general")
	}
	// freshProvider: fewer successes but recently used
	for i := 0; i < 5; i++ {
		opt.RecordSuccess("freshProvider", 100*time.Millisecond, "general")
	}

	// Manually set staleProvider's LastUsed to 11 minutes ago
	opt.mu.Lock()
	opt.metrics["staleProvider"].LastUsed = time.Now().Add(-11 * time.Minute)
	opt.mu.Unlock()

	best := opt.GetBestProvider("general", false)
	if best == "staleProvider" {
		t.Error("stale provider should have been skipped, but was returned as best")
	}
	if best != "freshProvider" {
		t.Errorf("expected freshProvider as best (non-stale), got %q", best)
	}
}

func TestOptimizer_ConcurrentAccess(t *testing.T) {
	opt := NewProviderOptimizer()

	var wg sync.WaitGroup
	providers := []string{"p1", "p2", "p3"}
	uiTypes := []string{"button", "text", "image", "general"}

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			provider := providers[idx%len(providers)]
			uiType := uiTypes[idx%len(uiTypes)]
			if idx%3 == 0 {
				opt.RecordFailure(provider, uiType)
			} else {
				opt.RecordSuccess(provider, time.Duration(idx)*time.Millisecond, uiType)
			}
		}(i)
	}
	wg.Wait()

	// Also test concurrent reads
	var wg2 sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg2.Add(1)
		go func(idx int) {
			defer wg2.Done()
			uiType := uiTypes[idx%len(uiTypes)]
			_ = opt.GetBestProvider(uiType, idx%2 == 0)
			_ = opt.GetAllMetrics()
		}(i)
	}
	wg2.Wait()

	// Verify total requests across all providers sum to 50
	all := opt.GetAllMetrics()
	var total int64
	for _, m := range all {
		total += m.TotalRequests
	}
	if total != 50 {
		t.Errorf("expected 50 total requests across all providers, got %d", total)
	}
}

func TestOptimizer_GetMetrics(t *testing.T) {
	opt := NewProviderOptimizer()

	// Non-existent provider returns nil
	m := opt.GetMetrics("nonexistent")
	if m != nil {
		t.Error("expected nil for non-existent provider")
	}

	opt.RecordSuccess("providerX", 300*time.Millisecond, "image")

	m = opt.GetMetrics("providerX")
	if m == nil {
		t.Fatal("expected non-nil metrics for providerX")
	}
	if m.ProviderName != "providerX" {
		t.Errorf("expected ProviderName=providerX, got %q", m.ProviderName)
	}

	// Verify it's a copy — mutating it should not affect internal state
	m.TotalRequests = 999
	m2 := opt.GetMetrics("providerX")
	if m2.TotalRequests == 999 {
		t.Error("GetMetrics should return a copy, not a reference")
	}
}

func TestOptimizer_GetAllMetrics(t *testing.T) {
	opt := NewProviderOptimizer()

	opt.RecordSuccess("alpha", 100*time.Millisecond, "button")
	opt.RecordSuccess("beta", 200*time.Millisecond, "text")
	opt.RecordFailure("gamma", "image")

	all := opt.GetAllMetrics()
	if len(all) != 3 {
		t.Errorf("expected 3 providers in all metrics, got %d", len(all))
	}

	for _, name := range []string{"alpha", "beta", "gamma"} {
		if _, ok := all[name]; !ok {
			t.Errorf("expected provider %q in all metrics", name)
		}
	}

	// Verify copy semantics — mutating the returned map should not affect internal state
	all["alpha"].TotalRequests = 999
	all2 := opt.GetAllMetrics()
	if all2["alpha"].TotalRequests == 999 {
		t.Error("GetAllMetrics should return copies, not references")
	}
}
