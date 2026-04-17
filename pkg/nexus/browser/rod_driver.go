//go:build nexus_rod

package browser

import (
	"context"
	"fmt"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"

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
	page, err := browser.Page(rod.PageOption{})
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
	return el.Click("left", 1)
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

var _ Driver = (*RodDriver)(nil)
var _ SessionHandle = (*rodHandle)(nil)
