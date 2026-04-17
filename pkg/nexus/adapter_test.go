package nexus

import "testing"

func TestPlatformConstants_Unique(t *testing.T) {
	seen := map[Platform]bool{}
	all := []Platform{
		PlatformWebChromedp, PlatformWebRod, PlatformWebPlaywright,
		PlatformAndroidAppium, PlatformAndroidTVAppium, PlatformIOSAppium,
		PlatformDesktopWindows, PlatformDesktopMacOS, PlatformDesktopLinux,
	}
	for _, p := range all {
		if seen[p] {
			t.Errorf("duplicate platform constant %q", p)
		}
		seen[p] = true
	}
}

func TestElementRef_StringType(t *testing.T) {
	var r ElementRef = "e12"
	if string(r) != "e12" {
		t.Errorf("ElementRef must stringify verbatim, got %q", string(r))
	}
}

func TestRect_ZeroValueUsable(t *testing.T) {
	var r Rect
	if r.X != 0 || r.Y != 0 || r.Width != 0 || r.Height != 0 {
		t.Errorf("Rect zero value must be usable")
	}
}
