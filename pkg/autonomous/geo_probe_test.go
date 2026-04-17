// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package autonomous

import (
	"context"
	"testing"
	"time"
)

func TestGetAlternativeApp(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"com.google.android.youtube", "ru.rutube.app"},
		{"com.netflix.mediaclient", "ru.kinopoisk"},
		{"tv.pluto.android", "ru.rutube.app"},
		{"ru.kinopoisk", "com.vkontakte.android"}, // known endpoint, no explicit alt → generic
		{"com.example.unknown", ""},               // unknown → empty
	}
	for _, c := range cases {
		if got := GetAlternativeApp(c.in); got != c.want {
			t.Errorf("GetAlternativeApp(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestProbeGeoRestriction_Cached(t *testing.T) {
	ResetGeoCache()
	t.Cleanup(ResetGeoCache)

	callCount := 0
	probeHostFunc = func(ctx context.Context, device, host string) *GeoProbeResult {
		callCount++
		return &GeoProbeResult{Restricted: false, Reason: "HTTP 200"}
	}
	t.Cleanup(func() { probeHostFunc = runAdbProbe })

	ctx := context.Background()
	first, err := ProbeGeoRestriction(ctx, "dev1", "com.google.android.youtube")
	if err != nil {
		t.Fatalf("probe 1: %v", err)
	}
	if first.Restricted {
		t.Errorf("expected not restricted, got %+v", first)
	}

	// Same device + pkg → cached, probe not called again.
	second, _ := ProbeGeoRestriction(ctx, "dev1", "com.google.android.youtube")
	if second != first {
		t.Errorf("expected same cached pointer")
	}
	if callCount != 1 {
		t.Errorf("probeHost called %d times, want 1 (second should be cached)", callCount)
	}

	// Different device → separate cache entry.
	_, _ = ProbeGeoRestriction(ctx, "dev2", "com.google.android.youtube")
	if callCount != 2 {
		t.Errorf("probeHost called %d times, want 2 after dev2", callCount)
	}
}

func TestProbeGeoRestriction_Restricted_SetsAlternative(t *testing.T) {
	ResetGeoCache()
	t.Cleanup(ResetGeoCache)

	probeHostFunc = func(ctx context.Context, device, host string) *GeoProbeResult {
		return &GeoProbeResult{Restricted: true, Reason: "HTTP 403"}
	}
	t.Cleanup(func() { probeHostFunc = runAdbProbe })

	r, err := ProbeGeoRestriction(context.Background(), "dev1", "com.netflix.mediaclient")
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if !r.Restricted {
		t.Fatalf("expected restricted, got %+v", r)
	}
	if r.Alternative != "ru.kinopoisk" {
		t.Errorf("Alternative = %q, want ru.kinopoisk", r.Alternative)
	}
	if r.CachedAt.IsZero() {
		t.Errorf("CachedAt not set")
	}
}

func TestProbeGeoRestriction_UnknownPackage_AssumedReachable(t *testing.T) {
	ResetGeoCache()
	t.Cleanup(ResetGeoCache)

	// probeHost MUST NOT be called for unknown endpoints — no network
	// poke for apps we don't have a known endpoint for.
	probeHostFunc = func(ctx context.Context, device, host string) *GeoProbeResult {
		t.Fatalf("probeHost should not be called for unknown package")
		return nil
	}
	t.Cleanup(func() { probeHostFunc = runAdbProbe })

	r, err := ProbeGeoRestriction(context.Background(), "dev1", "com.example.unknown")
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

func TestProbeGeoRestriction_RespectsContext(t *testing.T) {
	ResetGeoCache()
	t.Cleanup(ResetGeoCache)

	probeHostFunc = func(ctx context.Context, device, host string) *GeoProbeResult {
		// Honor the context by sleeping — test that a cancelled context
		// still returns *some* result (probeHost is free to return
		// Restricted=true on ctx timeout).
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
	r, err := ProbeGeoRestriction(ctx, "dev1", "com.google.android.youtube")
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if r == nil {
		t.Fatalf("probe returned nil result")
	}
}
