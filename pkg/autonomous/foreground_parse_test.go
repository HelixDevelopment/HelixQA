// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package autonomous

import "testing"

// realDumpsysOutput reproduces the exact shape of
// `adb shell dumpsys window windows` on Android 9 / SDK 28
// (Xiaomi Mi Box 4). It is the minimum fixture needed to
// regress FIX-QA-2026-04-21-019 part 3 — an earlier stub
// version of extractLine() returned the whole text, which
// made currentForegroundPackage() pluck the first `{...}`
// block it found (the InputMethod window) and mistake the
// on-screen keyboard's package for the foreground app.
const realDumpsysOutput = `WINDOW MANAGER WINDOWS (dumpsys window windows)
  Window #0 Window{5a26fd7 u0 InputMethod}:
    mDisplayId=0 stackId=0 mSession=Session{a279b18 7726:u0a10059} mClient=android.os.BinderProxy@6972356
    mOwnerUid=10059 mShowToOwnerOnly=true package=com.google.android.inputmethod.latin appop=NONE
    mAttrs={(0,0)(fillxfill) gr=BOTTOM CENTER_VERTICAL sim={adjust=pan} ty=INPUT_METHOD fmt=TRANSPARENT wanim=0x1030056
  Window #1 Window{54be45c u0 com.catalogizer.androidtv/com.catalogizer.androidtv.ui.MainActivity}:
    mDisplayId=0 stackId=1 mSession=Session{0000000 1234:u0a10100} mClient=android.os.BinderProxy@abc
    mOwnerUid=10100 mShowToOwnerOnly=false package=com.catalogizer.androidtv appop=NONE
  mCurrentFocus=Window{54be45c u0 com.catalogizer.androidtv/com.catalogizer.androidtv.ui.MainActivity}
  mFocusedApp=AppWindowToken{abc token=Token{def ActivityRecord{123 u0 com.catalogizer.androidtv/.ui.MainActivity t605}}}
`

const driftedDumpsysOutput = `WINDOW MANAGER WINDOWS (dumpsys window windows)
  Window #0 Window{5a26fd7 u0 InputMethod}:
    package=com.google.android.inputmethod.latin
  Window #1 Window{a7d42fd u0 ru.rutube.app/ru.rutube.app.MainActivity}:
    package=ru.rutube.app
  mCurrentFocus=Window{a7d42fd u0 ru.rutube.app/ru.rutube.app.MainActivity}
`

const launcherDumpsysOutput = `WINDOW MANAGER WINDOWS (dumpsys window windows)
  Window #0 Window{5a26fd7 u0 InputMethod}:
    package=com.google.android.inputmethod.latin
  mCurrentFocus=Window{7fa112 u0 com.mitv.tvhome.atv/com.mitv.tvhome.atv.MainActivity}
`

func TestExtractLine_FindsMCurrentFocus(t *testing.T) {
	line := extractLine(realDumpsysOutput, "mCurrentFocus=")
	want := "mCurrentFocus=Window{54be45c u0 com.catalogizer.androidtv/com.catalogizer.androidtv.ui.MainActivity}"
	if line != want {
		t.Fatalf("extractLine returned %q, want %q", line, want)
	}
}

func TestExtractLine_NoMatch(t *testing.T) {
	if got := extractLine(realDumpsysOutput, "mDoesNotExist="); got != "" {
		t.Fatalf("expected empty string for missing prefix, got %q", got)
	}
}

func TestCurrentForegroundPackage_Real(t *testing.T) {
	got := currentForegroundPackage(realDumpsysOutput)
	want := "com.catalogizer.androidtv"
	if got != want {
		t.Fatalf("currentForegroundPackage = %q, want %q", got, want)
	}
}

func TestCurrentForegroundPackage_Drifted(t *testing.T) {
	got := currentForegroundPackage(driftedDumpsysOutput)
	want := "ru.rutube.app"
	if got != want {
		t.Fatalf("currentForegroundPackage = %q, want %q", got, want)
	}
}

func TestCurrentForegroundPackage_Launcher(t *testing.T) {
	got := currentForegroundPackage(launcherDumpsysOutput)
	want := "com.mitv.tvhome.atv"
	if got != want {
		t.Fatalf("currentForegroundPackage = %q, want %q", got, want)
	}
	if !isLauncherPackage(got) {
		t.Fatalf("isLauncherPackage(%q) = false, want true", got)
	}
}

func TestIsLauncherPackage(t *testing.T) {
	cases := []struct {
		pkg  string
		want bool
	}{
		{"com.mitv.tvhome.atv", true},
		{"com.mitv.tvhome.michannel", true},
		{"com.google.android.tvlauncher", true},
		{"com.google.android.leanbacklauncher", true},
		{"com.amazon.tv.launcher", true},
		{"com.android.tv.launcher", true},
		{"com.catalogizer.androidtv", false},
		{"ru.rutube.app", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isLauncherPackage(c.pkg); got != c.want {
			t.Errorf("isLauncherPackage(%q) = %v, want %v", c.pkg, got, c.want)
		}
	}
}
