//go:build nexus_chromedp

package browser

import (
	"context"
	"fmt"

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
		chromedp.Headless,
		chromedp.NoSandbox,
		chromedp.DisableGPU,
		chromedp.Flag("remote-debugging-address", "127.0.0.1"),
		chromedp.Flag("disable-dev-shm-usage", true),
	}
	if cfg.UserDataDir != "" {
		opts = append(opts, chromedp.UserDataDir(cfg.UserDataDir))
	}
	if !cfg.Headless {
		opts = []chromedp.ExecAllocatorOption{
			chromedp.NoSandbox,
			chromedp.Flag("remote-debugging-address", "127.0.0.1"),
		}
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
		ctx:          browserCtx,
		cancelCtx:    browserCancel,
		cancelAlloc:  allocCancel,
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
	// Convert ref back to a selector via a small JS lookup. In Phase 1
	// refs are resolved by taking a fresh snapshot before the click;
	// operator-owned integrations may prefer to keep the selector from
	// the previous Snapshot's Elements table.
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

var _ Driver = (*ChromedpDriver)(nil)
var _ SessionHandle = (*chromedpHandle)(nil)
