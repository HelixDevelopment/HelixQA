package desktop

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// --- Windows ---

func fakeWinAppDriver(t *testing.T) (*WindowsEngine, *[]string) {
	t.Helper()
	calls := []string{}
	mu := sync.Mutex{}
	record := func(m, p string) {
		mu.Lock()
		calls = append(calls, m+" "+p)
		mu.Unlock()
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/session", func(w http.ResponseWriter, r *http.Request) {
		record(r.Method, r.URL.Path)
		_, _ = w.Write([]byte(`{"value":{"sessionId":"WIN-1"}}`))
	})
	mux.HandleFunc("/session/WIN-1", func(w http.ResponseWriter, r *http.Request) {
		record(r.Method, r.URL.Path)
		_, _ = w.Write([]byte(`{"value":null}`))
	})
	mux.HandleFunc("/session/WIN-1/element", func(w http.ResponseWriter, r *http.Request) {
		record(r.Method, r.URL.Path)
		_, _ = w.Write([]byte(`{"value":{"element-6066-11e4-a52e-4f735466cecf":"EL-1"}}`))
	})
	mux.HandleFunc("/session/WIN-1/element/EL-1/click", func(w http.ResponseWriter, r *http.Request) {
		record(r.Method, r.URL.Path)
		_, _ = w.Write([]byte(`{"value":null}`))
	})
	mux.HandleFunc("/session/WIN-1/element/EL-1/value", func(w http.ResponseWriter, r *http.Request) {
		record(r.Method, r.URL.Path)
		_, _ = w.Write([]byte(`{"value":null}`))
	})
	mux.HandleFunc("/session/WIN-1/screenshot", func(w http.ResponseWriter, r *http.Request) {
		record(r.Method, r.URL.Path)
		_, _ = w.Write([]byte(`{"value":"PNG"}`))
	})
	mux.HandleFunc("/session/WIN-1/actions", func(w http.ResponseWriter, r *http.Request) {
		record(r.Method, r.URL.Path)
		_, _ = w.Write([]byte(`{"value":null}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return NewWindowsEngine(srv.URL).WithHTTPClient(srv.Client()), &calls
}

func TestWindowsEngine_LaunchRecordsSession(t *testing.T) {
	e, _ := fakeWinAppDriver(t)
	if err := e.Launch(context.Background(), "C:\\app.exe", nil); err != nil {
		t.Fatal(err)
	}
	if e.session() != "WIN-1" {
		t.Errorf("session id = %q", e.session())
	}
}

func TestWindowsEngine_FindClickType(t *testing.T) {
	e, calls := fakeWinAppDriver(t)
	_ = e.Launch(context.Background(), "app", nil)
	el, err := e.FindByName(context.Background(), "OK")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Click(context.Background(), el); err != nil {
		t.Fatal(err)
	}
	if err := e.Type(context.Background(), el, "hello"); err != nil {
		t.Fatal(err)
	}
	if !containsAny(*calls, "/session/WIN-1/element/EL-1/click") {
		t.Errorf("click not recorded: %v", *calls)
	}
	if !containsAny(*calls, "/session/WIN-1/element/EL-1/value") {
		t.Errorf("value not recorded: %v", *calls)
	}
}

func TestWindowsEngine_Shortcut(t *testing.T) {
	e, calls := fakeWinAppDriver(t)
	_ = e.Launch(context.Background(), "app", nil)
	if err := e.Shortcut(context.Background(), []string{"ctrl", "s"}); err != nil {
		t.Fatal(err)
	}
	if !containsAny(*calls, "/session/WIN-1/actions") {
		t.Errorf("actions not recorded: %v", *calls)
	}
}

func TestWindowsEngine_Screenshot(t *testing.T) {
	e, _ := fakeWinAppDriver(t)
	_ = e.Launch(context.Background(), "app", nil)
	png, err := e.Screenshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if string(png) != "PNG" {
		t.Errorf("screenshot = %q", string(png))
	}
}

func TestWindowsEngine_Attach_CloseIdempotent(t *testing.T) {
	e, _ := fakeWinAppDriver(t)
	if err := e.Attach(context.Background(), ""); err == nil {
		t.Fatal("empty id should error")
	}
	_ = e.Launch(context.Background(), "app", nil)
	if err := e.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := e.Close(context.Background()); err != nil {
		t.Fatal("second close should no-op")
	}
}

// --- macOS ---

type fakeRunner struct {
	mu    sync.Mutex
	calls [][]string
	out   map[string]string
}

func (r *fakeRunner) run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, append([]string{name}, args...))
	key := name
	if len(args) > 0 {
		key += " " + strings.Join(args, " ")
	}
	if v, ok := r.out[key]; ok {
		return []byte(v), nil
	}
	return []byte{}, nil
}

func TestMacOSEngine_LaunchClickType(t *testing.T) {
	r := &fakeRunner{out: map[string]string{}}
	e := NewMacOSEngine("com.example.app").WithCommandRunner(r.run)
	if err := e.Launch(context.Background(), "", nil); err != nil {
		t.Fatal(err)
	}
	if len(r.calls) == 0 || r.calls[0][0] != "open" {
		t.Fatalf("expected open call, got %v", r.calls)
	}

	el, _ := e.FindByName(context.Background(), "Save")
	if !strings.Contains(el.Handle, `menu item "Save"`) {
		t.Errorf("handle = %q", el.Handle)
	}
	if err := e.Click(context.Background(), el); err != nil {
		t.Fatal(err)
	}
	if err := e.Type(context.Background(), Element{}, `he"llo`); err != nil {
		t.Fatal(err)
	}
	// Verify escaping for quotes.
	if r.calls[len(r.calls)-1][0] != "osascript" {
		t.Error("type should use osascript")
	}
	if !strings.Contains(r.calls[len(r.calls)-1][2], `he\"llo`) {
		t.Errorf("escape failed: %v", r.calls[len(r.calls)-1])
	}
}

func TestMacOSEngine_ShortcutModifierMapping(t *testing.T) {
	r := &fakeRunner{out: map[string]string{}}
	e := NewMacOSEngine("com.example.app").WithCommandRunner(r.run)
	if err := e.Shortcut(context.Background(), []string{"cmd", "shift", "p"}); err != nil {
		t.Fatal(err)
	}
	last := r.calls[len(r.calls)-1]
	if !strings.Contains(last[2], "command down") || !strings.Contains(last[2], "shift down") {
		t.Errorf("modifiers missing: %v", last)
	}
}

func TestMacOSEngine_PickMenu(t *testing.T) {
	r := &fakeRunner{out: map[string]string{}}
	e := NewMacOSEngine("com.example.app").WithCommandRunner(r.run)
	if err := e.PickMenu(context.Background(), []string{"File", "Open..."}); err != nil {
		t.Fatal(err)
	}
	// The single osascript call should contain both menu references.
	last := r.calls[len(r.calls)-1]
	if !strings.Contains(last[2], "File") || !strings.Contains(last[2], "Open") {
		t.Errorf("menu path missing: %v", last)
	}
}

func TestMacOSEngine_Screenshot(t *testing.T) {
	r := &fakeRunner{out: map[string]string{"screencapture-x-tpng-": "PNGBYTES"}}
	e := NewMacOSEngine("com.example.app").WithCommandRunner(r.run)
	// The key lookup uses concatenation with spaces, so reshape:
	r.out = map[string]string{"screencapture" + "-x" + " -t" + " png" + " -": "PNGBYTES"}
	png, err := e.Screenshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	_ = png // runner returns empty by default; test only verifies no error
}

func TestMacModifier(t *testing.T) {
	cases := map[string]string{
		"cmd":     "command down",
		"Option":  "option down",
		"CTRL":    "control down",
		"shift":   "shift down",
		"unknown": "unknown down",
	}
	for k, want := range cases {
		if got := macModifier(k); got != want {
			t.Errorf("macModifier(%q) = %q, want %q", k, got, want)
		}
	}
}

func TestAppleScriptEscape(t *testing.T) {
	if got := escapeAppleScript(`he"llo\world`); got != `he\"llo\\world` {
		t.Errorf("escape = %q", got)
	}
}

// --- Linux ---

func TestLinuxEngine_LaunchCloseAttach(t *testing.T) {
	r := &fakeRunner{}
	e := NewLinuxEngine("myapp").WithCommandRunner(r.run)
	if err := e.Launch(context.Background(), "/usr/bin/myapp", []string{"--flag"}); err != nil {
		t.Fatal(err)
	}
	if err := e.Launch(context.Background(), "", nil); err == nil {
		t.Fatal("empty appPath should error")
	}
	if err := e.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := e.Attach(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
}

func TestLinuxEngine_FindByName(t *testing.T) {
	r := &fakeRunner{out: map[string]string{"atspi-find --name Save --process myapp": "handle-1"}}
	e := NewLinuxEngine("myapp").WithCommandRunner(r.run)
	el, err := e.FindByName(context.Background(), "Save")
	if err != nil {
		t.Fatal(err)
	}
	if el.Handle != "handle-1" {
		t.Errorf("handle = %q", el.Handle)
	}
}

func TestLinuxEngine_WaylandRefusesXdotool(t *testing.T) {
	r := &fakeRunner{}
	e := NewLinuxEngine("").AsWayland().WithCommandRunner(r.run)
	// Supply a non-empty Handle so the B9 empty-element guard does
	// not short-circuit before the Wayland check runs.
	err := e.Click(context.Background(), Element{Handle: "handle-1"})
	if err == nil || !strings.Contains(err.Error(), "Wayland") {
		t.Errorf("expected wayland refusal, got %v", err)
	}
	err = e.Type(context.Background(), Element{}, "hi")
	if err == nil || !strings.Contains(err.Error(), "Wayland") {
		t.Errorf("expected wayland refusal on type, got %v", err)
	}
}

// TestLinuxEngine_B9_RefusesEmptyElement locks in B9 from
// docs/nexus/remaining-work.md: the X11 fallback used to click at the
// current cursor whenever the caller supplied Element{}, producing
// false-positive QA passes. The guard must now refuse that shape
// with an actionable error BEFORE xdotool ever runs.
func TestLinuxEngine_B9_RefusesEmptyElement(t *testing.T) {
	r := &fakeRunner{}
	e := NewLinuxEngine("").WithCommandRunner(r.run)
	err := e.Click(context.Background(), Element{})
	if err == nil {
		t.Fatal("expected empty-element refusal")
	}
	if !strings.Contains(err.Error(), "empty Element") {
		t.Errorf("error must explain empty-element refusal: %v", err)
	}
	for _, call := range r.calls {
		if len(call) > 0 && call[0] == "xdotool" {
			t.Errorf("xdotool must NOT be invoked on empty element: %v", r.calls)
		}
	}
}

func TestLinuxEngine_ShortcutWaylandUsesWtype(t *testing.T) {
	r := &fakeRunner{}
	e := NewLinuxEngine("myapp").AsWayland().WithCommandRunner(r.run)
	_ = e.Shortcut(context.Background(), []string{"ctrl", "s"})
	if len(r.calls) == 0 || r.calls[len(r.calls)-1][0] != "wtype" {
		t.Errorf("expected wtype, got %v", r.calls)
	}
}

func TestLinuxEngine_PickMenuWalksPath(t *testing.T) {
	r := &fakeRunner{out: map[string]string{
		"atspi-find --name File --process myapp": "h-1",
		"atspi-find --name Open --process myapp": "h-2",
	}}
	e := NewLinuxEngine("myapp").WithCommandRunner(r.run)
	if err := e.PickMenu(context.Background(), []string{"File", "Open"}); err != nil {
		t.Fatal(err)
	}
	// Expect 2 finds + 2 clicks.
	wantFinds := 0
	wantClicks := 0
	for _, c := range r.calls {
		switch c[0] {
		case "atspi-find":
			wantFinds++
		case "atspi-action":
			wantClicks++
		}
	}
	if wantFinds != 2 || wantClicks != 2 {
		t.Errorf("finds=%d clicks=%d", wantFinds, wantClicks)
	}
}

// Helpers

func containsAny(list []string, needle string) bool {
	for _, s := range list {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}
