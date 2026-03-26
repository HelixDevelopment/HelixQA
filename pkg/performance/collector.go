// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package performance

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// CommandRunner abstracts command execution so the collector
// can be tested without a real device attached.
type CommandRunner interface {
	// Run executes a named command with the given arguments and
	// returns its combined output.
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// rePSS matches the TOTAL PSS line in `dumpsys meminfo` output.
//
//	Example line:  "TOTAL            45000   ..."
var rePSS = regexp.MustCompile(`TOTAL\s+(\d+)`)

// reCPU matches a percentage value in a `dumpsys cpuinfo` line.
//
//	Example:  "12.5% com.example.app/..."
var reCPU = regexp.MustCompile(`(\d+\.?\d*)%`)

// MetricsCollector gathers performance metrics from an Android
// device via ADB for a specific application package.
type MetricsCollector struct {
	pkg      string
	platform string
	runner   CommandRunner
}

// Option configures a MetricsCollector.
type Option func(*MetricsCollector)

// WithCommandRunner replaces the default exec-based runner with
// a custom implementation (useful for unit tests).
func WithCommandRunner(r CommandRunner) Option {
	return func(c *MetricsCollector) {
		c.runner = r
	}
}

// New creates a MetricsCollector for the given package name and
// platform label. Apply Option functions to customise behaviour.
func New(pkg, platform string, opts ...Option) *MetricsCollector {
	c := &MetricsCollector{
		pkg:      pkg,
		platform: platform,
		runner:   &execRunner{},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// CollectMemory runs `adb shell dumpsys meminfo <pkg>` and
// returns a MetricSnapshot with the TOTAL PSS value in KB.
func (c *MetricsCollector) CollectMemory(
	ctx context.Context,
) (MetricSnapshot, error) {
	out, err := c.runner.Run(
		ctx, "adb", "shell", "dumpsys", "meminfo", c.pkg,
	)
	if err != nil {
		return MetricSnapshot{}, fmt.Errorf(
			"dumpsys meminfo: %w", err,
		)
	}

	matches := rePSS.FindSubmatch(out)
	if matches == nil {
		return MetricSnapshot{}, fmt.Errorf(
			"TOTAL PSS not found in meminfo output",
		)
	}

	kb, err := strconv.ParseFloat(string(matches[1]), 64)
	if err != nil {
		return MetricSnapshot{}, fmt.Errorf(
			"parse PSS value %q: %w", string(matches[1]), err,
		)
	}

	return MetricSnapshot{
		Type:      MetricMemoryRSS,
		Value:     kb,
		Timestamp: time.Now(),
		Platform:  c.platform,
	}, nil
}

// CollectCPU runs `adb shell dumpsys cpuinfo`, locates the line
// that contains the package name, and returns a MetricSnapshot
// with the CPU percentage.
func (c *MetricsCollector) CollectCPU(
	ctx context.Context,
) (MetricSnapshot, error) {
	out, err := c.runner.Run(
		ctx, "adb", "shell", "dumpsys", "cpuinfo",
	)
	if err != nil {
		return MetricSnapshot{}, fmt.Errorf(
			"dumpsys cpuinfo: %w", err,
		)
	}

	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, c.pkg) {
			continue
		}
		m := reCPU.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		pct, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			return MetricSnapshot{}, fmt.Errorf(
				"parse CPU value %q: %w", m[1], err,
			)
		}
		return MetricSnapshot{
			Type:      MetricCPUPercent,
			Value:     pct,
			Timestamp: time.Now(),
			Platform:  c.platform,
		}, nil
	}

	return MetricSnapshot{}, fmt.Errorf(
		"package %q not found in cpuinfo output", c.pkg,
	)
}
