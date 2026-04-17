package mobile

import (
	"context"
	"strings"
	"testing"
)

func TestGestures_TapDispatchesPerPlatform(t *testing.T) {
	c, _, rec := fakeAppium(t)
	_ = c.NewSession(context.Background(), NewAndroidCaps("Pixel", "com.x", ".M"))
	g := NewGestures(c, PlatformAndroid)
	if err := g.Tap(context.Background(), 10, 20); err != nil {
		t.Fatal(err)
	}
	if !lastBodyContains(*rec, "clickGesture") {
		t.Errorf("android tap should dispatch clickGesture, got %+v", *rec)
	}

	gi := NewGestures(c, PlatformIOS)
	if err := gi.Tap(context.Background(), 30, 40); err != nil {
		t.Fatal(err)
	}
	if !lastBodyContains(*rec, `mobile: tap`) {
		t.Errorf("ios tap should dispatch mobile: tap")
	}
}

func TestGestures_SwipeEncodesDuration(t *testing.T) {
	c, _, rec := fakeAppium(t)
	_ = c.NewSession(context.Background(), NewAndroidCaps("Pixel", "com.x", ".M"))
	g := NewGestures(c, PlatformAndroid)
	if err := g.Swipe(context.Background(), 100, 200, 100, 50, 500); err != nil {
		t.Fatal(err)
	}
	if !lastBodyContains(*rec, "swipeGesture") {
		t.Error("swipe not dispatched")
	}
}

func TestGestures_ScrollValidatesDirection(t *testing.T) {
	c, _, _ := fakeAppium(t)
	_ = c.NewSession(context.Background(), NewAndroidCaps("Pixel", "com.x", ".M"))
	g := NewGestures(c, PlatformAndroid)
	if err := g.Scroll(context.Background(), "diagonal", 100); err == nil {
		t.Fatal("diagonal should be rejected")
	}
	if err := g.Scroll(context.Background(), "down", 100); err != nil {
		t.Fatal(err)
	}
}

func TestGestures_PinchRejectsNegativeScale(t *testing.T) {
	c, _, _ := fakeAppium(t)
	_ = c.NewSession(context.Background(), NewAndroidCaps("Pixel", "com.x", ".M"))
	g := NewGestures(c, PlatformAndroid)
	if err := g.Pinch(context.Background(), 10, 10, 0); err == nil {
		t.Fatal("expected rejection for scale=0")
	}
	if err := g.Pinch(context.Background(), 10, 10, 1.5); err != nil {
		t.Fatal(err)
	}
}

func TestGestures_RotateValidatesOrientation(t *testing.T) {
	c, _, _ := fakeAppium(t)
	_ = c.NewSession(context.Background(), NewAndroidCaps("Pixel", "com.x", ".M"))
	g := NewGestures(c, PlatformAndroid)
	if err := g.Rotate(context.Background(), "tilted"); err == nil {
		t.Fatal("tilted should be rejected")
	}
	if err := g.Rotate(context.Background(), "portrait"); err != nil {
		t.Fatal(err)
	}
}

func TestGestures_KeyMapping(t *testing.T) {
	c, _, rec := fakeAppium(t)
	_ = c.NewSession(context.Background(), NewAndroidCaps("Pixel", "com.x", ".M"))
	g := NewGestures(c, PlatformAndroid)
	if err := g.Key(context.Background(), "home"); err != nil {
		t.Fatal(err)
	}
	if !lastBodyContains(*rec, `"keycode":3`) {
		t.Errorf("home should map to keycode 3, got %+v", *rec)
	}
}

func TestAndroidKeyCode_Known(t *testing.T) {
	cases := map[string]int{
		"home": 3, "back": 4, "menu": 82,
		"volume_up": 24, "volume_down": 25,
		"power": 26, "enter": 66, "tab": 61,
	}
	for k, v := range cases {
		if got := androidKeyCode(k); got != v {
			t.Errorf("androidKeyCode(%q) = %d, want %d", k, got, v)
		}
	}
	if androidKeyCode("unknown") != 0 {
		t.Error("unknown key should return 0")
	}
}

func lastBodyContains(r []recordedRequest, needle string) bool {
	for i := len(r) - 1; i >= 0; i-- {
		if strings.Contains(r[i].Body, needle) {
			return true
		}
	}
	return false
}
