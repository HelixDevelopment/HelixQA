//go:build nexus_chromedp

package browser

import (
	"context"
	"fmt"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"

	"digital.vasic.helixqa/pkg/nexus"
)

// ChromedpDriver drives a real Chromium instance over CDP via the
// chromedp library. The driver is guarded by the `nexus_chromedp` build
// tag so unit tests on CI-less workstations do not need a Chrome binary.
// Enable it in operator environments with:
//
//	go build -tags=nexus_chromedp ./...
type ChromedpDriver struct{}

// NewChromedpDriver returns a ready-to-use ChromedpDriver.
func NewChromedpDriver() *ChromedpDriver { return &ChromedpDriver{} }

// Kind reports the driver type.
func (*ChromedpDriver) Kind() EngineType { return EngineChromedp }

// Open launches a fresh Chromium process with hardened flags and
// returns a SessionHandle.
func (*ChromedpDriver) Open(ctx context.Context, cfg Config) (SessionHandle, error) {
	opts := []chromedp.ExecAllocatorOption{
		chromedp.NoSandbox,
		chromedp.DisableGPU,
		chromedp.Flag("remote-debugging-address", "127.0.0.1"),
		chromedp.Flag("disable-dev-shm-usage", true),
	}
	if cfg.Headless {
		opts = append(opts, chromedp.Headless)
	}
	if cfg.UserDataDir != "" {
		opts = append(opts, chromedp.UserDataDir(cfg.UserDataDir))
	}
	if cfg.WindowWidth > 0 && cfg.WindowHeight > 0 {
		opts = append(opts, chromedp.WindowSize(cfg.WindowWidth, cfg.WindowHeight))
	}
	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	if err := chromedp.Run(browserCtx); err != nil {
		allocCancel()
		browserCancel()
		return nil, fmt.Errorf("chromedp start: %w", err)
	}
	return &chromedpHandle{
		ctx:         browserCtx,
		cancelCtx:   browserCancel,
		cancelAlloc: allocCancel,
	}, nil
}

type chromedpHandle struct {
	ctx         context.Context
	cancelCtx   context.CancelFunc
	cancelAlloc context.CancelFunc
}

func (h *chromedpHandle) Close() error {
	h.cancelCtx()
	h.cancelAlloc()
	return nil
}

func (h *chromedpHandle) Navigate(ctx context.Context, url string) error {
	return chromedp.Run(h.ctx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
	)
}

func (h *chromedpHandle) Snapshot(ctx context.Context) (*nexus.Snapshot, error) {
	var html string
	var png []byte
	if err := chromedp.Run(h.ctx,
		chromedp.OuterHTML("html", &html),
		chromedp.CaptureScreenshot(&png),
	); err != nil {
		return nil, err
	}
	return SnapshotFromHTML(html, png)
}

func (h *chromedpHandle) Click(ctx context.Context, ref nexus.ElementRef) error {
	return chromedp.Run(h.ctx, chromedp.Click(string(ref), chromedp.NodeVisible))
}

func (h *chromedpHandle) Type(ctx context.Context, ref nexus.ElementRef, text string) error {
	return chromedp.Run(h.ctx,
		chromedp.SendKeys(string(ref), text, chromedp.NodeVisible),
	)
}

func (h *chromedpHandle) Screenshot(ctx context.Context) ([]byte, error) {
	var png []byte
	if err := chromedp.Run(h.ctx, chromedp.CaptureScreenshot(&png)); err != nil {
		return nil, err
	}
	return png, nil
}

func (h *chromedpHandle) Scroll(ctx context.Context, dx, dy int) error {
	script := fmt.Sprintf("window.scrollBy(%d, %d);", dx, dy)
	return chromedp.Run(h.ctx, chromedp.Evaluate(script, nil))
}

// --- ExtendedHandle implementation ---

func (h *chromedpHandle) Hover(ctx context.Context, ref nexus.ElementRef) error {
	// Fixes B3 from docs/nexus/remaining-work.md: the previous
	// implementation called chromedp.MouseEvent("", chromedp.NodeVisible)
	// which is not a real chromedp action signature. The correct
	// approach is to JS-dispatch a mouseover event on the matched
	// element after waiting for visibility. This keeps the hover
	// synchronous and testable without reaching into cdp's Input
	// domain directly.
	script := fmt.Sprintf(`(() => {
      const el = document.querySelector(%q);
      if (!el) return false;
      const r = el.getBoundingClientRect();
      const opts = { bubbles: true, cancelable: true, clientX: r.x + r.width/2, clientY: r.y + r.height/2 };
      el.dispatchEvent(new MouseEvent('mouseover', opts));
      el.dispatchEvent(new MouseEvent('mouseenter', opts));
      el.dispatchEvent(new MouseEvent('mousemove', opts));
      return true;
    })()`, string(ref))
	var ok bool
	return chromedp.Run(h.ctx,
		chromedp.WaitVisible(string(ref)),
		chromedp.Evaluate(script, &ok),
	)
}

