// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package controller provides a QA Process Controller that
// monitors HelixQA autonomous sessions in real-time. It
// detects hangs, kills stuck steps, and forces progression
// to prevent sessions from stalling indefinitely.
//
// The controller runs as a background goroutine alongside
// the session pipeline and uses heartbeat-based liveness
// detection. When a step exceeds its stale threshold with
// no progress, the controller cancels it via the step's
// context.CancelFunc.
package controller

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// StepState represents the current state of a monitored
// step.
type StepState string

const (
	StepRunning   StepState = "running"
	StepCompleted StepState = "completed"
	StepKilled    StepState = "killed"
	StepTimedOut  StepState = "timed_out"
)

// StepInfo holds the metadata for a currently monitored
// step.
type StepInfo struct {
	Phase       string
	Platform    string
	StepNumber  int
	Description string
	StartedAt   time.Time
	LastBeat    time.Time
	State       StepState
	Cancel      context.CancelFunc
}

// Duration returns how long the step has been running.
func (s StepInfo) Duration() time.Duration {
	if s.StartedAt.IsZero() {
		return 0
	}
	return time.Since(s.StartedAt)
}

// StaleDuration returns how long since the last heartbeat.
func (s StepInfo) StaleDuration() time.Duration {
	if s.LastBeat.IsZero() {
		return s.Duration()
	}
	return time.Since(s.LastBeat)
}

// Event describes a controller action taken during
// monitoring.
type Event struct {
	Time        time.Time
	Phase       string
	Platform    string
	StepNumber  int
	Action      string // "kill", "warn", "healthy"
	Reason      string
	StaleFor    time.Duration
	StepRuntime time.Duration
}

// Config holds controller parameters.
type Config struct {
	// StaleThreshold is how long a step can go without a
	// heartbeat before being killed. Default: 90s.
	StaleThreshold time.Duration

	// WarnThreshold is how long before a warning is
	// emitted. Should be less than StaleThreshold.
	// Default: 60s (2/3 of stale).
	WarnThreshold time.Duration

	// PollInterval is how often the controller checks
	// step liveness. Default: 5s.
	PollInterval time.Duration

	// MaxKillsPerPhase caps how many steps can be killed
	// in a single phase before the controller escalates
	// to a phase-level abort recommendation. Default: 5.
	MaxKillsPerPhase int
}

// DefaultConfig returns sensible defaults matching the
// pipeline's 90-second per-step watchdog.
func DefaultConfig() Config {
	return Config{
		StaleThreshold:   90 * time.Second,
		WarnThreshold:    60 * time.Second,
		PollInterval:     5 * time.Second,
		MaxKillsPerPhase: 5,
	}
}

// Controller monitors HelixQA session steps and kills
// stuck ones. Thread-safe.
type Controller struct {
	config Config
	mu     sync.Mutex
	steps  map[string]*StepInfo // key: "phase:platform:step"
	events []Event
	kills  map[string]int // kills per phase
	done   chan struct{}
}

// New creates a Controller with the given config.
func New(cfg Config) *Controller {
	if cfg.StaleThreshold == 0 {
		cfg = DefaultConfig()
	}
	if cfg.WarnThreshold == 0 {
		cfg.WarnThreshold = cfg.StaleThreshold * 2 / 3
	}
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 5 * time.Second
	}
	if cfg.MaxKillsPerPhase == 0 {
		cfg.MaxKillsPerPhase = 5
	}
	return &Controller{
		config: cfg,
		steps:  make(map[string]*StepInfo),
		kills:  make(map[string]int),
		done:   make(chan struct{}),
	}
}

// stepKey builds the map key for a step.
func stepKey(phase, platform string, step int) string {
	return fmt.Sprintf("%s:%s:%d", phase, platform, step)
}

// RegisterStep begins monitoring a step. The provided
// cancel func will be called if the step becomes stale.
func (c *Controller) RegisterStep(
	phase, platform string,
	stepNum int,
	description string,
	cancel context.CancelFunc,
) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	key := stepKey(phase, platform, stepNum)
	c.steps[key] = &StepInfo{
		Phase:       phase,
		Platform:    platform,
		StepNumber:  stepNum,
		Description: description,
		StartedAt:   now,
		LastBeat:    now,
		State:       StepRunning,
		Cancel:      cancel,
	}
}

