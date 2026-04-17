package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Platform names a target surface.
type Platform string

const (
	PlatformWeb     Platform = "web"
	PlatformMobile  Platform = "mobile"
	PlatformDesktop Platform = "desktop"
	PlatformAPI     Platform = "api"
)

// StepFn runs one step of a cross-platform flow, given the shared
// ExecutionContext. Returning an error aborts the whole flow.
type StepFn func(ctx context.Context, ec *ExecutionContext) error

// Step is a single flow unit targeted at a particular platform.
type Step struct {
	Name     string
	Platform Platform
	Action   StepFn
	Verify   StepFn
	Timeout  time.Duration
}

// ExecutionContext is the shared state that flows through every Step.
// It is concurrency-safe so Steps that fan out still see a consistent
// Data map.
type ExecutionContext struct {
	mu   sync.RWMutex
	data map[string]any

	Evidence *Evidence
}

// NewExecutionContext returns an empty context with an evidence vault.
func NewExecutionContext() *ExecutionContext {
	return &ExecutionContext{
		data:     make(map[string]any),
		Evidence: NewEvidence(),
	}
}

// Set stores a key/value pair.
func (e *ExecutionContext) Set(key string, value any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.data[key] = value
}

// Get returns the value for key and whether it was present.
func (e *ExecutionContext) Get(key string) (any, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	v, ok := e.data[key]
	return v, ok
}

// Snapshot returns a copy of the current state.
func (e *ExecutionContext) Snapshot() map[string]any {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make(map[string]any, len(e.data))
	for k, v := range e.data {
		out[k] = v
	}
	return out
}

// Flow is an ordered sequence of Steps.
type Flow struct {
	Name  string
	Steps []Step
}

// Run executes the flow sequentially. The first error aborts the run
// and is returned with the step index that failed.
func (f *Flow) Run(ctx context.Context, ec *ExecutionContext) error {
	if ec == nil {
		return errors.New("orchestrator: nil ExecutionContext")
	}
	for i, step := range f.Steps {
		stepCtx := ctx
		if step.Timeout > 0 {
			var cancel context.CancelFunc
			stepCtx, cancel = context.WithTimeout(ctx, step.Timeout)
			defer cancel()
		}
		if step.Action == nil {
			return fmt.Errorf("step %d (%q): no action", i+1, step.Name)
		}
		if err := step.Action(stepCtx, ec); err != nil {
			return fmt.Errorf("step %d (%q) on %s: %w", i+1, step.Name, step.Platform, err)
		}
		if step.Verify != nil {
			if err := step.Verify(stepCtx, ec); err != nil {
				return fmt.Errorf("step %d (%q) verify on %s: %w", i+1, step.Name, step.Platform, err)
			}
		}
	}
	return nil
}
