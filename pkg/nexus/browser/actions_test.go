package browser

import (
	"context"
	"errors"
	"testing"
	"time"

	"digital.vasic.helixqa/pkg/nexus"
)

// extHandle is a mockHandle that also implements ExtendedHandle.
type extHandle struct {
	mockHandle
	hoverRef   nexus.ElementRef
	dragFrom   nexus.ElementRef
	dragTo     nexus.ElementRef
	selectRef  nexus.ElementRef
	selectVal  string
	waitRef    nexus.ElementRef
	waitTO     time.Duration
	openTabURL string
	closeTabID string
	pdfCalled  bool
	consoleN   int
}

func (h *extHandle) Hover(_ context.Context, r nexus.ElementRef) error {
	h.hoverRef = r
	return nil
}
func (h *extHandle) Drag(_ context.Context, from, to nexus.ElementRef) error {
	h.dragFrom, h.dragTo = from, to
	return nil
}
func (h *extHandle) SelectOption(_ context.Context, r nexus.ElementRef, v string) error {
	h.selectRef, h.selectVal = r, v
	return nil
}
func (h *extHandle) WaitFor(_ context.Context, r nexus.ElementRef, t time.Duration) error {
	h.waitRef, h.waitTO = r, t
	return nil
}
func (h *extHandle) OpenTab(_ context.Context, url string) (string, error) {
	h.openTabURL = url
	return "tab-1", nil
}
func (h *extHandle) CloseTab(_ context.Context, id string) error {
	h.closeTabID = id
	return nil
}
func (h *extHandle) SavePDF(_ context.Context) ([]byte, error) {
	h.pdfCalled = true
	return []byte("%PDF"), nil
}
func (h *extHandle) ConsoleMessages(_ context.Context) ([]ConsoleMessage, error) {
	h.consoleN++
	return []ConsoleMessage{{Level: "info", Text: "ready"}}, nil
}

type extDriver struct {
	handle *extHandle
}

func (d *extDriver) Kind() EngineType { return EngineChromedp }
func (d *extDriver) Open(_ context.Context, _ Config) (SessionHandle, error) {
	if d.handle == nil {
		d.handle = &extHandle{}
	}
	return d.handle, nil
}

func TestDoExtended_AllKinds(t *testing.T) {
	d := &extDriver{}
	e, _ := NewEngine(d, Config{Engine: EngineChromedp})
	s, _ := e.Open(context.Background(), nexus.SessionOptions{})
	h := d.handle

	cases := []nexus.Action{
		{Kind: "hover", Target: "e1"},
		{Kind: "drag", Target: "e2", Params: map[string]any{"to": "e3"}},
		{Kind: "select", Target: "e4", Text: "value"},
		{Kind: "wait_for", Target: "e5", Timeout: 25 * time.Millisecond},
		{Kind: "tab_open", Target: "https://example.com"},
		{Kind: "tab_close", Target: "tab-1"},
		{Kind: "pdf"},
		{Kind: "console_read"},
	}
	for _, a := range cases {
		if err := e.DoExtended(context.Background(), s, a); err != nil {
			t.Errorf("DoExtended(%s): %v", a.Kind, err)
		}
	}
	if h.hoverRef != "e1" || h.dragFrom != "e2" || h.dragTo != "e3" {
		t.Errorf("unexpected handle state: %+v", h)
	}
	if h.selectRef != "e4" || h.selectVal != "value" {
		t.Errorf("select not wired: %+v", h)
	}
	if h.waitRef != "e5" || h.waitTO != 25*time.Millisecond {
		t.Errorf("wait not wired: %+v", h)
	}
	if h.openTabURL != "https://example.com" || h.closeTabID != "tab-1" {
		t.Errorf("tab actions not wired: %+v", h)
	}
	if !h.pdfCalled || h.consoleN != 1 {
		t.Errorf("pdf/console not wired: %+v", h)
	}
}

func TestDoExtended_DragRequiresTo(t *testing.T) {
	e, _ := NewEngine(&extDriver{}, Config{Engine: EngineChromedp})
	s, _ := e.Open(context.Background(), nexus.SessionOptions{})
	if err := e.DoExtended(context.Background(), s, nexus.Action{Kind: "drag", Target: "a"}); err == nil {
		t.Fatal("drag without Params[to] must error")
	}
}

func TestDoExtended_UnsupportedWhenDriverLacksInterface(t *testing.T) {
	d := &mockDriver{kind: EngineChromedp}
	e, _ := NewEngine(d, Config{Engine: EngineChromedp})
	s, _ := e.Open(context.Background(), nexus.SessionOptions{})
	err := e.DoExtended(context.Background(), s, nexus.Action{Kind: "hover", Target: "e1"})
	if err == nil || !errors.Is(err, ErrActionUnsupported) {
		t.Errorf("expected ErrActionUnsupported, got %v", err)
	}
}

func TestDoExtended_UnknownKind(t *testing.T) {
	d := &extDriver{}
	e, _ := NewEngine(d, Config{Engine: EngineChromedp})
	s, _ := e.Open(context.Background(), nexus.SessionOptions{})
	err := e.DoExtended(context.Background(), s, nexus.Action{Kind: "teleport", Target: "x"})
	if !errors.Is(err, ErrActionUnsupported) {
		t.Errorf("unknown kind should be unsupported, got %v", err)
	}
}

func TestDoExtended_ForeignSessionRejected(t *testing.T) {
	d := &extDriver{}
	e, _ := NewEngine(d, Config{Engine: EngineChromedp})
	if err := e.DoExtended(context.Background(), fakeSession{}, nexus.Action{Kind: "hover"}); err == nil {
		t.Fatal("foreign session must be rejected")
	}
}

func TestExtendedAccessor(t *testing.T) {
	d := &extDriver{}
	e, _ := NewEngine(d, Config{Engine: EngineChromedp})
	s, _ := e.Open(context.Background(), nexus.SessionOptions{})
	if e.Extended(s) == nil {
		t.Error("Extended should return non-nil for supporting driver")
	}
	if e.Extended(fakeSession{}) != nil {
		t.Error("Extended should return nil for foreign session")
	}

	d2 := &mockDriver{kind: EngineChromedp}
	e2, _ := NewEngine(d2, Config{Engine: EngineChromedp})
	s2, _ := e2.Open(context.Background(), nexus.SessionOptions{})
	if e2.Extended(s2) != nil {
		t.Error("Extended should return nil when driver lacks interface")
	}
}

func TestDoExtended_WaitForDefaultsTimeout(t *testing.T) {
	d := &extDriver{}
	e, _ := NewEngine(d, Config{Engine: EngineChromedp})
	s, _ := e.Open(context.Background(), nexus.SessionOptions{})
	_ = e.DoExtended(context.Background(), s, nexus.Action{Kind: "wait_for", Target: "e1"})
	if d.handle.waitTO != 10*time.Second {
		t.Errorf("default wait timeout = %v, want 10s", d.handle.waitTO)
	}
}
