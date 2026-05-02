package screenshot

import "context"

// Engine is the platform-specific capture implementation.
type Engine interface {
	// Capture returns raw screenshot bytes and metadata.
	Capture(ctx context.Context, opts CaptureOptions) (*Result, error)
	// Supported returns true if the engine is wired and tools are on PATH.
	Supported(ctx context.Context) bool
	// Name returns a human-readable identifier, e.g. "web-playwright".
	Name() string
}
