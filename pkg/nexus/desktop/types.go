package desktop

import "context"

// Platform names the target desktop OS.
type Platform string

const (
	PlatformWindows Platform = "windows"
	PlatformMacOS   Platform = "macos"
	PlatformLinux   Platform = "linux"
)

// Element is a desktop-surface UI element identified by a driver-
// specific handle plus a human-friendly name.
type Element struct {
	Handle string
	Name   string
	Role   string
	Bounds Rect
}

// Rect is a pixel rectangle.
type Rect struct {
	X, Y, W, H int
}

// Engine is the surface every desktop driver implements. Each method
// targets the current foreground app unless a specific session has been
// opened via Attach.
type Engine interface {
	Platform() Platform
	Launch(ctx context.Context, appPath string, args []string) error
	Attach(ctx context.Context, identifier string) error
	Close(ctx context.Context) error

	FindByName(ctx context.Context, name string) (Element, error)
	FindByRole(ctx context.Context, role string) (Element, error)

	Click(ctx context.Context, el Element) error
	Type(ctx context.Context, el Element, text string) error
	Screenshot(ctx context.Context) ([]byte, error)

	PickMenu(ctx context.Context, path []string) error
	Shortcut(ctx context.Context, keys []string) error
}
