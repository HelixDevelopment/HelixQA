// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_CustomConfig(t *testing.T) {
	cfg := Config{
		StaleThreshold:   30 * time.Second,
		WarnThreshold:    20 * time.Second,
		PollInterval:     1 * time.Second,
		MaxKillsPerPhase: 3,
	}
	c := New(cfg)
	assert.Equal(t, cfg, c.config)
}

func TestRegisterStep_And_CompleteStep(t *testing.T) {
	c := New(DefaultConfig())
	c.RegisterStep("curiosity", "androidtv", 1, "tap home", nil)
	assert.Equal(t, 1, c.ActiveSteps())

	c.CompleteStep("curiosity", "androidtv", 1)
	assert.Equal(t, 0, c.ActiveSteps())
}

func TestHeartbeat_UpdatesLastBeat(t *testing.T) {
	c := New(DefaultConfig())
	c.RegisterStep("curiosity", "web", 5, "click nav", nil)

	// Get initial beat time.
	c.mu.Lock()
	info := c.steps[stepKey("curiosity", "web", 5)]
	initialBeat := info.LastBeat
	c.mu.Unlock()

	time.Sleep(10 * time.Millisecond)
	c.Heartbeat("curiosity", "web", 5)

	c.mu.Lock()
	updatedBeat := info.LastBeat
	c.mu.Unlock()

	assert.True(t, updatedBeat.After(initialBeat),
		"Heartbeat should update LastBeat")
}

func TestCheckSteps_KillsStaleStep(t *testing.T) {
	cfg := Config{
		StaleThreshold:   50 * time.Millisecond,
		WarnThreshold:    25 * time.Millisecond,
		PollInterval:     10 * time.Millisecond,
		MaxKillsPerPhase: 5,
	}
	c := New(cfg)

	var cancelled atomic.Bool
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wrappedCancel := func() {
		cancelled.Store(true)
		cancel()
	}

	c.RegisterStep(
		"curiosity", "androidtv", 3,
		"stale step", wrappedCancel,
	)

	// Manually backdate the LastBeat to trigger stale.
	c.mu.Lock()
	key := stepKey("curiosity", "androidtv", 3)
	c.steps[key].LastBeat = time.Now().Add(-100 * time.Millisecond)
	c.steps[key].StartedAt = time.Now().Add(-100 * time.Millisecond)
	c.mu.Unlock()

	c.checkSteps()

	assert.True(t, cancelled.Load(),
		"Cancel should have been called")
	assert.Equal(t, 0, c.ActiveSteps(),
		"Killed step should be removed")
	assert.Equal(t, 1, c.KillCount("curiosity"))
	assert.Equal(t, 1, c.TotalKills())

	_ = ctx // Suppress unused warning.
}

func TestCheckSteps_WarnsBeforeKill(t *testing.T) {
	cfg := Config{
		StaleThreshold:   100 * time.Millisecond,
		WarnThreshold:    30 * time.Millisecond,
		PollInterval:     10 * time.Millisecond,
		MaxKillsPerPhase: 5,
	}
	c := New(cfg)
	c.RegisterStep("execute", "web", 1, "slow step", nil)

	// Backdate to warn threshold but not stale.
	c.mu.Lock()
	key := stepKey("execute", "web", 1)
	c.steps[key].LastBeat = time.Now().Add(-50 * time.Millisecond)
	c.mu.Unlock()

	c.checkSteps()

	events := c.Events()
	require.Len(t, events, 1)
	assert.Equal(t, "warn", events[0].Action)
	assert.Equal(t, 1, c.ActiveSteps(),
		"Step should still be active after warn")
}

func TestShouldAbortPhase(t *testing.T) {
	cfg := Config{
		StaleThreshold:   10 * time.Millisecond,
		MaxKillsPerPhase: 2,
	}
	c := New(cfg)

	assert.False(t, c.ShouldAbortPhase("curiosity"))

	// Simulate 2 kills.
	c.mu.Lock()
	c.kills["curiosity"] = 2
	c.mu.Unlock()

	assert.True(t, c.ShouldAbortPhase("curiosity"))
}

func TestStartStop(t *testing.T) {
	cfg := Config{
		StaleThreshold:   50 * time.Millisecond,
		WarnThreshold:    25 * time.Millisecond,
		PollInterval:     10 * time.Millisecond,
		MaxKillsPerPhase: 5,
	}
	c := New(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c.Start(ctx)

	var killed atomic.Bool
	c.RegisterStep("curiosity", "tv", 1, "test",
		func() { killed.Store(true) })

	// Backdate to trigger kill.
	c.mu.Lock()
	key := stepKey("curiosity", "tv", 1)
	c.steps[key].LastBeat = time.Now().Add(-100 * time.Millisecond)
	c.steps[key].StartedAt = time.Now().Add(-100 * time.Millisecond)
	c.mu.Unlock()

	// Wait for the monitor loop to detect and kill.
	time.Sleep(50 * time.Millisecond)

	c.Stop()

	assert.True(t, killed.Load(),
		"Monitor loop should have killed the step")
}

func TestStopIdempotent(t *testing.T) {
	c := New(DefaultConfig())
	c.Stop()
	c.Stop() // Should not panic.
}

func TestSummary(t *testing.T) {
	c := New(DefaultConfig())
	c.mu.Lock()
	c.events = []Event{
		{Action: "kill"},
		{Action: "warn"},
		{Action: "warn"},
		{Action: "kill"},
	}
	c.mu.Unlock()

	summary := c.Summary()
	assert.Contains(t, summary, "2 kills")
	assert.Contains(t, summary, "2 warnings")
	assert.Contains(t, summary, "4 events")
}

func TestStepInfo_Duration(t *testing.T) {
	info := StepInfo{StartedAt: time.Now().Add(-5 * time.Second)}
	d := info.Duration()
	assert.InDelta(t, 5.0, d.Seconds(), 0.5)
}

func TestStepInfo_Duration_Zero(t *testing.T) {
	info := StepInfo{}
	assert.Equal(t, time.Duration(0), info.Duration())
}

func TestStepInfo_StaleDuration(t *testing.T) {
	info := StepInfo{
		StartedAt: time.Now().Add(-10 * time.Second),
		LastBeat:  time.Now().Add(-3 * time.Second),
	}
	d := info.StaleDuration()
	assert.InDelta(t, 3.0, d.Seconds(), 0.5)
}

func TestStepInfo_StaleDuration_NoBeat(t *testing.T) {
	info := StepInfo{
		StartedAt: time.Now().Add(-7 * time.Second),
	}
	d := info.StaleDuration()
	assert.InDelta(t, 7.0, d.Seconds(), 0.5)
}

func TestHeartbeat_NonexistentStep(t *testing.T) {
	c := New(DefaultConfig())
	// Should not panic.
	c.Heartbeat("x", "y", 99)
}

func TestCompleteStep_NonexistentStep(t *testing.T) {
	c := New(DefaultConfig())
	// Should not panic.
	c.CompleteStep("x", "y", 99)
}

func TestEvents_ReturnsCopy(t *testing.T) {
	c := New(DefaultConfig())
	c.mu.Lock()
	c.events = []Event{{Action: "kill"}}
	c.mu.Unlock()

	events := c.Events()
	events[0].Action = "modified"

	original := c.Events()
	assert.Equal(t, "kill", original[0].Action,
		"Events() should return a copy")
}
