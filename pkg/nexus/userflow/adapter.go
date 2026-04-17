// Package userflow adapts the Nexus browser Engine to the
// digital.vasic.challenges/pkg/userflow.BrowserAdapter contract so
// existing HelixQA challenge banks can target Nexus without changes.
//
// The adapter preserves the existing BrowserAdapter surface
// (selector-based Click/Fill/GetText/Screenshot) while delegating every
// action to the Nexus Engine. Element references produced by a Nexus
// Snapshot are treated as ordinary selectors; raw CSS selectors also
// flow through untouched because the underlying chromedp / go-rod
// drivers already accept both.
package userflow

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	uf "digital.vasic.challenges/pkg/userflow"

	"digital.vasic.helixqa/pkg/nexus"
	"digital.vasic.helixqa/pkg/nexus/browser"
)

// NexusBrowserAdapter wraps a *browser.Engine and satisfies
// uf.BrowserAdapter. Construct one per test session.
type NexusBrowserAdapter struct {
	engine  *browser.Engine
	session nexus.Session

	mu     sync.Mutex
	closed bool
}

// NewNexusBrowserAdapter returns an adapter bound to the given Engine.
// The adapter opens a new nexus.Session on Initialize and closes it on
// Close.
func NewNexusBrowserAdapter(engine *browser.Engine) *NexusBrowserAdapter {
	return &NexusBrowserAdapter{engine: engine}
}

// Initialize opens a Nexus session from the supplied BrowserConfig.
// BrowserConfig's Headless and Timeout fields map straight through;
// other fields are ignored because Nexus runs with adapter-specific
// defaults (CDP port, allowlist) that are set at Engine construction.
func (a *NexusBrowserAdapter) Initialize(ctx context.Context, cfg uf.BrowserConfig) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.engine == nil {
		return errors.New("nexus browser adapter: Engine is nil")
	}
	if a.session != nil {
		return errors.New("nexus browser adapter: already initialized")
	}
	opts := nexus.SessionOptions{
		Headless:   cfg.Headless,
		WindowSize: cfg.WindowSize,
	}
	sess, err := a.engine.Open(ctx, opts)
	if err != nil {
		return fmt.Errorf("nexus: open session: %w", err)
	}
	a.session = sess
	return nil
}

// Navigate delegates to the Engine with allowlist + scheme checks.
func (a *NexusBrowserAdapter) Navigate(ctx context.Context, url string) error {
	if err := a.requireSession(); err != nil {
		return err
	}
	return a.engine.Navigate(ctx, a.session, url)
}

// Click resolves the selector (or Nexus ref) and clicks.
func (a *NexusBrowserAdapter) Click(ctx context.Context, selector string) error {
	if err := a.requireSession(); err != nil {
		return err
	}
	return a.engine.Do(ctx, a.session, nexus.Action{Kind: "click", Target: selector})
}

// Fill types a value into the target.
func (a *NexusBrowserAdapter) Fill(ctx context.Context, selector, value string) error {
	if err := a.requireSession(); err != nil {
		return err
	}
	return a.engine.Do(ctx, a.session, nexus.Action{
		Kind: "type", Target: selector, Text: value,
	})
}

// SelectOption is not yet supported by the Phase 1 Engine; it returns
// a clear error so callers can fall back to Fill where possible.
func (a *NexusBrowserAdapter) SelectOption(ctx context.Context, selector, value string) error {
	return errors.New("nexus browser adapter: SelectOption lands in Phase 1 P1-05 (pending)")
}

// IsVisible takes a snapshot and scans for the selector. It is a best-
// effort check; callers that need DOM-accurate visibility should use
// WaitForSelector with a short timeout.
func (a *NexusBrowserAdapter) IsVisible(ctx context.Context, selector string) (bool, error) {
	if err := a.requireSession(); err != nil {
		return false, err
	}
	snap, err := a.engine.Snapshot(ctx, a.session)
	if err != nil {
		return false, err
	}
	for _, el := range snap.Elements {
		if string(el.Ref) == selector || el.Selector == selector {
			return true, nil
		}
	}
	return false, nil
}

// WaitForSelector polls snapshots until the selector appears or the
// timeout elapses.
func (a *NexusBrowserAdapter) WaitForSelector(ctx context.Context, selector string, timeout time.Duration) error {
	if err := a.requireSession(); err != nil {
		return err
	}
	deadline := time.Now().Add(timeout)
	for {
		visible, err := a.IsVisible(ctx, selector)
		if err == nil && visible {
			return nil
		}
		if time.Now().After(deadline) {
			if err != nil {
				return fmt.Errorf("wait_for_selector %q: %w", selector, err)
			}
			return fmt.Errorf("wait_for_selector %q: timeout after %s", selector, timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// GetText fetches the snapshot and returns the Name of the first
// element whose Ref or Selector matches.
func (a *NexusBrowserAdapter) GetText(ctx context.Context, selector string) (string, error) {
	if err := a.requireSession(); err != nil {
		return "", err
	}
	snap, err := a.engine.Snapshot(ctx, a.session)
	if err != nil {
		return "", err
	}
	for _, el := range snap.Elements {
		if string(el.Ref) == selector || el.Selector == selector {
			return el.Name, nil
		}
	}
	return "", fmt.Errorf("selector %q not found in current snapshot", selector)
}

// GetAttribute is not yet supported by the Phase 1 Engine; it returns
// a clear error so the caller can choose a different action.
func (a *NexusBrowserAdapter) GetAttribute(ctx context.Context, selector, attr string) (string, error) {
	return "", errors.New("nexus browser adapter: GetAttribute lands in Phase 1 P1-05 (pending)")
}

// Screenshot delegates to the Engine.
func (a *NexusBrowserAdapter) Screenshot(ctx context.Context) ([]byte, error) {
	if err := a.requireSession(); err != nil {
		return nil, err
	}
	return a.engine.Screenshot(ctx, a.session)
}

// EvaluateJS is intentionally refused under the Phase 1 security
// posture (the Engine does not expose inline-script execution). Callers
// should use Snapshot + Do to drive the page.
func (a *NexusBrowserAdapter) EvaluateJS(ctx context.Context, script string) (string, error) {
	return "", errors.New("nexus browser adapter: arbitrary JS evaluation is disabled by policy")
}

// NetworkIntercept is not yet supported; it returns a clear error.
func (a *NexusBrowserAdapter) NetworkIntercept(ctx context.Context, pattern string, handler func(req *uf.InterceptedRequest)) error {
	return errors.New("nexus browser adapter: NetworkIntercept lands in Phase 5 (observability)")
}

// Close releases the underlying Nexus session. Safe to call multiple
// times.
func (a *NexusBrowserAdapter) Close(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed || a.session == nil {
		a.closed = true
		return nil
	}
	err := a.session.Close()
	a.session = nil
	a.closed = true
	return err
}

// Available reports whether the adapter is ready to run. It is true
// after a successful Initialize and false after Close or before
// Initialize.
func (a *NexusBrowserAdapter) Available(_ context.Context) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.session != nil && !a.closed
}

func (a *NexusBrowserAdapter) requireSession() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.session == nil {
		return errors.New("nexus browser adapter: call Initialize first")
	}
	if a.closed {
		return errors.New("nexus browser adapter: already closed")
	}
	return nil
}

var _ uf.BrowserAdapter = (*NexusBrowserAdapter)(nil)