func (h *chromedpHandle) Drag(ctx context.Context, from, to nexus.ElementRef) error {
	script := fmt.Sprintf(`(() => {
      const from = document.querySelector(%q);
      const to = document.querySelector(%q);
      if (!from || !to) return false;
      const rFrom = from.getBoundingClientRect();
      const rTo = to.getBoundingClientRect();
      from.dispatchEvent(new DragEvent('dragstart', {bubbles:true}));
      to.dispatchEvent(new DragEvent('dragover', {bubbles:true, clientX: rTo.x, clientY: rTo.y}));
      to.dispatchEvent(new DragEvent('drop', {bubbles:true, clientX: rTo.x, clientY: rTo.y}));
      from.dispatchEvent(new DragEvent('dragend', {bubbles:true}));
      return true;
    })()`, string(from), string(to))
	var ok bool
	return chromedp.Run(h.ctx, chromedp.Evaluate(script, &ok))
}

func (h *chromedpHandle) SelectOption(ctx context.Context, ref nexus.ElementRef, value string) error {
	return chromedp.Run(h.ctx, chromedp.SetValue(string(ref), value, chromedp.NodeVisible))
}

func (h *chromedpHandle) WaitFor(ctx context.Context, ref nexus.ElementRef, timeout time.Duration) error {
	waitCtx, cancel := context.WithTimeout(h.ctx, timeout)
	defer cancel()
	return chromedp.Run(waitCtx, chromedp.WaitVisible(string(ref)))
}

func (h *chromedpHandle) OpenTab(ctx context.Context, url string) (string, error) {
	var targetID target.ID
	if err := chromedp.Run(h.ctx, chromedp.ActionFunc(func(c context.Context) error {
		id, err := target.CreateTarget(url).Do(c)
		if err != nil {
			return err
		}
		targetID = id
		return nil
	})); err != nil {
		return "", err
	}
	return string(targetID), nil
}

func (h *chromedpHandle) CloseTab(ctx context.Context, tabID string) error {
	return chromedp.Run(h.ctx, chromedp.ActionFunc(func(c context.Context) error {
		_, err := target.CloseTarget(target.ID(tabID)).Do(c)
		return err
	}))
}

func (h *chromedpHandle) SavePDF(ctx context.Context) ([]byte, error) {
	var pdf []byte
	if err := chromedp.Run(h.ctx, chromedp.ActionFunc(func(c context.Context) error {
		data, _, err := page.PrintToPDF().Do(c)
		if err != nil {
			return err
		}
		pdf = data
		return nil
	})); err != nil {
		return nil, err
	}
	return pdf, nil
}

func (h *chromedpHandle) ConsoleMessages(ctx context.Context) ([]ConsoleMessage, error) {
	// chromedp does not ship a ready-made log buffer; operators
	// subscribe to runtime.ConsoleAPICalled in their own harness and
	// feed entries here. We surface a stub that returns whatever the
	// runtime currently exposes via window.console entries captured
	// on the page so the interface is at least self-contained.
	var entries []map[string]any
	if err := chromedp.Run(h.ctx, chromedp.Evaluate(
		`(window.__helixConsole || []).slice(-200)`, &entries)); err != nil {
		return nil, err
	}
	out := make([]ConsoleMessage, 0, len(entries))
	for _, e := range entries {
		msg := ConsoleMessage{
			Level: stringField(e, "level"),
			Text:  stringField(e, "text"),
			URL:   stringField(e, "url"),
		}
		if l, ok := e["line"].(float64); ok {
			msg.Line = int(l)
		}
		out = append(out, msg)
	}
	return out, nil
}

func stringField(m map[string]any, k string) string {
	if v, ok := m[k].(string); ok {
		return v
	}
	return ""
}

var _ Driver = (*ChromedpDriver)(nil)
var _ SessionHandle = (*chromedpHandle)(nil)
var _ ExtendedHandle = (*chromedpHandle)(nil)
