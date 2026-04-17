package desktop

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// LinuxEngine drives Linux GUI applications via AT-SPI over DBus as the
// primary path (rich accessibility tree) and X11 / xdotool as a
// fallback (coordinate-driven input). Wayland sessions fall back to
// AT-SPI exclusively because xdotool does not work under Wayland.
//
// The engine is command-runner-based so tests can inject a capture
// function rather than spawning real processes.
type LinuxEngine struct {
	bundleID      string
	display       string
	waylandNative bool
	commandRunner func(ctx context.Context, name string, args ...string) ([]byte, error)
}

// NewLinuxEngine returns an engine bound to the given AT-SPI-friendly
// bundle identifier (process name, typically).
func NewLinuxEngine(bundleID string) *LinuxEngine {
	return &LinuxEngine{
		bundleID:      bundleID,
		commandRunner: defaultCommandRunner,
	}
}

// WithDisplay targets a specific X11 DISPLAY (":0", ":1", ...).
func (l *LinuxEngine) WithDisplay(display string) *LinuxEngine {
	l.display = display
	return l
}

// AsWayland marks the engine as running under Wayland, disabling the
// xdotool fallback path.
func (l *LinuxEngine) AsWayland() *LinuxEngine {
	l.waylandNative = true
	return l
}

// WithCommandRunner injects a test command runner.
func (l *LinuxEngine) WithCommandRunner(r func(ctx context.Context, name string, args ...string) ([]byte, error)) *LinuxEngine {
	l.commandRunner = r
	return l
}

// Platform returns PlatformLinux.
func (*LinuxEngine) Platform() Platform { return PlatformLinux }

// Launch runs appPath (executable or .desktop file).
func (l *LinuxEngine) Launch(ctx context.Context, appPath string, args []string) error {
	if appPath == "" {
		return errors.New("linux: empty app path")
	}
	cmd := append([]string{}, args...)
	full := append([]string{appPath}, cmd...)
	_, err := l.commandRunner(ctx, full[0], full[1:]...)
	return err
}

// Attach is a no-op for Linux; the engine targets bundleID by name.
func (l *LinuxEngine) Attach(_ context.Context, _ string) error { return nil }

// Close sends a clean quit via `kill -SIGTERM` on the target process.
func (l *LinuxEngine) Close(ctx context.Context) error {
	_, err := l.commandRunner(ctx, "pkill", "-f", l.bundleID)
	return err
}

// FindByName locates a node whose accessibility name matches via
// `busctl` / `at-spi2-core`. The implementation shells out to a helper
// so the test suite can replace it.
func (l *LinuxEngine) FindByName(ctx context.Context, name string) (Element, error) {
	out, err := l.commandRunner(ctx, "atspi-find", "--name", name, "--process", l.bundleID)
	if err != nil {
		return Element{}, fmt.Errorf("linux find-by-name: %w", err)
	}
	handle := strings.TrimSpace(string(out))
	if handle == "" {
		return Element{}, fmt.Errorf("linux: no element named %q", name)
	}
	return Element{Handle: handle, Name: name}, nil
}

// FindByRole locates a node by accessibility role.
func (l *LinuxEngine) FindByRole(ctx context.Context, role string) (Element, error) {
	out, err := l.commandRunner(ctx, "atspi-find", "--role", role, "--process", l.bundleID)
	if err != nil {
		return Element{}, fmt.Errorf("linux find-by-role: %w", err)
	}
	handle := strings.TrimSpace(string(out))
	if handle == "" {
		return Element{}, fmt.Errorf("linux: no element role=%q", role)
	}
	return Element{Handle: handle, Role: role}, nil
}

// Click uses AT-SPI's default action on the handle; falls back to
// `xdotool` on X11 if the bundleID is unset.
func (l *LinuxEngine) Click(ctx context.Context, el Element) error {
	if el.Handle != "" && l.bundleID != "" {
		_, err := l.commandRunner(ctx, "atspi-action", "--handle", el.Handle, "--action", "click")
		return err
	}
	if l.waylandNative {
		return errors.New("linux: xdotool fallback unavailable under Wayland")
	}
	_, err := l.commandRunner(ctx, "xdotool", "click", "1")
	return err
}

// Type sends text via `atspi-type` or xdotool fallback.
func (l *LinuxEngine) Type(ctx context.Context, _ Element, text string) error {
	if l.bundleID != "" {
		_, err := l.commandRunner(ctx, "atspi-type", "--process", l.bundleID, "--text", text)
		return err
	}
	if l.waylandNative {
		return errors.New("linux: xdotool fallback unavailable under Wayland")
	}
	_, err := l.commandRunner(ctx, "xdotool", "type", "--delay", "10", text)
	return err
}

// Screenshot invokes `gnome-screenshot` (works on both X11 and Wayland
// with native support), falling back to `import` from ImageMagick when
// gnome-screenshot is unavailable.
func (l *LinuxEngine) Screenshot(ctx context.Context) ([]byte, error) {
	if out, err := l.commandRunner(ctx, "gnome-screenshot", "-f", "/dev/stdout"); err == nil {
		return out, nil
	}
	return l.commandRunner(ctx, "import", "-window", "root", "png:-")
}

// PickMenu navigates a menu path.
func (l *LinuxEngine) PickMenu(ctx context.Context, path []string) error {
	for _, item := range path {
		el, err := l.FindByName(ctx, item)
		if err != nil {
			return err
		}
		if err := l.Click(ctx, el); err != nil {
			return err
		}
	}
	return nil
}

// Shortcut presses a keyboard shortcut via xdotool on X11 or a Wayland
// tool when available.
func (l *LinuxEngine) Shortcut(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return errors.New("linux: empty shortcut")
	}
	combo := strings.Join(keys, "+")
	if l.waylandNative {
		_, err := l.commandRunner(ctx, "wtype", "--key", combo)
		return err
	}
	_, err := l.commandRunner(ctx, "xdotool", "key", combo)
	return err
}

// xdotoolAvailable reports whether xdotool is in PATH. Useful for the
// engine's own diagnostics.
func xdotoolAvailable() bool {
	_, err := exec.LookPath("xdotool")
	return err == nil
}

var _ Engine = (*LinuxEngine)(nil)
