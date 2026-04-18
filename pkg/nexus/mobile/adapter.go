package mobile

import (
	"context"
	"errors"
	"fmt"

	"digital.vasic.helixqa/pkg/nexus"
)

// Engine is the Nexus-facing adapter that presents an AppiumClient as
// a nexus.Adapter. A single Engine targets one Capabilities profile;
// callers rebuild the Engine when switching between iOS and Android.
type Engine struct {
	client *AppiumClient
	caps   Capabilities
}

// NewEngine returns an Engine bound to the given Appium URL and caps.
// The session is lazily created on Open.
func NewEngine(appiumURL string, caps Capabilities) (*Engine, error) {
	if err := caps.Validate(); err != nil {
		return nil, err
	}
	return &Engine{
		client: NewAppiumClient(appiumURL),
		caps:   caps,
	}, nil
}

// WithClient injects a preconfigured AppiumClient (useful for tests
// driving an httptest.Server).
func (e *Engine) WithClient(c *AppiumClient) *Engine {
	e.client = c
	return e
}

// Client exposes the underlying AppiumClient.
func (e *Engine) Client() *AppiumClient { return e.client }

// Platform returns the session's nexus.Platform.
func (e *Engine) Platform() nexus.Platform { return e.client.Platform(e.caps) }

// Open creates a new Appium session.
func (e *Engine) Open(ctx context.Context, _ nexus.SessionOptions) (nexus.Session, error) {
	if err := e.client.NewSession(ctx, e.caps); err != nil {
		return nil, fmt.Errorf("mobile open: %w", err)
	}
	return &mobileSession{id: e.client.SessionID(), engine: e}, nil
}

// Navigate launches the app activity / bundle or deep-links to url.
func (e *Engine) Navigate(ctx context.Context, s nexus.Session, target string) error {
	if _, ok := s.(*mobileSession); !ok {
		return fmt.Errorf("mobile: foreign session %T", s)
	}
	if target == "" {
		return errors.New("mobile: empty navigation target")
	}
	if e.caps.Platform == PlatformIOS {
		_, err := e.client.ExecuteScript(ctx, "mobile: launchApp", map[string]any{"bundleId": e.caps.BundleID})
		if err != nil {
			return err
		}
	} else {
		_, err := e.client.ExecuteScript(ctx, "mobile: startActivity", map[string]any{
			"intent": fmt.Sprintf("%s/%s", e.caps.AppPackage, e.caps.AppActivity),
		})
		if err != nil {
			return err
		}
	}
	// If target looks like a URL, attempt a deep-link intent.
	if len(target) > 7 && (target[:7] == "http://" || target[:8] == "https://" || target[:10] == "content://") {
		_, err := e.client.ExecuteScript(ctx, "mobile: deepLink", map[string]any{"url": target})
		return err
	}
	return nil
}

// Snapshot fetches page source + screenshot and returns a nexus.Snapshot.
func (e *Engine) Snapshot(ctx context.Context, s nexus.Session) (*nexus.Snapshot, error) {
	if _, ok := s.(*mobileSession); !ok {
		return nil, fmt.Errorf("mobile: foreign session %T", s)
	}
	xml, err := e.client.PageSource(ctx)
	if err != nil {
		return nil, err
	}
	tree, err := ParseAccessibilityTree(xml)
	if err != nil {
		return nil, err
	}
	png, _ := e.client.Screenshot(ctx) // non-fatal; snapshot is still useful without the frame
	elements := flattenElements(tree)
	return &nexus.Snapshot{
		Tree:     xml,
		Frame:    png,
		Elements: elements,
	}, nil
}

