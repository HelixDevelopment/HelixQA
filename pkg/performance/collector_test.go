// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package performance

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mock runner ---

type mockRunner struct {
	responses map[string]mockResponse
}

type mockResponse struct {
	output []byte
	err    error
}

func newMockRunner() *mockRunner {
	return &mockRunner{
		responses: make(map[string]mockResponse),
	}
}

// On registers an expected command key with its canned response.
// The key is matched against "name arg1 arg2 ..." with prefix
// matching, mirroring the detector package convention.
func (m *mockRunner) On(key string, output []byte, err error) {
	m.responses[key] = mockResponse{output: output, err: err}
}

func (m *mockRunner) Run(
	ctx context.Context, name string, args ...string,
) ([]byte, error) {
	key := name
	if len(args) > 0 {
		key = name + " " + strings.Join(args, " ")
	}

	// Exact match.
	if resp, ok := m.responses[key]; ok {
		return resp.output, resp.err
	}
	// Name-only match.
	if resp, ok := m.responses[name]; ok {
		return resp.output, resp.err
	}
	// Prefix match.
	for k, resp := range m.responses {
		if strings.HasPrefix(key, k) {
			return resp.output, resp.err
		}
	}

	return nil, fmt.Errorf("no mock for: %s", key)
}

// --- CollectMemory tests ---

func TestCollectMemory_ParsesTotalPSS(t *testing.T) {
	mock := newMockRunner()
	meminfo := "" +
		"** MEMINFO in pid 1234 [com.example.app] **\n" +
		"                   Pss  Private  Private  SwapPss\n" +
		"                 Total    Dirty    Clean    Dirty\n" +
		"                ------   ------   ------   ------\n" +
		"  Native Heap    12000    12000        0        0\n" +
		"  Dalvik Heap    18000    18000        0        0\n" +
		"        Stack     1000     1000        0        0\n" +
		"       TOTAL    45000    40000        0        0\n"
	mock.On(
		"adb shell dumpsys meminfo com.example.app",
		[]byte(meminfo),
		nil,
	)

	c := New(
		"com.example.app", "android",
		WithCommandRunner(mock),
	)

	snap, err := c.CollectMemory(context.Background())
	require.NoError(t, err)
	assert.Equal(t, MetricMemoryRSS, snap.Type)
	assert.Equal(t, float64(45000), snap.Value)
	assert.Equal(t, "android", snap.Platform)
	assert.WithinDuration(t, time.Now(), snap.Timestamp, 5*time.Second)
}

func TestCollectMemory_CommandError(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"adb shell dumpsys meminfo com.example.app",
		nil,
		fmt.Errorf("adb: device not found"),
	)

	c := New(
		"com.example.app", "android",
		WithCommandRunner(mock),
	)

	_, err := c.CollectMemory(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dumpsys meminfo")
}

func TestCollectMemory_MissingTotalLine(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"adb shell dumpsys meminfo com.example.app",
		[]byte("no useful output here\n"),
		nil,
	)

	c := New(
		"com.example.app", "android",
		WithCommandRunner(mock),
	)

	_, err := c.CollectMemory(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TOTAL PSS not found")
}

// --- CollectCPU tests ---

func TestCollectCPU_ParsesCPUPercent(t *testing.T) {
	mock := newMockRunner()
	cpuinfo := "" +
		"Load: 3.2 / 2.8 / 2.1\n" +
		"CPU usage from 1000ms to 0ms ago:\n" +
		"  12.5% com.example.app/com.example.app.MainActivity\n" +
		"   3.1% system_server\n"
	mock.On(
		"adb shell dumpsys cpuinfo",
		[]byte(cpuinfo),
		nil,
	)

	c := New(
		"com.example.app", "android",
		WithCommandRunner(mock),
	)

	snap, err := c.CollectCPU(context.Background())
	require.NoError(t, err)
	assert.Equal(t, MetricCPUPercent, snap.Type)
	assert.Equal(t, 12.5, snap.Value)
	assert.Equal(t, "android", snap.Platform)
	assert.WithinDuration(t, time.Now(), snap.Timestamp, 5*time.Second)
}

func TestCollectCPU_CommandError(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"adb shell dumpsys cpuinfo",
		nil,
		fmt.Errorf("adb: device offline"),
	)

	c := New(
		"com.example.app", "android",
		WithCommandRunner(mock),
	)

	_, err := c.CollectCPU(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dumpsys cpuinfo")
}

