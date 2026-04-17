package nexus

import (
	"context"
	"time"
)

// Platform enumerates the surfaces a Nexus driver can target.
type Platform string

const (
	PlatformWebChromedp     Platform = "web-chromedp"
	PlatformWebRod          Platform = "web-rod"
	PlatformWebPlaywright   Platform = "web-playwright"
	PlatformAndroidAppium   Platform = "android-appium"
	PlatformAndroidTVAppium Platform = "androidtv-appium"
	PlatformIOSAppium       Platform = "ios-appium"
	PlatformDesktopWindows  Platform = "desktop-windows"
	PlatformDesktopMacOS    Platform = "desktop-macos"
	PlatformDesktopLinux    Platform = "desktop-linux"
)

// Session represents a single, isolated automation session bound to a
// particular Platform. It is the unit of acquire / release from a pool
// and the unit of recording for evidence.
type Session interface {
	// ID is the stable identifier written into the helixqa_*_sessions tables.
	ID() string

	// Platform reports which adapter produced this session.
	Platform() Platform

	// Close releases the underlying resources (browser process, device
	// connection, desktop driver session) and cancels any in-flight work.
	Close() error
}

// ElementRef is an OpenClaw-style stable reference (for example e12, e23)
// that resolves to a concrete element inside the target UI tree. The ref
// remains valid as long as the tree used to produce it has not changed
// beyond recognition. Callers should take a fresh Snapshot whenever a
// previously valid ref starts to miss.
type ElementRef string

// Action is the executable intent the AI navigator or a test bank emits.
// Driver adapters translate the Action to platform-specific operations.
type Action struct {
	// Kind names the action: click, type, scroll, drag, tap, swipe, key,
	// wait_for, screenshot, pdf, tab_open, tab_close, menu_pick, and so on.
	Kind string

	// Target is either an ElementRef (preferred) or a human description
	// that a self-healer can fall back to.
	Target string

	// Text carries user-typed text for "type" actions.
	Text string

	// X, Y are raw coordinates used only when Target is empty and the
	// caller has explicit pixel intent (rare; prefer refs).
	X, Y int

	// Params carry adapter-specific extras (for example swipe velocity).
	Params map[string]any

	// Timeout overrides the driver default for this single action.
	Timeout time.Duration
}

// Snapshot is the platform-neutral representation of the UI at a moment
// in time. It always carries a visual frame (PNG bytes) plus a structured
// tree that a language model can reason over.
type Snapshot struct {
	// CapturedAt is the wall-clock moment the snapshot was taken.
	CapturedAt time.Time

	// Frame is the visual frame, always PNG-encoded.
	Frame []byte

	// Tree is an adapter-specific structured tree. For browser sessions
	// it carries ARIA roles and reference labels; for mobile it carries
	// the AccessibilityService / UIA / XCUITest hierarchy.
	Tree string

	// Elements lists every interactable element discovered during the
	// snapshot, keyed by stable ElementRef.
	Elements []Element
}

// Element is a single interactable node captured in a Snapshot.
type Element struct {
	Ref         ElementRef
	Role        string
	Name        string
	Description string
	Bounds      Rect
	Selector    string // CSS / XPath / query the driver uses to resolve Ref
}

// Rect is a pixel rectangle in the frame coordinate space.
type Rect struct {
	X, Y, Width, Height int
}

// Adapter is the common surface every Nexus driver implements. Concrete
// adapters live under pkg/nexus/<platform>/.
type Adapter interface {
	// Open starts a new Session on this Adapter's Platform.
	Open(ctx context.Context, opts SessionOptions) (Session, error)

	// Navigate loads a URL, launches a binary, or brings an app into the
	// foreground. The exact semantics depend on the platform, but the
	// signature is uniform so the orchestrator can reason about flows.
	Navigate(ctx context.Context, s Session, target string) error

	// Snapshot captures the current UI state for LLM analysis.
	Snapshot(ctx context.Context, s Session) (*Snapshot, error)

	// Do runs an Action against the session. The returned error, if any,
	// is already passed through ToAIFriendlyError before being wrapped.
	Do(ctx context.Context, s Session, a Action) error

	// Screenshot captures a PNG frame without the full tree. Cheaper than
	// Snapshot when an LLM only needs the pixels (for example to compare
	// against a baseline).
	Screenshot(ctx context.Context, s Session) ([]byte, error)
}

// SessionOptions controls the behaviour of a new Session. Each adapter
// reads the fields relevant to it; unrecognised fields are ignored.
type SessionOptions struct {
	Headless    bool
	WindowSize  [2]int
	UserDataDir string
	DeviceName  string
	BundleID    string
	AppPath     string
	Timeout     time.Duration
	Extra       map[string]string
}
