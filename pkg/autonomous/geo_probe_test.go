// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package autonomous

import (
	"context"
	"testing"
	"time"
)

func setupTestRegistry(t *testing.T) {
	t.Helper()
	ResetGeoRegistry()
	ResetGeoCache()
	t.Cleanup(func() {
		ResetGeoRegistry()
		ResetGeoCache()
	})
}

func TestRegisterEndpoint_And_Alternative(t *testing.T) {
	setupTestRegistry(t)

	RegisterEndpoint("com.example.video", "video.example.com")
	RegisterAlternative("com.example.video", "com.example.local")

	if got := GetAlternativeApp("com.example.video"); got != "com.example.local" {
		t.Errorf("GetAlternativeApp = %q, want com.example.local", got)
	}

	// Clear via empty alt
	RegisterAlternative("com.example.video", "")
	if got := GetAlternativeApp("com.example.video"); got != "" {
		t.Errorf("after clearing alt, GetAlternativeApp = %q, want empty (no generic set)", got)
	}
}

func TestGetAlternativeApp_GenericFallback(t *testing.T) {
	setupTestRegistry(t)

	RegisterEndpoint("com.example.video", "video.example.com")
	SetGenericAlternative("com.example.generic")

	// Known endpoint but no explicit alt → generic
	if got := GetAlternativeApp("com.example.video"); got != "com.example.generic" {
		t.Errorf("generic fallback: got %q, want com.example.generic", got)
	}

	// Unknown package → empty (no generic fallback for unknown apps)
	if got := GetAlternativeApp("com.unknown"); got != "" {
		t.Errorf("unknown package: got %q, want empty", got)
	}

	// Explicit alt beats generic
	RegisterAlternative("com.example.video", "com.example.specific")
	if got := GetAlternativeApp("com.example.video"); got != "com.example.specific" {
		t.Errorf("explicit alt should win: got %q, want com.example.specific", got)
	}
}

func TestProbeGeoRestriction_Cached(t *testing.T) {
	setupTestRegistry(t)
	RegisterEndpoint("com.example.video", "video.example.com")

	callCount := 0
	probeHostFunc = func(ctx context.Context, device, host string) *GeoProbeResult {
		callCount++
		return &GeoProbeResult{Restricted: false, Reason: "HTTP 200"}
	}
	t.Cleanup(func() { probeHostFunc = runAdbProbe })

	ctx := context.Background()
	first, err := ProbeGeoRestriction(ctx, "dev1", "com.example.video")
	if err != nil {
		t.Fatalf("probe 1: %v", err)
	}
	if first.Restricted {
		t.Errorf("expected not restricted, got %+v", first)
	}

	second, _ := ProbeGeoRestriction(ctx, "dev1", "com.example.video")
	if second != first {
		t.Errorf("expected same cached pointer")
	}
	if callCount != 1 {
		t.Errorf("probeHost called %d times, want 1 (second should be cached)", callCount)
	}

	_, _ = ProbeGeoRestriction(ctx, "dev2", "com.example.video")
	if callCount != 2 {
		t.Errorf("probeHost called %d times, want 2 after dev2", callCount)
	}
}

func TestProbeGeoRestriction_Restricted_SetsAlternative(t *testing.T) {
	setupTestRegistry(t)
	RegisterEndpoint("com.example.video", "video.example.com")
	RegisterAlternative("com.example.video", "com.example.local")

	probeHostFunc = func(ctx context.Context, device, host string) *GeoProbeResult {
		return &GeoProbeResult{Restricted: true, Reason: "HTTP 403"}
	}
	t.Cleanup(func() { probeHostFunc = runAdbProbe })

	r, err := ProbeGeoRestriction(context.Background(), "dev1", "com.example.video")
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if !r.Restricted {
		t.Fatalf("expected restricted, got %+v", r)
	}
	if r.Alternative != "com.example.local" {
		t.Errorf("Alternative = %q, want com.example.local", r.Alternative)
	}
	if r.CachedAt.IsZero() {
		t.Errorf("CachedAt not set")
	}
}

func TestProbeGeoRestriction_UnknownPackage_AssumedReachable(t *testing.T) {
	setupTestRegistry(t)

	probeHostFunc = func(ctx context.Context, device, host string) *GeoProbeResult {
		t.Fatalf("probeHost should not be called for unknown package")
		return nil
	}
	t.Cleanup(func() { probeHostFunc = runAdbProbe })

	r, err := ProbeGeoRestriction(context.Background(), "dev1", "com.unknown")
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if r.Restricted {
		t.Errorf("unknown package should NOT be marked restricted, got %+v", r)
	}
	if r.Reason == "" {
		t.Errorf("expected a reason to be set for caching clarity")
	}
}

func TestProbeGeoRestriction_ExplicitEmptyHost_SkipsProbe(t *testing.T) {
	setupTestRegistry(t)
	RegisterEndpoint("com.example.offline", "")

	probeHostFunc = func(ctx context.Context, device, host string) *GeoProbeResult {
		t.Fatalf("probeHost should not be called when host is explicitly empty")
		return nil
	}
	t.Cleanup(func() { probeHostFunc = runAdbProbe })

	r, err := ProbeGeoRestriction(context.Background(), "dev1", "com.example.offline")
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if r.Restricted {
		t.Errorf("explicit empty host should be assumed reachable, got restricted")
	}
}

func TestProbeGeoRestriction_RespectsContext(t *testing.T) {
	setupTestRegistry(t)
	RegisterEndpoint("com.example.video", "video.example.com")

	probeHostFunc = func(ctx context.Context, device, host string) *GeoProbeResult {
		select {
		case <-ctx.Done():
			return &GeoProbeResult{Restricted: true, Reason: "ctx cancelled"}
		case <-time.After(50 * time.Millisecond):
			return &GeoProbeResult{Reason: "HTTP 200"}
		}
	}
	t.Cleanup(func() { probeHostFunc = runAdbProbe })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	r, err := ProbeGeoRestriction(ctx, "dev1", "com.example.video")
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if r == nil {
		t.Fatalf("probe returned nil result")
	}
}

func TestResetGeoRegistry_ClearsAll(t *testing.T) {
	setupTestRegistry(t)
	RegisterEndpoint("a", "a.com")
	RegisterAlternative("a", "b")
	SetGenericAlternative("c")

	ResetGeoRegistry()

	if got := GetAlternativeApp("a"); got != "" {
		t.Errorf("after reset, GetAlternativeApp = %q, want empty", got)
	}
	if _, ok := lookupEndpoint("a"); ok {
		t.Errorf("after reset, lookupEndpoint should report unknown")
	}
}