func TestCollectCPU_PackageNotFound(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"adb shell dumpsys cpuinfo",
		[]byte("  5.0% some.other.app/...\n"),
		nil,
	)

	c := New(
		"com.example.app", "android",
		WithCommandRunner(mock),
	)

	_, err := c.CollectCPU(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found in cpuinfo")
}

// --- DetectMemoryLeak tests (on MetricsTimeline) ---

func TestDetectMemoryLeak_LeakDetected(t *testing.T) {
	// 15% growth — above a 10% threshold.
	base := time.Now()
	tl := &MetricsTimeline{Platform: "android"}
	tl.Add(MetricSnapshot{
		Type:      MetricMemoryRSS,
		Value:     100000,
		Timestamp: base,
		Platform:  "android",
	})
	tl.Add(MetricSnapshot{
		Type:      MetricMemoryRSS,
		Value:     110000,
		Timestamp: base.Add(30 * time.Second),
		Platform:  "android",
	})
	tl.Add(MetricSnapshot{
		Type:      MetricMemoryRSS,
		Value:     115000,
		Timestamp: base.Add(60 * time.Second),
		Platform:  "android",
	})

	indicator := tl.DetectMemoryLeak(10.0)
	require.NotNil(t, indicator)
	assert.True(t, indicator.IsLeak)
	assert.Equal(t, "android", indicator.Platform)
	assert.Equal(t, float64(100000), indicator.StartKB)
	assert.Equal(t, float64(115000), indicator.EndKB)
	assert.InDelta(t, 15.0, indicator.GrowthPercent, 0.01)
	assert.InDelta(t, 60.0, indicator.DurationSecs, 1.0)
}

func TestDetectMemoryLeak_NoLeak(t *testing.T) {
	// 2% growth — below a 10% threshold.
	base := time.Now()
	tl := &MetricsTimeline{Platform: "android"}
	tl.Add(MetricSnapshot{
		Type:      MetricMemoryRSS,
		Value:     100000,
		Timestamp: base,
		Platform:  "android",
	})
	tl.Add(MetricSnapshot{
		Type:      MetricMemoryRSS,
		Value:     102000,
		Timestamp: base.Add(60 * time.Second),
		Platform:  "android",
	})

	indicator := tl.DetectMemoryLeak(10.0)
	require.NotNil(t, indicator)
	assert.False(t, indicator.IsLeak)
	assert.InDelta(t, 2.0, indicator.GrowthPercent, 0.01)
}

func TestDetectMemoryLeak_InsufficientData(t *testing.T) {
	tl := &MetricsTimeline{Platform: "android"}
	// Only one sample — not enough to detect a trend.
	tl.Add(MetricSnapshot{
		Type:      MetricMemoryRSS,
		Value:     50000,
		Timestamp: time.Now(),
		Platform:  "android",
	})

	indicator := tl.DetectMemoryLeak(10.0)
	assert.Nil(t, indicator)
}

func TestDetectMemoryLeak_NoSamples(t *testing.T) {
	tl := &MetricsTimeline{Platform: "android"}
	assert.Nil(t, tl.DetectMemoryLeak(10.0))
}

// --- MetricsTimeline helper tests ---

func TestMetricsTimeline_Add(t *testing.T) {
	tl := &MetricsTimeline{Platform: "web"}
	tl.Add(MetricSnapshot{Type: MetricCPUPercent, Value: 5.0})
	tl.Add(MetricSnapshot{Type: MetricMemoryRSS, Value: 8192})
	assert.Len(t, tl.Snapshots, 2)
}

func TestMetricsTimeline_OfType(t *testing.T) {
	tl := &MetricsTimeline{Platform: "web"}
	tl.Add(MetricSnapshot{Type: MetricCPUPercent, Value: 5.0})
	tl.Add(MetricSnapshot{Type: MetricMemoryRSS, Value: 8192})
	tl.Add(MetricSnapshot{Type: MetricCPUPercent, Value: 7.0})

	cpu := tl.OfType(MetricCPUPercent)
	assert.Len(t, cpu, 2)

	mem := tl.OfType(MetricMemoryRSS)
	assert.Len(t, mem, 1)

	fps := tl.OfType(MetricFPS)
	assert.Empty(t, fps)
}

// --- New / WithCommandRunner option test ---

func TestNew_WithCommandRunner(t *testing.T) {
	mock := newMockRunner()
	c := New("com.test", "android", WithCommandRunner(mock))
	assert.NotNil(t, c.runner)
	assert.Equal(t, "com.test", c.pkg)
	assert.Equal(t, "android", c.platform)
}
