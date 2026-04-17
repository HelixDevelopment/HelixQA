// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// W10 closure: Nexus adapter registration for the helixqa CLI.
//
// This file provides the factory functions that build
// pre-instrumented Nexus browser / mobile / desktop adapters so the
// CLI can opt into the Nexus stack via the --nexus flag instead of
// the pre-Nexus legacy runtime. Every adapter returned from here is
// wrapped with observability.NexusMetrics + DefaultTracer so
// Grafana panels populate the moment the autonomous pipeline starts
// driving sessions.

package main

import (
	"fmt"

	"digital.vasic.helixqa/pkg/nexus"
	"digital.vasic.helixqa/pkg/nexus/browser"
	"digital.vasic.helixqa/pkg/nexus/mobile"
	"digital.vasic.helixqa/pkg/nexus/observability"
)

// NexusAdapterStack bundles the browser / mobile adapters the CLI
// swaps in when the operator runs `helixqa autonomous --nexus`. The
// stack is self-contained: every adapter emits spans + metrics into
// the supplied Registry so a single /metrics scrape surfaces every
// platform's activity.
type NexusAdapterStack struct {
	Registry *observability.Registry
	Metrics  *observability.NexusMetrics
	Browser  nexus.Adapter
	Mobile   nexus.Adapter
}

// BuildNexusAdapterStack constructs the Nexus stack for the running
// CLI process. It accepts caller-supplied browser + mobile
// configuration so projects that want non-default drivers (chromedp
// vs rod, a specific Appium endpoint) can pin their choices.
//
// Arguments:
//
//   - browserDriver — concrete browser driver (chromedp / rod / etc).
//     Nil returns a stack without a browser adapter, useful for
//     mobile-only campaigns.
//   - browserCfg — browser engine configuration.
//   - appiumURL — Appium server URL. Empty returns a stack without a
//     mobile adapter, useful for web-only campaigns.
//   - caps — mobile capabilities (package, activity, device name).
func BuildNexusAdapterStack(
	browserDriver browser.Driver,
	browserCfg browser.Config,
	appiumURL string,
	caps mobile.Capabilities,
) (*NexusAdapterStack, error) {
	reg := observability.NewRegistry()
	metrics := observability.DefaultMetrics(reg)

	stack := &NexusAdapterStack{
		Registry: reg,
		Metrics:  metrics,
	}

	if browserDriver != nil {
		inst, err := browser.NewInstrumentedEngine(browserDriver, browserCfg, metrics)
		if err != nil {
			return nil, fmt.Errorf("nexus: build browser adapter: %w", err)
		}
		stack.Browser = inst
	}

	if appiumURL != "" {
		m, err := mobile.NewEngine(appiumURL, caps)
		if err != nil {
			return nil, fmt.Errorf("nexus: build mobile adapter: %w", err)
		}
		stack.Mobile = m
	}

	return stack, nil
}

// Describe returns a short, human-readable summary of the registered
// adapters. The CLI prints this when --nexus is active so operators
// can see at a glance which adapter set is in play.
func (s *NexusAdapterStack) Describe() string {
	browserLabel := "disabled"
	if s.Browser != nil {
		browserLabel = "instrumented browser"
	}
	mobileLabel := "disabled"
	if s.Mobile != nil {
		mobileLabel = "appium"
	}
	metricsCount := len(s.Registry.Counters()) +
		len(s.Registry.Gauges()) +
		len(s.Registry.Histograms())
	return fmt.Sprintf(
		"Nexus stack: browser=%s mobile=%s metrics=%d",
		browserLabel, mobileLabel, metricsCount,
	)
}
