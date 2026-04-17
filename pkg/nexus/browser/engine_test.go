package browser

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"

	"digital.vasic.helixqa/pkg/nexus"
)

// mockDriver is a Driver that records calls and returns canned values.
type mockDriver struct {
	kind    EngineType
	open    func(ctx context.Context, cfg Config) (SessionHandle, error)
	openErr error
}

func (m *mockDriver) Kind() EngineType { return m.kind }
func (m *mockDriver) Open(ctx context.Context, cfg Config) (SessionHandle, error) {
	if m.openErr != nil {
		return nil, m.openErr
	}
	if m.open != nil {
		return m.open(ctx, cfg)
	}
	return &mockHandle{}, nil
}

type mockHandle struct {
	mu         sync.Mutex
	closed     bool
	navigated  string
	lastClick  nexus.ElementRef
	lastType   nexus.ElementRef
	lastText   string
	scrollDX   int
	scrollDY   int
	shotCount  int
	failAction error
}

func (h *mockHandle) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	return nil
}
func (h *mockHandle) Navigate(_ context.Context, url string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.failAction != nil {
		return h.failAction
	}
	h.navigated = url
	return nil
}
func (h *mockHandle) Snapshot(_ context.Context) (*nexus.Snapshot, error) {
	return &nexus.Snapshot{Tree: "<html/>", Elements: []nexus.Element{{Ref: "e1"}}}, nil
}
func (h *mockHandle) Click(_ context.Context, ref nexus.ElementRef) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastClick = ref
	return nil
}
func (h *mockHandle) Type(_ context.Context, ref nexus.ElementRef, text string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastType, h.lastText = ref, text
	return nil
}
func (h *mockHandle) Screenshot(_ context.Context) ([]byte, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.shotCount++
	return []byte("png"), nil
}
func (h *mockHandle) Scroll(_ context.Context, dx, dy int) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.scrollDX, h.scrollDY = dx, dy
	return nil
}

func TestNewEngine_RejectsNilDriver(t *testing.T) {
	if _, err := NewEngine(nil, Config{Engine: EngineChromedp}); err == nil {
		t.Fatal("expected error for nil driver")
	}
}

func TestNewEngine_RejectsKindMismatch(t *testing.T) {
	d := &mockDriver{kind: EngineRod}
	if _, err := NewEngine(d, Config{Engine: EngineChromedp}); err == nil {
		t.Fatal("expected error when driver kind disagrees with config")
	}
}

func TestNewEngine_FillsDefaults(t *testing.T) {
	d := &mockDriver{kind: EngineChromedp}
	e, err := NewEngine(d, Config{})
	if err != nil {
		t.Fatal(err)
	}
	if e.cfg.Engine != EngineChromedp {
		t.Errorf("default engine not filled: %s", e.cfg.Engine)
	}
	if e.cfg.MaxBodyBytes != 32<<20 {
		t.Errorf("default MaxBodyBytes not set: %d", e.cfg.MaxBodyBytes)
	}
}

