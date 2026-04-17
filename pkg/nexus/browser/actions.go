package browser

import (
	"context"
	"errors"
	"fmt"
	"time"

	"digital.vasic.helixqa/pkg/nexus"
)

// ExtendedHandle is the optional interface a Driver's SessionHandle
// may satisfy to expose the remaining Phase 1 actions. Drivers that
// implement it unlock drag / hover / select / wait_for / tab_open /
// tab_close / pdf / console_read via Engine.Do; drivers that don't
// return a clear "unsupported" error so callers can pick a different
// engine.
type ExtendedHandle interface {
	Hover(ctx context.Context, ref nexus.ElementRef) error
	Drag(ctx context.Context, from, to nexus.ElementRef) error
	SelectOption(ctx context.Context, ref nexus.ElementRef, value string) error
	WaitFor(ctx context.Context, ref nexus.ElementRef, timeout time.Duration) error
	OpenTab(ctx context.Context, url string) (string, error)
	CloseTab(ctx context.Context, tabID string) error
	SavePDF(ctx context.Context) ([]byte, error)
	ConsoleMessages(ctx context.Context) ([]ConsoleMessage, error)
}

// ConsoleMessage is one entry from the browser console.
type ConsoleMessage struct {
	Level   string    // debug | info | warn | error
	Text    string
	URL     string
	Line    int
	AtTime  time.Time
}

// ErrActionUnsupported is returned when the engine's current driver
// cannot perform the requested action.
var ErrActionUnsupported = errors.New("browser: action unsupported by driver")

// DoExtended lets callers perform the advanced actions without reaching
// into the Engine's internal session struct. It delegates to the
// driver's ExtendedHandle implementation when available.
func (e *Engine) DoExtended(ctx context.Context, s nexus.Session, a nexus.Action) error {
	sess, ok := s.(*session)
	if !ok {
		return fmt.Errorf("browser: foreign session type %T", s)
	}
	ext, ok := sess.handle.(ExtendedHandle)
	if !ok {
		return fmt.Errorf("%w: kind=%s", ErrActionUnsupported, a.Kind)
	}
	switch a.Kind {
	case "hover":
		return ext.Hover(ctx, nexus.ElementRef(a.Target))
	case "drag":
		to, _ := a.Params["to"].(string)
		if to == "" {
			return errors.New("browser: drag requires Params[\"to\"]")
		}
		return ext.Drag(ctx, nexus.ElementRef(a.Target), nexus.ElementRef(to))
	case "select":
		return ext.SelectOption(ctx, nexus.ElementRef(a.Target), a.Text)
	case "wait_for":
		timeout := a.Timeout
		if timeout <= 0 {
			timeout = 10 * time.Second
		}
		return ext.WaitFor(ctx, nexus.ElementRef(a.Target), timeout)
	case "tab_open":
		_, err := ext.OpenTab(ctx, a.Target)
		return err
	case "tab_close":
		return ext.CloseTab(ctx, a.Target)
	case "pdf":
		_, err := ext.SavePDF(ctx)
		return err
	case "console_read":
		_, err := ext.ConsoleMessages(ctx)
		return err
	default:
		return fmt.Errorf("%w: kind=%s", ErrActionUnsupported, a.Kind)
	}
}

// Extended returns the ExtendedHandle backing the session, or nil if
// the driver does not implement it. Useful when callers want the full
// typed results instead of going through DoExtended.
func (e *Engine) Extended(s nexus.Session) ExtendedHandle {
	sess, ok := s.(*session)
	if !ok {
		return nil
	}
	ext, _ := sess.handle.(ExtendedHandle)
	return ext
}
