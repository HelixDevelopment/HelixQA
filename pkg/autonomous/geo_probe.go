// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package autonomous: geo_probe.go — per-constitution connectivity probe.
//
// Some video apps (YouTube, Netflix, Disney+, etc.) are geo-restricted in
// certain regions and require a VPN. A test that tries to play content in
// such an app will appear to fail (timeout, black screen, HTTP 403) when
// in reality the app cannot reach its CDN. The constitution requires that
// we probe each app's content endpoint BEFORE attempting playback, mark
// geo-restricted apps as SKIPPED (not FAILED), and substitute a local
// alternative when available.
//
// Results are cached per device for the lifetime of the process (sync.Map
// keyed by "<deviceSerial>|<package>") so we probe at most once per app
// per session.

package autonomous

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// GeoProbeResult is the cached outcome of probing a package for geo-
// restricted content connectivity.
type GeoProbeResult struct {
	// Package is the app package name the probe was run for.
	Package string
	// Restricted is true when the probe could not reach the app's
	// content endpoint. Tests MUST treat this as SKIPPED, not FAILED.
	Restricted bool
	// Reason carries a short human-readable explanation ("HTTP 403",
	// "timeout", "DNS failure", "curl missing").
	Reason string
	// Alternative is the recommended replacement package, empty if
	// none known.
	Alternative string
	// CachedAt records when the probe ran (for TTL/debugging).
	CachedAt time.Time
}

// geoCache holds GeoProbeResult values keyed by "<device>|<pkg>". Entries
// persist for the lifetime of the process — the constitution requires
// "check once, reuse result" per device per session.
var geoCache sync.Map

// KnownEndpoints maps package names to the content-API hostnames that
// must be reachable for the app to function. Empty host means "no known
// probe endpoint" (the probe is skipped and the app is assumed
// reachable).
var KnownEndpoints = map[string]string{
	// Google / geo-restricted in RU
	"com.google.android.youtube":     "www.youtube.com",
	"com.google.android.youtube.tv":  "www.youtube.com",
	"com.netflix.mediaclient":        "www.netflix.com",
	"com.disney.disneyplus":          "www.disneyplus.com",
	"com.hulu.plus":                  "www.hulu.com",
	"com.hbo.hbonow":                 "www.max.com",
	"tv.pluto.android":               "api.pluto.tv",
	"com.cbs.ott":                    "www.paramountplus.com",
	// RU-local, generally not geo-restricted here
	"ru.kinopoisk":                   "www.kinopoisk.ru",
	"ru.rutube.app":                  "rutube.ru",
	"com.vkontakte.android":          "vk.com",
	"ru.mail.mymusic":                "my.mail.ru",
	"ru.ivi.client":                  "www.ivi.ru",
}

// Alternatives maps a geo-restricted package to the recommended
// replacement that is available in the same region. Callers look up
// via GetAlternativeApp.
var Alternatives = map[string]string{
	"com.google.android.youtube":    "ru.rutube.app",
	"com.google.android.youtube.tv": "ru.rutube.app",
	"com.netflix.mediaclient":       "ru.kinopoisk",
	"com.disney.disneyplus":         "ru.kinopoisk",
	"com.hulu.plus":                 "ru.kinopoisk",
	"com.hbo.hbonow":                "ru.kinopoisk",
	"tv.pluto.android":              "ru.rutube.app",
	"com.cbs.ott":                   "ru.kinopoisk",
}

// GenericAlternative is returned when no specific replacement exists.
const GenericAlternative = "com.vkontakte.android"

// GetAlternativeApp returns the recommended substitute for a geo-
// restricted package. Empty string means "no alternative, skip the test
// entirely".
func GetAlternativeApp(pkg string) string {
	if alt, ok := Alternatives[pkg]; ok {
		return alt
	}
	if _, known := KnownEndpoints[pkg]; known {
		return GenericAlternative
	}
	return ""
}

// ProbeGeoRestriction runs the probe for <device, pkg>, caching the
// outcome. device is an adb serial (-s target). Returns a non-nil result
// even on probe-execution failures — callers should inspect
// result.Restricted, not the error, for the test-gating decision.
//
// The probe uses `adb -s <device> shell curl -sS -o /dev/null -w '%{http_code}' --connect-timeout 5 https://<host>`
// and treats any 2xx/3xx code as reachable. curl is assumed present on
// ATMOSphere builds (busybox provides it); if missing the probe falls
// back to `adb shell ping -c 1 -W 3 <host>`.
func ProbeGeoRestriction(ctx context.Context, device, pkg string) (*GeoProbeResult, error) {
	key := device + "|" + pkg
	if cached, ok := geoCache.Load(key); ok {
		return cached.(*GeoProbeResult), nil
	}

	host, known := KnownEndpoints[pkg]
	if !known || host == "" {
		// No known endpoint → assume reachable (tests proceed).
		result := &GeoProbeResult{
			Package:  pkg,
			Reason:   "no probe endpoint configured",
			CachedAt: time.Now(),
		}
		geoCache.Store(key, result)
		return result, nil
	}

	result := probeHost(ctx, device, host)
	result.Package = pkg
	result.CachedAt = time.Now()
	if result.Restricted {
		result.Alternative = GetAlternativeApp(pkg)
	}
	geoCache.Store(key, result)
	return result, nil
}

// probeHost is the low-level probe implementation — exposed for testing.
// Overridable via probeHostFunc so tests can inject a fake without
// touching adb.
var probeHostFunc = runAdbProbe

func probeHost(ctx context.Context, device, host string) *GeoProbeResult {
	return probeHostFunc(ctx, device, host)
}

// runAdbProbe executes the actual adb+curl/ping probe. Separated so
// tests can replace it via probeHostFunc.
func runAdbProbe(ctx context.Context, device, host string) *GeoProbeResult {
	curlArgs := []string{}
	if device != "" {
		curlArgs = append(curlArgs, "-s", device)
	}
	curlArgs = append(curlArgs,
		"shell",
		"curl", "-sS",
		"-o", "/dev/null",
		"-w", "%{http_code}",
		"--connect-timeout", "5",
		"https://"+host,
	)

	cctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cctx, "adb", curlArgs...).CombinedOutput()
	if err == nil {
		code := strings.TrimSpace(string(out))
		if len(code) == 3 && (code[0] == '2' || code[0] == '3') {
			return &GeoProbeResult{Reason: "HTTP " + code}
		}
		if len(code) == 3 && code[0] == '4' {
			return &GeoProbeResult{
				Restricted: true,
				Reason:     "HTTP " + code,
			}
		}
		// Ambiguous — fall through to ping fallback.
	}

	// Ping fallback. One packet, 3s deadline.
	pingArgs := []string{}
	if device != "" {
		pingArgs = append(pingArgs, "-s", device)
	}
	pingArgs = append(pingArgs, "shell", "ping", "-c", "1", "-W", "3", host)
	pctx, pcancel := context.WithTimeout(ctx, 6*time.Second)
	defer pcancel()
	if perr := exec.CommandContext(pctx, "adb", pingArgs...).Run(); perr == nil {
		return &GeoProbeResult{Reason: "ping ok (curl unavailable/ambiguous)"}
	}
	return &GeoProbeResult{
		Restricted: true,
		Reason:     fmt.Sprintf("unreachable (curl+ping failed: %v)", err),
	}
}

// ResetGeoCache clears the in-process cache. Only tests should call
// this; production code relies on session-scoped caching.
func ResetGeoCache() {
	geoCache.Range(func(k, _ any) bool {
		geoCache.Delete(k)
		return true
	})
}
