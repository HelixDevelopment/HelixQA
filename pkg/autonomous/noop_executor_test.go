// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// noopExecutor and NoopExecutorFactory live in a *_test.go file so
// they are compiled ONLY by `go test` and NEVER by `go build` of any
// production binary. They satisfy navigator.ActionExecutor by returning
// nil/empty for every UI action, which is appropriate for unit tests
// where the test asserts orchestration logic without driving a real
// device — but would be a §11.4 PASS-bluff if reached from production
// code (any autonomous QA session that ended up with this executor
// would silently "succeed" every UI action without device interaction).
//
// Production code MUST use DefaultExecutorFactory
// (executor_factory.go) which dispatches to a real platform executor
// (ADB / Playwright / X11) or returns an explicit unsupported-platform
// error.

package autonomous

import (
	"context"

	"digital.vasic.helixqa/pkg/navigator"
)

// noopExecutor is a no-op ActionExecutor used only by tests.
type noopExecutor struct{}

func (n *noopExecutor) Click(_ context.Context, _, _ int) error { return nil }
func (n *noopExecutor) Type(_ context.Context, _ string) error  { return nil }
func (n *noopExecutor) Clear(_ context.Context) error           { return nil }
func (n *noopExecutor) Scroll(_ context.Context, _ string, _ int) error {
	return nil
}
func (n *noopExecutor) LongPress(_ context.Context, _, _ int) error { return nil }
func (n *noopExecutor) Swipe(_ context.Context, _, _, _, _ int) error {
	return nil
}
func (n *noopExecutor) KeyPress(_ context.Context, _ string) error { return nil }
func (n *noopExecutor) Back(_ context.Context) error               { return nil }
func (n *noopExecutor) Home(_ context.Context) error               { return nil }
func (n *noopExecutor) Screenshot(_ context.Context) ([]byte, error) {
	return []byte("mock-screenshot"), nil
}

// NoopExecutorFactory always returns a noopExecutor — test-only.
type NoopExecutorFactory struct{}

// Create returns a noopExecutor regardless of platform.
func (f *NoopExecutorFactory) Create(
	_ string,
) (navigator.ActionExecutor, error) {
	return &noopExecutor{}, nil
}