// Heartbeat signals that a step is still making progress.
// Call this after each successful sub-operation (screenshot
// taken, LLM responded, action executed).
func (c *Controller) Heartbeat(
	phase, platform string, stepNum int,
) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := stepKey(phase, platform, stepNum)
	if info, ok := c.steps[key]; ok {
		info.LastBeat = time.Now()
	}
}

// CompleteStep marks a step as completed and removes it
// from active monitoring.
func (c *Controller) CompleteStep(
	phase, platform string, stepNum int,
) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := stepKey(phase, platform, stepNum)
	if info, ok := c.steps[key]; ok {
		info.State = StepCompleted
		delete(c.steps, key)
	}
}

// Start begins the background monitoring loop. Call
// Stop to terminate.
func (c *Controller) Start(ctx context.Context) {
	go c.monitorLoop(ctx)
}

// Stop terminates the monitoring loop.
func (c *Controller) Stop() {
	select {
	case <-c.done:
		// Already stopped.
	default:
		close(c.done)
	}
}

// monitorLoop runs until ctx is done or Stop is called.
func (c *Controller) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(c.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		case <-ticker.C:
			c.checkSteps()
		}
	}
}

// checkSteps iterates active steps and kills stale ones.
func (c *Controller) checkSteps() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key, info := range c.steps {
		if info.State != StepRunning {
			continue
		}

		stale := info.StaleDuration()
		runtime := info.Duration()

		if stale >= c.config.StaleThreshold {
			// Kill the step.
			info.State = StepKilled
			if info.Cancel != nil {
				info.Cancel()
			}
			c.kills[info.Phase]++

			evt := Event{
				Time:        time.Now(),
				Phase:       info.Phase,
				Platform:    info.Platform,
				StepNumber:  info.StepNumber,
				Action:      "kill",
				Reason:      "stale: no heartbeat",
				StaleFor:    stale,
				StepRuntime: runtime,
			}
			c.events = append(c.events, evt)
			fmt.Printf(
				"  [controller] KILL %s step %d on %s "+
					"(stale %s, runtime %s)\n",
				info.Phase, info.StepNumber,
				info.Platform,
				stale.Round(time.Second),
				runtime.Round(time.Second),
			)
			delete(c.steps, key)

		} else if stale >= c.config.WarnThreshold {
			evt := Event{
				Time:        time.Now(),
				Phase:       info.Phase,
				Platform:    info.Platform,
				StepNumber:  info.StepNumber,
				Action:      "warn",
				Reason:      "approaching stale threshold",
				StaleFor:    stale,
				StepRuntime: runtime,
			}
			c.events = append(c.events, evt)
			fmt.Printf(
				"  [controller] WARN %s step %d on %s "+
					"(stale %s/%s)\n",
				info.Phase, info.StepNumber,
				info.Platform,
				stale.Round(time.Second),
				c.config.StaleThreshold.Round(time.Second),
			)
		}
	}
}

// Events returns a copy of all controller events.
func (c *Controller) Events() []Event {
	c.mu.Lock()
	defer c.mu.Unlock()

	result := make([]Event, len(c.events))
	copy(result, c.events)
	return result
}

// KillCount returns how many steps were killed in a phase.
func (c *Controller) KillCount(phase string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.kills[phase]
}

// TotalKills returns the total number of killed steps.
func (c *Controller) TotalKills() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	total := 0
	for _, n := range c.kills {
		total += n
	}
	return total
}

// ShouldAbortPhase returns true if too many steps have
// been killed in the given phase, suggesting a systemic
// issue (e.g., vision server down).
func (c *Controller) ShouldAbortPhase(phase string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.kills[phase] >= c.config.MaxKillsPerPhase
}

// ActiveSteps returns the count of currently monitored
// steps.
func (c *Controller) ActiveSteps() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.steps)
}

// Summary returns a human-readable summary of controller
// activity for inclusion in pipeline reports.
func (c *Controller) Summary() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	kills := 0
	warns := 0
	for _, e := range c.events {
		switch e.Action {
		case "kill":
			kills++
		case "warn":
			warns++
		}
	}

	return fmt.Sprintf(
		"Process Controller: %d kills, %d warnings, "+
			"%d events total",
		kills, warns, len(c.events),
	)
}
