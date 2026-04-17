// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package autonomous: geo_probe.go — per-constitution connectivity probe.
//
// Some video apps are geo-restricted in certain regions and require a VPN
// or a local substitute. The probe hits the app's content endpoint before
// any playback attempt and marks the app GEO_RESTRICTED when unreachable
// so tests can SKIP (not FAIL) and optionally substitute an alternative.
//
// This file is INTENTIONALLY project-agnostic. It ships with EMPTY maps of
// endpoints and alternatives. Callers (any project — ATMOSphere, a TV
// vendor, a generic Android farm) register the packages they care about
// via RegisterEndpoint / RegisterAlternative, or mutate the exported maps
// directly. The probing mechanism itself — curl-over-adb with ping
// fallback, sync.Map caching per device-per-session — is generic.
//
// Thread-safety: RegisterEndpoint / RegisterAlternative / SetGenericAlternative
// are guarded by registryMu. The geoCache is a sync.Map.

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
	Package     string
	Restricted  bool
	Reason      string
	Alternative string
	CachedAt    time.Time
}

// geoCache holds GeoProbeResult values keyed by "<device>|<pkg>". Entries
// persist for the lifetime of the process — the constitution requires
// "check once, reuse result" per device per session.
var geoCache sync.Map

var (
	registryMu         sync.RWMutex
	knownEndpoints     = map[string]string{}
	alternatives       = map[string]string{}
	genericAlternative = "" // empty means "no fallback, skip test entirely"
)

// RegisterEndpoint teaches the probe that pkg can be reached by probing
// https://host/. Pass host="" to explicitly mark pkg as "no probe
// endpoint" (the probe is skipped and the app is assumed reachable).
func RegisterEndpoint(pkg, host string) {
	registryMu.Lock()
	defer registryMu.Unlock()
	knownEndpoints[pkg] = host
}

// RegisterAlternative associates a substitute package for a geo-
// restricted one. Pass alt="" to clear the mapping.
func RegisterAlternative(pkg, alt string) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if alt == "" {
		delete(alternatives, pkg)
		return
	}
	alternatives[pkg] = alt
}

// SetGenericAlternative sets the fallback substitute used when a package
// has a known endpoint but no explicit Alternatives entry. Empty string
// disables the generic fallback.
func SetGenericAlternative(alt string) {
	registryMu.Lock()
	defer registryMu.Unlock()
	genericAlternative = alt
}

// GetAlternativeApp returns the recommended substitute for a geo-
// restricted package. Empty string means "no alternative, skip the test".
func GetAlternativeApp(pkg string) string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	if alt, ok := alternatives[pkg]; ok {
		return alt
	}
	if _, known := knownEndpoints[pkg]; known {
		return genericAlternative
	}
	return ""
}

// lookupEndpoint returns (host, known) for pkg under registryMu's read
// lock. Separated so callers can hold a consistent snapshot.
func lookupEndpoint(pkg string) (string, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	host, ok := knownEndpoints[pkg]
	return host, ok
}

// ProbeGeoRestriction runs the probe for <device, pkg>, caching the
// outcome. device is an adb serial (-s target) or empty for default.
// Returns a non-nil result even on probe-execution failures — callers
// should inspect result.Restricted, not the error, for gating.
func ProbeGeoRestriction(ctx context.Context, device, pkg string) (*GeoProbeResult, error) {
	key := device + "|" + pkg
	if cached, ok := geoCache.Load(key); ok {
		return cached.(*GeoProbeResult), nil
	}

	host, known := lookupEndpoint(pkg)
	if !known || host == "" {
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

// probeHostFunc is the probe implementation — overridable by tests.
var probeHostFunc = runAdbProbe

func probeHost(ctx context.Context, device, host string) *GeoProbeResult {
	return probeHostFunc(ctx, device, host)
}

// runAdbProbe executes the actual adb+curl/ping probe.
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
	}

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

// ResetGeoCache clears the in-process cache. Tests only.
func ResetGeoCache() {
	geoCache.Range(func(k, _ any) bool {
		geoCache.Delete(k)
		return true
	})
}

// ResetGeoRegistry clears all RegisterEndpoint/RegisterAlternative state.
// Tests only.
func ResetGeoRegistry() {
	registryMu.Lock()
	defer registryMu.Unlock()
	knownEndpoints = map[string]string{}
	alternatives = map[string]string{}
	genericAlternative = ""
}
