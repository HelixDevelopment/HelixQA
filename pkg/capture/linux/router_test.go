// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package linux

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"digital.vasic.helixqa/pkg/capture/frames"
)

func TestBackend_String(t *testing.T) {
	cases := map[Backend]string{
		BackendAuto:    "auto",
		BackendPortal:  "portal",
		BackendKMSGrab: "kmsgrab",
		BackendX11Grab: "x11grab",
	}
	for b, want := range cases {
		if got := b.String(); got != want {
			t.Errorf("Backend(%d).String() = %q, want %q", b, got, want)
		}
	}
}

func TestParseBackend(t *testing.T) {
	cases := map[string]Backend{
		"":         BackendAuto,
		"  ":       BackendAuto,
		"portal":   BackendPortal,
		"PIPEWIRE": BackendPortal,
		"kmsgrab":  BackendKMSGrab,
		"kms":      BackendKMSGrab,
		"x11":      BackendX11Grab,
		"x11grab":  BackendX11Grab,
		"bogus":    BackendAuto,
	}
	for s, want := range cases {
		if got := ParseBackend(s); got != want {
			t.Errorf("ParseBackend(%q) = %v, want %v", s, got, want)
		}
	}
}

func envMap(pairs map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) {
		v, ok := pairs[k]
		return v, ok
	}
}

func TestResolveBackend_Precedence(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		want Backend
	}{
		{
			"override-wins-over-env",
			Config{
				BackendOverride: BackendKMSGrab,
				LookupEnv:       envMap(map[string]string{"HELIX_LINUX_CAPTURE": "portal"}),
			},
			BackendKMSGrab,
		},
		{
			"env-override-wins-over-session-type",
			Config{
				LookupEnv: envMap(map[string]string{
					"HELIX_LINUX_CAPTURE": "kmsgrab",
					"XDG_SESSION_TYPE":    "wayland",
				}),
			},
			BackendKMSGrab,
		},
		{
			"wayland-session-picks-portal",
			Config{LookupEnv: envMap(map[string]string{"XDG_SESSION_TYPE": "wayland"})},
			BackendPortal,
		},
		{
			"x11-session-picks-x11grab",
			Config{LookupEnv: envMap(map[string]string{"XDG_SESSION_TYPE": "x11"})},
			BackendX11Grab,
		},
		{
			"tty-session-picks-x11grab",
			Config{LookupEnv: envMap(map[string]string{"XDG_SESSION_TYPE": "tty"})},
			BackendX11Grab,
		},
		{
			"no-env-defaults-to-portal",
			Config{LookupEnv: envMap(nil)},
			BackendPortal,
		},
		{
			"unknown-helix-capture-falls-through",
			Config{LookupEnv: envMap(map[string]string{
				"HELIX_LINUX_CAPTURE": "tiger",
				"XDG_SESSION_TYPE":    "wayland",
			})},
			BackendPortal,
		},
		{
			"unknown-session-type-defaults-to-portal",
			Config{LookupEnv: envMap(map[string]string{"XDG_SESSION_TYPE": "mir"})},
			BackendPortal,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveBackend(tc.cfg)
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestResolveBackend_UsesRealEnvWhenLookupNil(t *testing.T) {
	// Just exercise the os.LookupEnv fallback path — the real env on CI is
	// typically unset, so we expect BackendPortal (the safe default).
	t.Setenv("HELIX_LINUX_CAPTURE", "")
	t.Setenv("XDG_SESSION_TYPE", "")
	got := ResolveBackend(Config{})
	if got != BackendPortal {
		t.Errorf("default resolve = %v, want %v", got, BackendPortal)
	}
}

// --- NewSource dispatch ---

func mkFakeSource() Source {
	fr := &fakeRunner{stdoutFeed: bytes.NewReader(nil), stderrFeed: bytes.NewReader(nil)}
	r, _ := NewSidecarRunner(SidecarConfig{
		Binary: "x", Source: "y", Width: 1, Height: 1,
		Format: frames.FormatNV12, Runner: fr,
	})
	return WrapSidecarAsSource(r, BackendPortal)
}

func TestNewSource_CallsChosenFactory(t *testing.T) {
	invoked := map[Backend]int{}
	mkFactory := func(b Backend) BackendFactory {
		return func(Config) (Source, error) {
			invoked[b]++
			return &sidecarSource{backend: b}, nil
		}
	}
	cfg := Config{
		Width: 1920, Height: 1080,
		LookupEnv:      envMap(map[string]string{"HELIX_LINUX_CAPTURE": "portal"}),
		PortalFactory:  mkFactory(BackendPortal),
		KMSGrabFactory: mkFactory(BackendKMSGrab),
		X11GrabFactory: mkFactory(BackendX11Grab),
	}
	src, err := NewSource(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if src.Backend() != BackendPortal {
		t.Errorf("got %v, want portal", src.Backend())
	}
	if invoked[BackendPortal] != 1 || invoked[BackendKMSGrab] != 0 || invoked[BackendX11Grab] != 0 {
		t.Errorf("wrong factory invocations: %v", invoked)
	}
}

func TestNewSource_UnwiredFactory(t *testing.T) {
	cfg := Config{
		Width: 1920, Height: 1080,
		BackendOverride: BackendKMSGrab,
		// no KMSGrabFactory
	}
	_, err := NewSource(cfg)
	if !errors.Is(err, ErrUnsupportedBackend) {
		t.Errorf("want ErrUnsupportedBackend, got %v", err)
	}
}

func TestNewSource_BadDimensions(t *testing.T) {
	_, err := NewSource(Config{Width: 0, Height: 1080})
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("want ErrInvalidConfig, got %v", err)
	}
}

func TestNewSource_FactoryError(t *testing.T) {
	boom := errors.New("nope")
	cfg := Config{
		Width: 1, Height: 1,
		BackendOverride: BackendPortal,
		PortalFactory: func(Config) (Source, error) {
			return nil, boom
		},
	}
	_, err := NewSource(cfg)
	if !errors.Is(err, boom) {
		t.Errorf("want boom, got %v", err)
	}
}

// --- WrapSidecarAsSource integration ---

func TestWrapSidecarAsSource(t *testing.T) {
	var buf bytes.Buffer
	buf.Write(EncodeEnvelope(1_000_000, []byte{0x99}))
	fr := &fakeRunner{stdoutFeed: &buf, stderrFeed: bytes.NewReader(nil)}
	r, err := NewSidecarRunner(SidecarConfig{
		Binary: "x", Source: "portal", Width: 100, Height: 100,
		Format: frames.FormatH264AnnexB, Runner: fr,
	})
	if err != nil {
		t.Fatal(err)
	}
	src := WrapSidecarAsSource(r, BackendPortal)
	if src.Backend() != BackendPortal {
		t.Errorf("Backend = %v", src.Backend())
	}
	if err := src.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = src.Stop() })
	got := <-src.Frames()
	if got.Source != "portal" || got.Width != 100 {
		t.Errorf("frame wrong: %+v", got)
	}
}
