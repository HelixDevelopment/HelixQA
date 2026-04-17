package mobile

import (
	"context"
	"fmt"
)

// Gestures wraps AppiumClient with mobile-specific interaction helpers
// (tap, longPress, swipe, pinch, rotate, hardware-button). Each helper
// dispatches the platform-appropriate `mobile:` script so callers stay
// platform-agnostic.
type Gestures struct {
	client *AppiumClient
	plat   PlatformType
}

// NewGestures wraps an AppiumClient for a known PlatformType.
func NewGestures(client *AppiumClient, platform PlatformType) *Gestures {
	return &Gestures{client: client, plat: platform}
}

// Tap performs a single tap at absolute coordinates.
func (g *Gestures) Tap(ctx context.Context, x, y int) error {
	script := "mobile: clickGesture"
	if g.plat == PlatformIOS {
		script = "mobile: tap"
	}
	_, err := g.client.ExecuteScript(ctx, script, map[string]any{"x": x, "y": y})
	return err
}

// LongPress holds a tap for the given duration in milliseconds.
func (g *Gestures) LongPress(ctx context.Context, x, y, durationMS int) error {
	script := "mobile: longClickGesture"
	args := map[string]any{"x": x, "y": y, "duration": durationMS}
	if g.plat == PlatformIOS {
		script = "mobile: touchAndHold"
		args = map[string]any{"x": x, "y": y, "duration": float64(durationMS) / 1000.0}
	}
	_, err := g.client.ExecuteScript(ctx, script, args)
	return err
}

// Swipe drags from (x1,y1) to (x2,y2) over durationMS milliseconds.
func (g *Gestures) Swipe(ctx context.Context, x1, y1, x2, y2, durationMS int) error {
	script := "mobile: swipeGesture"
	args := map[string]any{
		"startX": x1, "startY": y1, "endX": x2, "endY": y2,
		"speed": gestureSpeedFromDuration(durationMS),
	}
	if g.plat == PlatformIOS {
		script = "mobile: dragFromToForDuration"
		args = map[string]any{
			"fromX": x1, "fromY": y1, "toX": x2, "toY": y2,
			"duration": float64(durationMS) / 1000.0,
		}
	}
	_, err := g.client.ExecuteScript(ctx, script, args)
	return err
}

// Scroll performs a deterministic scroll in one of up/down/left/right.
// The distance is expressed as pixel count to move.
func (g *Gestures) Scroll(ctx context.Context, direction string, distance int) error {
	if !validDirection(direction) {
		return fmt.Errorf("scroll: invalid direction %q", direction)
	}
	script := "mobile: scrollGesture"
	args := map[string]any{"direction": direction, "percent": percentFromDistance(distance)}
	if g.plat == PlatformIOS {
		script = "mobile: scroll"
		args = map[string]any{"direction": direction}
	}
	_, err := g.client.ExecuteScript(ctx, script, args)
	return err
}

// Pinch performs a pinch in or out at the given centre. scale > 1 is
// pinch-out (zoom in), scale < 1 is pinch-in (zoom out).
func (g *Gestures) Pinch(ctx context.Context, x, y int, scale float64) error {
	if scale <= 0 {
		return fmt.Errorf("pinch: scale must be > 0, got %f", scale)
	}
	script := "mobile: pinchCloseGesture"
	if scale > 1 {
		script = "mobile: pinchOpenGesture"
	}
	args := map[string]any{"x": x, "y": y, "percent": pinchPercent(scale)}
	if g.plat == PlatformIOS {
		script = "mobile: pinch"
		args = map[string]any{"scale": scale, "velocity": 1.0}
	}
	_, err := g.client.ExecuteScript(ctx, script, args)
	return err
}

// Rotate rotates the device orientation between "portrait" and "landscape".
func (g *Gestures) Rotate(ctx context.Context, orientation string) error {
	switch orientation {
	case "portrait", "landscape":
	default:
		return fmt.Errorf("rotate: invalid orientation %q", orientation)
	}
	_, err := g.client.ExecuteScript(ctx, "mobile: setOrientation", map[string]any{"orientation": orientation})
	return err
}

// Key presses a hardware button (Android) or simulates a key (iOS). Named
// keys are "home", "back", "volume_up", "volume_down", "power", "menu".
func (g *Gestures) Key(ctx context.Context, key string) error {
	script := "mobile: pressKey"
	args := map[string]any{"keycode": androidKeyCode(key)}
	if g.plat == PlatformIOS {
		script = "mobile: pressButton"
		args = map[string]any{"name": iOSButtonName(key)}
	}
	_, err := g.client.ExecuteScript(ctx, script, args)
	return err
}

func validDirection(d string) bool {
	switch d {
	case "up", "down", "left", "right":
		return true
	}
	return false
}

func gestureSpeedFromDuration(ms int) int {
	if ms <= 0 {
		return 1000
	}
	// simple inverse mapping; longer duration = lower speed
	return 1000 * 1000 / ms
}

func percentFromDistance(distance int) float64 {
	if distance <= 0 {
		return 0.5
	}
	p := float64(distance) / 2000.0
	if p > 1 {
		return 1
	}
	return p
}

func pinchPercent(scale float64) float64 {
	// Normalise scale delta to a 0.1..1.0 percent field expected by
	// UiAutomator2 pinch gestures.
	d := scale - 1
	if d < 0 {
		d = -d
	}
	if d > 1 {
		return 1
	}
	if d < 0.1 {
		return 0.1
	}
	return d
}

func androidKeyCode(key string) int {
	switch key {
	case "home":
		return 3
	case "back":
		return 4
	case "menu":
		return 82
	case "volume_up":
		return 24
	case "volume_down":
		return 25
	case "power":
		return 26
	case "enter":
		return 66
	case "tab":
		return 61
	}
	return 0
}

func iOSButtonName(key string) string {
	switch key {
	case "home":
		return "home"
	case "volume_up":
		return "volumeUp"
	case "volume_down":
		return "volumeDown"
	}
	return key
}
