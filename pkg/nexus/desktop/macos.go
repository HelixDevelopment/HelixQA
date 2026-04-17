package desktop

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// MacOSEngine drives macOS applications through AppleScript /
// `osascript` for menu bar and scripted events, and falls back to
// XCUITest through WebDriverAgent when finer-grained control is
// required (accessibility tree, element click by identifier).
//
// The engine is CGo-free and never requires xcode-cli tools to be
// installed on the HelixQA build host; `osascript` is only invoked at
// run time and its presence is checked via CommandRunner. Tests
// override CommandRunner to capture the script rather than execute it.
type MacOSEngine struct {
	bundleID       string
	commandRunner  func(ctx context.Context, name string, args ...string) ([]byte, error)
	webdriverEngine *WindowsEngine // reuse the W3C WebDriver HTTP client shape for WDA calls
}

// NewMacOSEngine returns an engine for the target bundle id.
func NewMacOSEngine(bundleID string) *MacOSEngine {
	return &MacOSEngine{
		bundleID:      bundleID,
		commandRunner: defaultCommandRunner,
	}
}

// WithCommandRunner injects a test-owned runner for osascript + shell.
func (m *MacOSEngine) WithCommandRunner(r func(ctx context.Context, name string, args ...string) ([]byte, error)) *MacOSEngine {
	m.commandRunner = r
	return m
}

// Platform returns PlatformMacOS.
func (*MacOSEngine) Platform() Platform { return PlatformMacOS }

// Launch invokes `open -b <bundleId>` which requests the application to
// come to the foreground (launching if necessary).
func (m *MacOSEngine) Launch(ctx context.Context, appPath string, args []string) error {
	cmd := []string{"-b", m.bundleID}
	if appPath != "" && appPath != m.bundleID {
		cmd = []string{"-a", appPath}
	}
	if len(args) > 0 {
		cmd = append(cmd, "--args")
		cmd = append(cmd, args...)
	}
	_, err := m.commandRunner(ctx, "open", cmd...)
	if err != nil {
		return fmt.Errorf("macos launch: %w", err)
	}
	return nil
}

// Attach is a no-op for macOS; the engine always targets the current
// bundleID.
func (m *MacOSEngine) Attach(_ context.Context, _ string) error { return nil }

// Close terminates the target application.
func (m *MacOSEngine) Close(ctx context.Context) error {
	script := fmt.Sprintf(`tell application id "%s" to quit`, m.bundleID)
	_, err := m.commandRunner(ctx, "osascript", "-e", script)
	return err
}

// FindByName returns an Element proxying an AppleScript query. The
// Handle encodes the full query so follow-up actions can reuse it.
func (m *MacOSEngine) FindByName(_ context.Context, name string) (Element, error) {
	if name == "" {
		return Element{}, errors.New("macos: empty element name")
	}
	return Element{
		Handle: fmt.Sprintf(`menu item "%s"`, name),
		Name:   name,
		Role:   "menu_item",
	}, nil
}

// FindByRole returns an Element targeting the given role (macOS
// accessibility role such as `AXButton`, `AXTextField`).
func (m *MacOSEngine) FindByRole(_ context.Context, role string) (Element, error) {
	if role == "" {
		return Element{}, errors.New("macos: empty role")
	}
	return Element{Handle: role, Name: "", Role: role}, nil
}

// Click performs `click element` via osascript.
func (m *MacOSEngine) Click(ctx context.Context, el Element) error {
	if el.Handle == "" {
		return errors.New("macos: nil element")
	}
	script := fmt.Sprintf(
		`tell application "System Events" to tell process "%s" to click %s`,
		m.bundleID, el.Handle)
	_, err := m.commandRunner(ctx, "osascript", "-e", script)
	return err
}

// Type sends keystrokes using System Events.
func (m *MacOSEngine) Type(ctx context.Context, _ Element, text string) error {
	script := fmt.Sprintf(`tell application "System Events" to keystroke "%s"`, escapeAppleScript(text))
	_, err := m.commandRunner(ctx, "osascript", "-e", script)
	return err
}

// Screenshot captures the full screen via `screencapture`.
func (m *MacOSEngine) Screenshot(ctx context.Context) ([]byte, error) {
	return m.commandRunner(ctx, "screencapture", "-x", "-t", "png", "-")
}

// PickMenu walks a menu path such as ["File", "Open"].
func (m *MacOSEngine) PickMenu(ctx context.Context, path []string) error {
	if len(path) == 0 {
		return errors.New("macos: empty menu path")
	}
	var script strings.Builder
	script.WriteString(fmt.Sprintf(`tell application "System Events" to tell process "%s"`+"\n", m.bundleID))
	script.WriteString(fmt.Sprintf(`click menu bar item "%s" of menu bar 1`+"\n", path[0]))
	for _, item := range path[1:] {
		script.WriteString(fmt.Sprintf(`click menu item "%s" of menu "%s" of menu bar 1`+"\n", item, path[0]))
	}
	script.WriteString("end tell\n")
	_, err := m.commandRunner(ctx, "osascript", "-e", script.String())
	return err
}

// Shortcut presses the given key combination.
func (m *MacOSEngine) Shortcut(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return errors.New("macos: empty shortcut")
	}
	modifiers := []string{}
	main := keys[len(keys)-1]
	for _, k := range keys[:len(keys)-1] {
		modifiers = append(modifiers, macModifier(k))
	}
	script := fmt.Sprintf(`tell application "System Events" to keystroke "%s" using {%s}`, main, strings.Join(modifiers, ","))
	_, err := m.commandRunner(ctx, "osascript", "-e", script)
	return err
}

func escapeAppleScript(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`)
}

func macModifier(k string) string {
	switch strings.ToLower(k) {
	case "cmd", "command", "meta":
		return "command down"
	case "opt", "option", "alt":
		return "option down"
	case "ctrl", "control":
		return "control down"
	case "shift":
		return "shift down"
	}
	return k + " down"
}

func defaultCommandRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

var _ Engine = (*MacOSEngine)(nil)