// Do executes a single Action.
func (e *Engine) Do(ctx context.Context, s nexus.Session, a nexus.Action) error {
	if _, ok := s.(*mobileSession); !ok {
		return fmt.Errorf("mobile: foreign session %T", s)
	}
	switch a.Kind {
	case "click", "tap":
		return e.tapOrClick(ctx, a)
	case "type":
		elID, err := e.resolve(ctx, a.Target)
		if err != nil {
			return err
		}
		return e.client.SendKeys(ctx, elID, a.Text)
	case "scroll":
		dir, _ := a.Params["direction"].(string)
		dist, _ := a.Params["distance"].(int)
		return NewGestures(e.client, e.caps.Platform).Scroll(ctx, dir, dist)
	case "key":
		return NewGestures(e.client, e.caps.Platform).Key(ctx, a.Target)

	// Phase-6 coord-grounded dispatch: map to the same underlying
	// Gestures primitives so UI-TARS-style (x, y) instructions work
	// on mobile without a separate Adapter.
	case "coord_click":
		return NewGestures(e.client, e.caps.Platform).Tap(ctx, a.X, a.Y)
	case "coord_type":
		// Tap the target first so focus lands on the input, then
		// send keys to the active element using Appium's
		// no-element path (empty element id).
		if err := NewGestures(e.client, e.caps.Platform).Tap(ctx, a.X, a.Y); err != nil {
			return fmt.Errorf("mobile coord_type tap: %w", err)
		}
		return e.client.SendKeys(ctx, "", a.Text)
	case "coord_scroll":
		dx, _ := a.Params["dx"].(int)
		dy, _ := a.Params["dy"].(int)
		// Derive a direction/distance pair so the same Gestures.Scroll
		// primitive dispatches. Prefer the larger axis; fall back to
		// the configured default when both are zero.
		dir := "down"
		dist := 0
		if absInt(dy) >= absInt(dx) {
			if dy < 0 {
				dir = "up"
				dist = -dy
			} else {
				dir = "down"
				dist = dy
			}
		} else {
			if dx < 0 {
				dir = "left"
				dist = -dx
			} else {
				dir = "right"
				dist = dx
			}
		}
		return NewGestures(e.client, e.caps.Platform).Scroll(ctx, dir, dist)

	default:
		return fmt.Errorf("mobile: unsupported action kind %q", a.Kind)
	}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// Screenshot fetches a PNG frame.
func (e *Engine) Screenshot(ctx context.Context, s nexus.Session) ([]byte, error) {
	if _, ok := s.(*mobileSession); !ok {
		return nil, fmt.Errorf("mobile: foreign session %T", s)
	}
	return e.client.Screenshot(ctx)
}

func (e *Engine) tapOrClick(ctx context.Context, a nexus.Action) error {
	if a.Target != "" {
		elID, err := e.resolve(ctx, a.Target)
		if err != nil {
			return err
		}
		return e.client.Click(ctx, elID)
	}
	return NewGestures(e.client, e.caps.Platform).Tap(ctx, a.X, a.Y)
}

// resolve turns a Nexus element ref or selector into an Appium element id.
func (e *Engine) resolve(ctx context.Context, target string) (string, error) {
	// Treat refs that start with 'a' (from ParseAccessibilityTree) as
	// indirection through the current page source; for now we fall back
	// to accessibility id lookup on the content-desc or label string.
	strategy := "xpath"
	value := target
	switch {
	case len(target) > 0 && target[0] == '#':
		strategy = "id"
		value = target[1:]
	case len(target) > 0 && target[0] == '/':
		strategy = "xpath"
	case len(target) > 2 && target[:2] == "~":
		strategy = "accessibility id"
		value = target[2:]
	}
	return e.client.FindElement(ctx, strategy, value)
}

func flattenElements(root *AccessibilityNode) []nexus.Element {
	var out []nexus.Element
	_ = root.Walk(func(n *AccessibilityNode) error {
		label := n.Label
		if label == "" {
			label = n.ContentDesc
		}
		if label == "" {
			label = n.Text
		}
		if label == "" && !n.Clickable {
			return nil
		}
		out = append(out, nexus.Element{
			Ref:   nexus.ElementRef(n.Ref),
			Role:  roleFromClass(n.Class),
			Name:  label,
		})
		return nil
	})
	return out
}

func roleFromClass(class string) string {
	switch {
	case class == "XCUIElementTypeButton" || class == "android.widget.Button":
		return "button"
	case class == "XCUIElementTypeTextField" || class == "android.widget.EditText":
		return "textbox"
	case class == "XCUIElementTypeLink" || class == "android.widget.ImageButton":
		return "link"
	case class == "XCUIElementTypeCell" || class == "android.widget.TextView":
		return "text"
	}
	return "generic"
}

type mobileSession struct {
	id     string
	engine *Engine
}

func (s *mobileSession) ID() string              { return s.id }
func (s *mobileSession) Platform() nexus.Platform { return s.engine.Platform() }
func (s *mobileSession) Close() error             { return s.engine.client.DeleteSession(context.Background()) }

var _ nexus.Adapter = (*Engine)(nil)
var _ nexus.Session = (*mobileSession)(nil)
