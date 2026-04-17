//go:build nexus_rod

package browser

import (
	"context"
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"

	"digital.vasic.helixqa/pkg/nexus"
)

// RodDriver drives Chromium through the go-rod library. Guarded by the
// `nexus_rod` build tag to keep the default build lean.
//
//	go build -tags=nexus_rod ./...
type RodDriver struct{}

// NewRodDriver returns a ready-to-use RodDriver.
func NewRodDriver() *RodDriver { return &RodDriver{} }

// Kind reports the driver type.
func (*RodDriver) Kind() EngineType { return EngineRod }

// Open launches a new browser via go-rod's launcher and returns a
// SessionHandle backed by a single page.
func (*RodDriver) Open(ctx context.Context, cfg Config) (SessionHandle, error) {
	l := launcher.New().Headless(cfg.Headless)
	if cfg.UserDataDir != "" {
		l = l.UserDataDir(cfg.UserDataDir)
	}
	if cfg.CDPPort > 0 {
		l = l.Set("remote-debugging-port", fmt.Sprintf("%d", cfg.CDPPort))
	}
	url, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("rod launch: %w", err)
	}
	browser := rod.New().ControlURL(url)
	if err := browser.Connect(); err != nil {
		return nil, fmt.Errorf("rod connect: %w", err)
	}
	page, err := browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		browser.Close()
		return nil, fmt.Errorf("rod new page: %w", err)
	}
	return &rodHandle{browser: browser, page: page}, nil
}

type rodHandle struct {
	browser *rod.Browser
	page    *rod.Page
}

func (h *rodHandle) Close() error                                            { return h.browser.Close() }
func (h *rodHandle) Navigate(_ context.Context, url string) error            { return h.page.Navigate(url) }
func (h *rodHandle) Snapshot(_ context.Context) (*nexus.Snapshot, error) {
	html, err := h.page.HTML()
	if err != nil {
		return nil, err
	}
	png, err := h.page.Screenshot(true, nil)
	if err != nil {
		return nil, err
	}
	return SnapshotFromHTML(html, png)
}
func (h *rodHandle) Click(_ context.Context, ref nexus.ElementRef) error {
	el, err := h.page.Element(string(ref))
	if err != nil {
		return err
	}
	return el.Click(proto.InputMouseButtonLeft, 1)
}
func (h *rodHandle) Type(_ context.Context, ref nexus.ElementRef, text string) error {
	el, err := h.page.Element(string(ref))
	if err != nil {
		return err
	}
	return el.Input(text)
}
func (h *rodHandle) Screenshot(_ context.Context) ([]byte, error) {
	return h.page.Screenshot(true, nil)
}
func (h *rodHandle) Scroll(_ context.Context, dx, dy int) error {
	_, err := h.page.Eval(fmt.Sprintf("() => window.scrollBy(%d, %d)", dx, dy))
	return err
}

// --- ExtendedHandle implementation ---

func (h *rodHandle) Hover(_ context.Context, ref nexus.ElementRef) error {
	el, err := h.page.Element(string(ref))
	if err != nil {
		return err
	}
	return el.Hover()
}

func (h *rodHandle) Drag(_ context.Context, from, to nexus.ElementRef) error {
	fromEl, err := h.page.Element(string(from))
	if err != nil {
		return fmt.Errorf("drag from: %w", err)
	}
	toEl, err := h.page.Element(string(to))
	if err != nil {
		return fmt.Errorf("drag to: %w", err)
	}
	fBox, err := fromEl.Shape()
	if err != nil {
		return err
	}
	tBox, err := toEl.Shape()
	if err != nil {
		return err
	}
	page := h.page
	if err := page.Mouse.MoveTo(proto.NewPoint(fBox.Box().X+fBox.Box().Width/2, fBox.Box().Y+fBox.Box().Height/2)); err != nil {
		return err
	}
	if err := page.Mouse.Down(proto.InputMouseButtonLeft, 1); err != nil {
		return err
	}
	if err := page.Mouse.MoveTo(proto.NewPoint(tBox.Box().X+tBox.Box().Width/2, tBox.Box().Y+tBox.Box().Height/2)); err != nil {
		return err
	}
	return page.Mouse.Up(proto.InputMouseButtonLeft, 1)
}

func (h *rodHandle) SelectOption(_ context.Context, ref nexus.ElementRef, value string) error {
	el, err := h.page.Element(string(ref))
	if err != nil {
		return err
	}
	return el.Select([]string{value}, true, rod.SelectorTypeText)
}

func (h *rodHandle) WaitFor(_ context.Context, ref nexus.ElementRef, timeout time.Duration) error {
	page := h.page.Timeout(timeout)
	_, err := page.Element(string(ref))
	return err
}

func (h *rodHandle) OpenTab(_ context.Context, url string) (string, error) {
	p, err := h.browser.Page(proto.TargetCreateTarget{URL: url})
	if err != nil {
		return "", err
	}
	return string(p.TargetID), nil
}

func (h *rodHandle) CloseTab(_ context.Context, tabID string) error {
	pages, err := h.browser.Pages()
	if err != nil {
		return err
	}
	for _, p := range pages {
		if string(p.TargetID) == tabID {
			return p.Close()
		}
	}
	return fmt.Errorf("rod: tab %q not found", tabID)
}

func (h *rodHandle) SavePDF(_ context.Context) ([]byte, error) {
	reader, err := h.page.PDF(&proto.PagePrintToPDF{})
	if err != nil {
		return nil, err
	}
	var buf []byte
	b := make([]byte, 32*1024)
	for {
		n, err := reader.Read(b)
		if n > 0 {
			buf = append(buf, b[:n]...)
		}
		if err != nil {
			break
		}
	}
	return buf, nil
}

func (h *rodHandle) ConsoleMessages(_ context.Context) ([]ConsoleMessage, error) {
	// go-rod exposes console events via page.EachEvent. The HelixQA
	// runtime subscribes at startup and feeds entries here; this
	// default implementation queries the page's own cache.
	val, err := h.page.Eval(`() => (window.__helixConsole || []).slice(-200)`)
	if err != nil {
		return nil, err
	}
	entries, ok := val.Value.Val().([]any)
	if !ok {
		return nil, nil
	}
	out := make([]ConsoleMessage, 0, len(entries))
	for _, raw := range entries {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		msg := ConsoleMessage{
			Level: stringFromMap(m, "level"),
			Text:  stringFromMap(m, "text"),
			URL:   stringFromMap(m, "url"),
		}
		if l, ok := m["line"].(float64); ok {
			msg.Line = int(l)
		}
		out = append(out, msg)
	}
	return out, nil
}

func stringFromMap(m map[string]any, k string) string {
	if v, ok := m[k].(string); ok {
		return v
	}
	return ""
}

var _ Driver = (*RodDriver)(nil)
var _ SessionHandle = (*rodHandle)(nil)
var _ ExtendedHandle = (*rodHandle)(nil)
