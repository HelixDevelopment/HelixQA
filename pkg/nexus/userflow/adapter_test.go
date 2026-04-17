package userflow

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	uf "digital.vasic.challenges/pkg/userflow"

	"digital.vasic.helixqa/pkg/nexus"
	"digital.vasic.helixqa/pkg/nexus/browser"
)

// testDriver is a minimal Driver for the adapter tests. It satisfies
// the browser.Driver contract without pulling in chromedp / go-rod.
type testDriver struct {
	kind    browser.EngineType
	openErr error
	handle  *testHandle
}

func (d *testDriver) Kind() browser.EngineType { return d.kind }
func (d *testDriver) Open(_ context.Context, _ browser.Config) (browser.SessionHandle, error) {
	if d.openErr != nil {
		return nil, d.openErr
	}
	if d.handle == nil {
		d.handle = &testHandle{}
	}
	return d.handle, nil
}

type testHandle struct {
	mu        sync.Mutex
	closed    bool
	navigated []string
	clicks    []string
	typed     map[string]string
	html      string
	shot      []byte
}

func (h *testHandle) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	return nil
}
func (h *testHandle) Navigate(_ context.Context, url string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.navigated = append(h.navigated, url)
	return nil
}
func (h *testHandle) Snapshot(_ context.Context) (*nexus.Snapshot, error) {
	html := h.html
	if html == "" {
		html = `<button id="submit" aria-label="Save"></button>`
	}
	return browser.SnapshotFromHTML(html, h.shot)
}
func (h *testHandle) Click(_ context.Context, ref nexus.ElementRef) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clicks = append(h.clicks, string(ref))
	return nil
}
func (h *testHandle) Type(_ context.Context, ref nexus.ElementRef, text string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.typed == nil {
		h.typed = map[string]string{}
	}
	h.typed[string(ref)] = text
	return nil
}
func (h *testHandle) Screenshot(_ context.Context) ([]byte, error) { return []byte("PNG"), nil }
func (h *testHandle) Scroll(_ context.Context, _, _ int) error     { return nil }

func newAdapter(t *testing.T) (*NexusBrowserAdapter, *testHandle) {
	t.Helper()
	d := &testDriver{kind: browser.EngineChromedp}
	eng, err := browser.NewEngine(d, browser.Config{Engine: browser.EngineChromedp})
	if err != nil {
		t.Fatal(err)
	}
	a := NewNexusBrowserAdapter(eng)
	if err := a.Initialize(context.Background(), uf.BrowserConfig{Headless: true, WindowSize: [2]int{1024, 768}}); err != nil {
		t.Fatal(err)
	}
	return a, d.handle
}

func TestAdapter_InitializeTwiceFails(t *testing.T) {
	a, _ := newAdapter(t)
	err := a.Initialize(context.Background(), uf.BrowserConfig{})
	if err == nil {
		t.Fatal("second Initialize must fail")
	}
}

func TestAdapter_Navigate(t *testing.T) {
	a, h := newAdapter(t)
	if err := a.Navigate(context.Background(), "https://example.com"); err != nil {
		t.Fatal(err)
	}
	if len(h.navigated) != 1 || h.navigated[0] != "https://example.com" {
		t.Errorf("driver did not record navigate: %v", h.navigated)
	}
}

func TestAdapter_ClickAndFill(t *testing.T) {
	a, h := newAdapter(t)
	if err := a.Click(context.Background(), "e1"); err != nil {
		t.Fatal(err)
	}
	if err := a.Fill(context.Background(), "e2", "hello"); err != nil {
		t.Fatal(err)
	}
	if len(h.clicks) != 1 || h.clicks[0] != "e1" {
		t.Errorf("click not recorded: %+v", h.clicks)
	}
	if h.typed["e2"] != "hello" {
		t.Errorf("fill not recorded: %+v", h.typed)
	}
}

func TestAdapter_IsVisibleFromSnapshot(t *testing.T) {
	a, h := newAdapter(t)
	h.html = `<button id="submit">Save</button><a href=# id="home">Home</a>`
	visible, err := a.IsVisible(context.Background(), "#submit")
	if err != nil {
		t.Fatal(err)
	}
	if !visible {
		t.Error("expected #submit to be visible")
	}
	visible, err = a.IsVisible(context.Background(), "#not-there")
	if err != nil {
		t.Fatal(err)
	}
	if visible {
		t.Error("expected non-existent selector to be invisible")
	}
}

func TestAdapter_GetTextFromSnapshot(t *testing.T) {
	a, h := newAdapter(t)
	h.html = `<button id="save" aria-label="Save"></button>`
	got, err := a.GetText(context.Background(), "#save")
	if err != nil {
		t.Fatal(err)
	}
	if got != "Save" {
		t.Errorf("GetText = %q, want Save", got)
	}
}

func TestAdapter_Screenshot(t *testing.T) {
	a, _ := newAdapter(t)
	buf, err := a.Screenshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if string(buf) != "PNG" {
		t.Errorf("unexpected screenshot bytes: %q", string(buf))
	}
}

func TestAdapter_EvaluateJSRefused(t *testing.T) {
	a, _ := newAdapter(t)
	_, err := a.EvaluateJS(context.Background(), "1+1")
	if err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Errorf("EvaluateJS must be refused by policy, got err=%v", err)
	}
}

func TestAdapter_CloseIdempotent(t *testing.T) {
	a, _ := newAdapter(t)
	if err := a.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := a.Close(context.Background()); err != nil {
		t.Fatalf("second Close should be a no-op, got %v", err)
	}
	if a.Available(context.Background()) {
		t.Error("Available must be false after Close")
	}
}

func TestAdapter_OperationsRequireInitialize(t *testing.T) {
	eng, _ := browser.NewEngine(&testDriver{kind: browser.EngineChromedp}, browser.Config{Engine: browser.EngineChromedp})
	a := NewNexusBrowserAdapter(eng)

	if err := a.Navigate(context.Background(), "https://example.com"); err == nil {
		t.Error("Navigate without Initialize should fail")
	}
	if err := a.Click(context.Background(), "e1"); err == nil {
		t.Error("Click without Initialize should fail")
	}
	if _, err := a.Screenshot(context.Background()); err == nil {
		t.Error("Screenshot without Initialize should fail")
	}
}

func TestAdapter_WaitForSelectorTimesOut(t *testing.T) {
	a, _ := newAdapter(t)
	start := time.Now()
	err := a.WaitForSelector(context.Background(), "#never-there", 75*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if time.Since(start) < 75*time.Millisecond {
		t.Errorf("WaitForSelector returned too early: %s", time.Since(start))
	}
}

func TestAdapter_InitializeFailsOnOpenError(t *testing.T) {
	d := &testDriver{kind: browser.EngineChromedp, openErr: errors.New("no chromium")}
	eng, _ := browser.NewEngine(d, browser.Config{Engine: browser.EngineChromedp})
	a := NewNexusBrowserAdapter(eng)
	if err := a.Initialize(context.Background(), uf.BrowserConfig{}); err == nil {
		t.Fatal("expected error propagation")
	}
}
