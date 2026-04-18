// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"strings"
	"testing"

	"digital.vasic.helixqa/pkg/nexus"
	"digital.vasic.helixqa/pkg/nexus/browser"
	"digital.vasic.helixqa/pkg/nexus/mobile"
)

// TestBuildNexusAdapterStack_W10_BrowserOnlyCampaign locks in W10
// from docs/nexus/remaining-work.md: the CLI must be able to assemble
// a Nexus adapter stack that contains a pre-instrumented browser
// adapter (so NexusMetrics counters populate) without requiring
// Appium to be configured.
func TestBuildNexusAdapterStack_W10_BrowserOnlyCampaign(t *testing.T) {
	stack, err := BuildNexusAdapterStack(
		&mockDriver{},
		browser.Config{Engine: browser.EngineChromedp},
		"", // no Appium — browser-only stack
		mobile.Capabilities{},
	)
	if err != nil {
		t.Fatalf("BuildNexusAdapterStack: %v", err)
	}
	if stack == nil || stack.Registry == nil || stack.Metrics == nil {
		t.Fatal("stack / registry / metrics must be non-nil")
	}
	if stack.Browser == nil {
		t.Fatal("browser-only stack must wire a browser adapter")
	}
	if stack.Mobile != nil {
		t.Error("empty appiumURL must yield a nil mobile adapter")
	}
	desc := stack.Describe()
	if !strings.Contains(desc, "browser=instrumented browser") {
		t.Errorf("Describe missing browser wiring: %q", desc)
	}
	if !strings.Contains(desc, "mobile=disabled") {
		t.Errorf("Describe missing mobile-disabled marker: %q", desc)
	}
}

// TestBuildNexusAdapterStack_W10_MobileOnlyCampaign confirms a
// mobile-only stack (no browser driver) is valid for Android / iOS
// campaigns that never touch the web.
func TestBuildNexusAdapterStack_W10_MobileOnlyCampaign(t *testing.T) {
	stack, err := BuildNexusAdapterStack(
		nil,
		browser.Config{},
		"http://appium.local:4723",
		mobile.Capabilities{
			Platform:    mobile.PlatformAndroid,
			DeviceName:  "Pixel 8",
			AppPackage:  "com.example.app",
			AppActivity: ".MainActivity",
		},
	)
	if err != nil {
		t.Fatalf("BuildNexusAdapterStack: %v", err)
	}
	if stack.Browser != nil {
		t.Error("nil browserDriver must yield a nil browser adapter")
	}
	if stack.Mobile == nil {
		t.Fatal("mobile-only stack must wire a mobile adapter")
	}
	desc := stack.Describe()
	if !strings.Contains(desc, "mobile=appium") {
		t.Errorf("Describe missing mobile=appium marker: %q", desc)
	}
}

// TestBuildNexusAdapterStack_W10_RegistryPopulated proves the stack
// ships with the full NexusMetrics catalogue wired into a shared
// Registry so a single /metrics scrape surfaces every adapter.
func TestBuildNexusAdapterStack_W10_RegistryPopulated(t *testing.T) {
	stack, err := BuildNexusAdapterStack(
		&mockDriver{},
		browser.Config{Engine: browser.EngineChromedp},
		"",
		mobile.Capabilities{},
	)
	if err != nil {
		t.Fatal(err)
	}
	// NexusMetrics has non-zero counters + gauges + histograms
	// registered. The exact count is an implementation detail; we
	// only assert it's above a sane floor.
	total := len(stack.Registry.Counters()) +
		len(stack.Registry.Gauges()) +
		len(stack.Registry.Histograms())
	if total < 3 {
		t.Errorf("registry holds %d metrics, want at least 3", total)
	}
}

// mockDriver is a local no-op browser.Driver implementation used
// only by these tests. It satisfies the browser.Driver contract so
// BuildNexusAdapterStack can wire it into an InstrumentedEngine.
type mockDriver struct{}

func (*mockDriver) Kind() browser.EngineType { return browser.EngineChromedp }
func (*mockDriver) Open(_ context.Context, _ browser.Config) (browser.SessionHandle, error) {
	return &mockSession{}, nil
}

type mockSession struct{}

func (*mockSession) Close() error                               { return nil }
func (*mockSession) Navigate(_ context.Context, _ string) error { return nil }
func (*mockSession) Snapshot(_ context.Context) (*nexus.Snapshot, error) {
	return &nexus.Snapshot{}, nil
}
func (*mockSession) Click(_ context.Context, _ nexus.ElementRef) error          { return nil }
func (*mockSession) Type(_ context.Context, _ nexus.ElementRef, _ string) error { return nil }
func (*mockSession) Screenshot(_ context.Context) ([]byte, error)               { return []byte{}, nil }
func (*mockSession) Scroll(_ context.Context, _, _ int) error                   { return nil }