func TestEngine_OpenIncrementsSessionCount(t *testing.T) {
	d := &mockDriver{kind: EngineChromedp}
	e, _ := NewEngine(d, Config{Engine: EngineChromedp})
	s1, err := e.Open(context.Background(), nexus.SessionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	s2, _ := e.Open(context.Background(), nexus.SessionOptions{})
	if got := e.ActiveSessions(); got != 2 {
		t.Errorf("ActiveSessions = %d, want 2", got)
	}
	_ = s1.Close()
	_ = s2.Close()
	if got := e.ActiveSessions(); got != 0 {
		t.Errorf("after Close, ActiveSessions = %d, want 0", got)
	}
}

func TestEngine_OpenTranslatesDriverError(t *testing.T) {
	d := &mockDriver{kind: EngineChromedp, openErr: errors.New("no such element")}
	e, _ := NewEngine(d, Config{Engine: EngineChromedp})
	_, err := e.Open(context.Background(), nexus.SessionOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsIgnoreCase(err.Error(), "not on the page") {
		t.Errorf("error should be translated for AI, got %q", err.Error())
	}
}

func TestEngine_NavigateBlocksUnsafeScheme(t *testing.T) {
	e, _ := NewEngine(&mockDriver{kind: EngineChromedp}, Config{Engine: EngineChromedp})
	s, _ := e.Open(context.Background(), nexus.SessionOptions{})
	defer s.Close()
	for _, bad := range []string{"file:///etc/passwd", "javascript:alert(1)", "data:text/html,x", "vbscript:msgbox"} {
		if err := e.Navigate(context.Background(), s, bad); err == nil {
			t.Errorf("expected block for %q", bad)
		}
	}
}

func TestEngine_NavigateAllowlistEnforced(t *testing.T) {
	e, _ := NewEngine(&mockDriver{kind: EngineChromedp}, Config{
		Engine:       EngineChromedp,
		AllowedHosts: []string{"example.com"},
	})
	s, _ := e.Open(context.Background(), nexus.SessionOptions{})
	defer s.Close()
	if err := e.Navigate(context.Background(), s, "https://evil.test/x"); err == nil {
		t.Fatal("expected allowlist block")
	}
	if err := e.Navigate(context.Background(), s, "https://example.com/home"); err != nil {
		t.Fatalf("example.com should be allowed, got %v", err)
	}
}

func TestEngine_NavigateEmptyURL(t *testing.T) {
	e, _ := NewEngine(&mockDriver{kind: EngineChromedp}, Config{Engine: EngineChromedp})
	s, _ := e.Open(context.Background(), nexus.SessionOptions{})
	defer s.Close()
	if err := e.Navigate(context.Background(), s, ""); err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestEngine_Do_ClickAndType(t *testing.T) {
	d := &mockDriver{kind: EngineChromedp}
	h := &mockHandle{}
	d.open = func(_ context.Context, _ Config) (SessionHandle, error) { return h, nil }
	e, _ := NewEngine(d, Config{Engine: EngineChromedp})
	s, _ := e.Open(context.Background(), nexus.SessionOptions{})

	if err := e.Do(context.Background(), s, nexus.Action{Kind: "click", Target: "e5"}); err != nil {
		t.Fatal(err)
	}
	if h.lastClick != "e5" {
		t.Errorf("click target = %q, want e5", h.lastClick)
	}

	if err := e.Do(context.Background(), s, nexus.Action{Kind: "type", Target: "e7", Text: "hello"}); err != nil {
		t.Fatal(err)
	}
	if h.lastType != "e7" || h.lastText != "hello" {
		t.Errorf("type args = (%q, %q), want (e7, hello)", h.lastType, h.lastText)
	}

	if err := e.Do(context.Background(), s, nexus.Action{Kind: "scroll", Params: map[string]any{"dx": 0, "dy": 400}}); err != nil {
		t.Fatal(err)
	}
	if h.scrollDY != 400 {
		t.Errorf("scrollDY = %d, want 400", h.scrollDY)
	}
}

func TestEngine_Do_UnknownAction(t *testing.T) {
	e, _ := NewEngine(&mockDriver{kind: EngineChromedp}, Config{Engine: EngineChromedp})
	s, _ := e.Open(context.Background(), nexus.SessionOptions{})
	if err := e.Do(context.Background(), s, nexus.Action{Kind: "teleport"}); err == nil {
		t.Fatal("expected error for unknown action")
	}
}

func TestEngine_ForeignSessionTypeRejected(t *testing.T) {
	e, _ := NewEngine(&mockDriver{kind: EngineChromedp}, Config{Engine: EngineChromedp})
	bogus := fakeSession{}
	if _, err := e.Snapshot(context.Background(), bogus); err == nil {
		t.Error("expected error for foreign session type on Snapshot")
	}
	if err := e.Navigate(context.Background(), bogus, "https://example.com"); err == nil {
		t.Error("expected error for foreign session type on Navigate")
	}
	if _, err := e.Screenshot(context.Background(), bogus); err == nil {
		t.Error("expected error for foreign session type on Screenshot")
	}
	if err := e.Do(context.Background(), bogus, nexus.Action{Kind: "click"}); err == nil {
		t.Error("expected error for foreign session type on Do")
	}
}

type fakeSession struct{}

func (fakeSession) ID() string              { return "fake" }
func (fakeSession) Platform() nexus.Platform { return nexus.PlatformWebChromedp }
func (fakeSession) Close() error             { return nil }

func TestEngine_ConcurrentOpenClose(t *testing.T) {
	e, _ := NewEngine(&mockDriver{kind: EngineChromedp}, Config{Engine: EngineChromedp})
	var wg sync.WaitGroup
	var opened atomic.Int64
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := e.Open(context.Background(), nexus.SessionOptions{})
			if err != nil {
				t.Errorf("open: %v", err)
				return
			}
			opened.Add(1)
			_ = s.Close()
		}()
	}
	wg.Wait()
	if e.ActiveSessions() != 0 {
		t.Errorf("after all close, ActiveSessions = %d", e.ActiveSessions())
	}
	if opened.Load() != 50 {
		t.Errorf("only %d of 50 sessions opened", opened.Load())
	}
}

// TestEngine_SnapshotPassesThrough guards against stripping content on
// valid calls.
func TestEngine_SnapshotPassesThrough(t *testing.T) {
	e, _ := NewEngine(&mockDriver{kind: EngineChromedp}, Config{Engine: EngineChromedp})
	s, _ := e.Open(context.Background(), nexus.SessionOptions{})
	defer s.Close()
	snap, err := e.Snapshot(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Elements) != 1 || snap.Elements[0].Ref != "e1" {
		t.Errorf("snapshot not passed through: %+v", snap)
	}
}

// TestEngine_ScreenshotPassesThrough guards against accidental nil/empty.
func TestEngine_ScreenshotPassesThrough(t *testing.T) {
	e, _ := NewEngine(&mockDriver{kind: EngineChromedp}, Config{Engine: EngineChromedp})
	s, _ := e.Open(context.Background(), nexus.SessionOptions{})
	defer s.Close()
	buf, err := e.Screenshot(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	if len(buf) == 0 {
		t.Error("expected non-empty screenshot")
	}
}

// containsIgnoreCase is a tiny helper to avoid importing strings twice.
func containsIgnoreCase(s, sub string) bool {
	if len(sub) > len(s) {
		return false
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		match := true
		for j := 0; j < len(sub); j++ {
			c1 := s[i+j]
			c2 := sub[j]
			if c1 >= 'A' && c1 <= 'Z' {
				c1 += 'a' - 'A'
			}
			if c2 >= 'A' && c2 <= 'Z' {
				c2 += 'a' - 'A'
			}
			if c1 != c2 {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// reference io to avoid unused-import complaints if this file shrinks
var _ = io.EOF
