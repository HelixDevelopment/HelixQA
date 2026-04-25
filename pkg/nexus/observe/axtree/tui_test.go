// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package axtree

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// StripANSI
// ---------------------------------------------------------------------------

func TestStripANSI_PlainTextPassesThrough(t *testing.T) {
	in := []byte("hello world\nline two\n")
	out := StripANSI(in)
	if string(out) != "hello world\nline two\n" {
		t.Fatalf("plain text = %q", out)
	}
}

func TestStripANSI_CSIColorsStripped(t *testing.T) {
	// \x1b[31m = red, \x1b[0m = reset.
	in := []byte("\x1b[31mhello\x1b[0m world")
	out := StripANSI(in)
	if string(out) != "hello world" {
		t.Fatalf("CSI stripped = %q", out)
	}
}

func TestStripANSI_OSCTitleStripped(t *testing.T) {
	// \x1b]0;window title\x07 = OSC set title.
	in := []byte("\x1b]0;My Window\x07visible text")
	out := StripANSI(in)
	if string(out) != "visible text" {
		t.Fatalf("OSC stripped = %q", out)
	}
}

func TestStripANSI_OSCWithSTTerminator(t *testing.T) {
	// \x1b]8;;http://x\x1b\\link\x1b]8;;\x1b\\ = hyperlink.
	in := []byte("\x1b]8;;http://x\x1b\\link\x1b]8;;\x1b\\")
	out := StripANSI(in)
	if string(out) != "link" {
		t.Fatalf("OSC with ST = %q", out)
	}
}

func TestStripANSI_BackspaceOvertype(t *testing.T) {
	// "x\x08x" → "x" (man-style bold).
	in := []byte("x\x08x")
	out := StripANSI(in)
	if string(out) != "x" {
		t.Fatalf("backspace = %q", out)
	}
}

func TestStripANSI_SingleCharEscape(t *testing.T) {
	// \x1bM = reverse linefeed. Should be skipped entirely.
	in := []byte("before\x1bMafter")
	out := StripANSI(in)
	if string(out) != "beforeafter" {
		t.Fatalf("single-char escape = %q", out)
	}
}

func TestStripANSI_UnterminatedCSI(t *testing.T) {
	// Malformed: CSI with no final byte. Should consume to EOF
	// without panicking.
	in := []byte("\x1b[31;1")
	out := StripANSI(in)
	if string(out) != "" {
		t.Fatalf("unterminated CSI = %q", out)
	}
}

func TestStripANSI_UnterminatedOSC(t *testing.T) {
	in := []byte("\x1b]0;title-no-terminator")
	out := StripANSI(in)
	if string(out) != "" {
		t.Fatalf("unterminated OSC = %q", out)
	}
}

// ---------------------------------------------------------------------------
// TUISnapshotter — happy path
// ---------------------------------------------------------------------------

type fakeTUIFetcher struct {
	data    []byte
	err     error
	closeN  int
}

func (f *fakeTUIFetcher) Fetch(ctx context.Context) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.data, nil
}

func (f *fakeTUIFetcher) Close() error { f.closeN++; return nil }

func TestTUISnapshotter_Snapshot_FullTree(t *testing.T) {
	raw := []byte("line one\n\x1b[31mline two\x1b[0m\nline three\n")
	snap := NewTUISnapshotterWithFetcher(&fakeTUIFetcher{data: raw})
	defer snap.Close()
	root, err := snap.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if root.Platform != PlatformTUI {
		t.Fatalf("Platform = %q, want %q", root.Platform, PlatformTUI)
	}
	if root.Role != "application" || root.Name != "terminal" {
		t.Fatalf("root = (%q, %q)", root.Role, root.Name)
	}
	// Three non-empty rows → three children.
	if len(root.Children) != 3 {
		t.Fatalf("children = %d, want 3", len(root.Children))
	}
	// Middle row had CSI colors — should be stripped.
	if root.Children[1].Name != "line two" {
		t.Fatalf("row 1 name = %q, want 'line two'", root.Children[1].Name)
	}
	// Each row has Role=text + Enabled=true.
	for i, c := range root.Children {
		if c.Role != "text" {
			t.Errorf("row %d role = %q", i, c.Role)
		}
		if !c.Enabled {
			t.Errorf("row %d not Enabled", i)
		}
	}
}

func TestTUISnapshotter_Snapshot_SkipsEmptyRows(t *testing.T) {
	raw := []byte("line one\n\n   \nline four\n")
	snap := NewTUISnapshotterWithFetcher(&fakeTUIFetcher{data: raw})
	defer snap.Close()
	root, _ := snap.Snapshot(context.Background())
	// Empty + whitespace-only rows should be skipped.
	if len(root.Children) != 2 {
		t.Fatalf("children = %d, want 2 (empty rows skipped)", len(root.Children))
	}
	if root.Children[0].Name != "line one" || root.Children[1].Name != "line four" {
		t.Errorf("names = %q, %q", root.Children[0].Name, root.Children[1].Name)
	}
}

func TestTUISnapshotter_Snapshot_RowBoundsInCharCoords(t *testing.T) {
	raw := []byte("hello\n\nworld\n")
	snap := NewTUISnapshotterWithFetcher(&fakeTUIFetcher{data: raw})
	defer snap.Close()
	root, _ := snap.Snapshot(context.Background())
	// Row 0: "hello" → bounds (0, 0)-(5, 1).
	if root.Children[0].Bounds.Min.X != 0 || root.Children[0].Bounds.Min.Y != 0 {
		t.Fatalf("row 0 bounds = %v", root.Children[0].Bounds)
	}
	if root.Children[0].Bounds.Max.X != 5 || root.Children[0].Bounds.Max.Y != 1 {
		t.Fatalf("row 0 bounds = %v", root.Children[0].Bounds)
	}
	// Row "world" is at line index 2 (0-indexed), not 1 — because
	// the blank line 1 occupies its own Y position.
	if root.Children[1].Bounds.Min.Y != 2 {
		t.Fatalf("row 2 Y = %d, want 2", root.Children[1].Bounds.Min.Y)
	}
}

func TestTUISnapshotter_Snapshot_CustomDimensions(t *testing.T) {
	// Explicit Cols + Rows override auto-detection.
	raw := []byte("short\n")
	snap := NewTUISnapshotterWithFetcher(&fakeTUIFetcher{data: raw})
	snap.Cols = 80
	snap.Rows = 25
	defer snap.Close()
	root, _ := snap.Snapshot(context.Background())
	// Root bounds should use configured dimensions.
	if root.Bounds.Max.X != 80 || root.Bounds.Max.Y != 25 {
		t.Fatalf("root bounds = %v, want 0,0-80,25", root.Bounds)
	}
}

// ---------------------------------------------------------------------------
// Error paths
// ---------------------------------------------------------------------------

func TestTUISnapshotter_Snapshot_EmptyBytesError(t *testing.T) {
	snap := NewTUISnapshotterWithFetcher(&fakeTUIFetcher{data: []byte{}})
	_, err := snap.Snapshot(context.Background())
	if !errors.Is(err, ErrNoRoot) {
		t.Fatalf("empty bytes: %v, want ErrNoRoot", err)
	}
}

func TestTUISnapshotter_Snapshot_FetcherError(t *testing.T) {
	snap := NewTUISnapshotterWithFetcher(&fakeTUIFetcher{err: errors.New("read failed")})
	_, err := snap.Snapshot(context.Background())
	if err == nil {
		t.Fatal("fetcher error should propagate")
	}
}

func TestTUISnapshotter_Snapshot_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	snap := NewTUISnapshotterWithFetcher(&fakeTUIFetcher{data: []byte("x\n")})
	if _, err := snap.Snapshot(ctx); err == nil {
		t.Fatal("canceled ctx should fail")
	}
}

func TestTUISnapshotter_Close_Idempotent(t *testing.T) {
	f := &fakeTUIFetcher{data: []byte("x\n")}
	snap := NewTUISnapshotterWithFetcher(f)
	snap.Close()
	snap.Close()
	if f.closeN != 1 {
		t.Fatalf("fetcher.Close called %d times, want 1", f.closeN)
	}
}

func TestTUISnapshotter_Snapshot_AfterCloseReturnsError(t *testing.T) {
	snap := NewTUISnapshotterWithFetcher(&fakeTUIFetcher{data: []byte("x\n")})
	snap.Close()
	if _, err := snap.Snapshot(context.Background()); err != ErrSnapshotClosed {
		t.Fatalf("post-Close: %v, want ErrSnapshotClosed", err)
	}
}

// ---------------------------------------------------------------------------
// TUIStdinFetcher
// ---------------------------------------------------------------------------

func TestTUIStdinFetcher_ReadsFromReader(t *testing.T) {
	r := bytes.NewReader([]byte("hello\nworld\n"))
	f := &TUIStdinFetcher{Reader: r}
	data, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if string(data) != "hello\nworld\n" {
		t.Fatalf("data = %q", data)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestTUIStdinFetcher_NilReaderError(t *testing.T) {
	f := &TUIStdinFetcher{}
	if _, err := f.Fetch(context.Background()); err == nil {
		t.Fatal("nil Reader must error")
	}
}

func TestTUIStdinFetcher_ContextCanceled(t *testing.T) {
	r := bytes.NewReader([]byte("x"))
	f := &TUIStdinFetcher{Reader: r}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := f.Fetch(ctx); err == nil {
		t.Fatal("canceled ctx should fail")
	}
}

func TestTUIStdinFetcher_CustomMaxBytes(t *testing.T) {
	// MaxBytes=5 reads only the first 5 bytes.
	r := bytes.NewReader([]byte("0123456789"))
	f := &TUIStdinFetcher{Reader: r, MaxBytes: 5}
	data, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "01234" {
		t.Fatalf("data = %q, want '01234'", data)
	}
}

// ---------------------------------------------------------------------------
// ParseGrid
// ---------------------------------------------------------------------------

func TestParseGrid_SimpleText(t *testing.T) {
	in := []byte("abc\ndef\n")
	g := ParseGrid(in)
	if g.Cols != 3 || g.Rows != 2 {
		t.Fatalf("grid = %d×%d, want 3×2", g.Cols, g.Rows)
	}
	if g.At(0, 0).Ch != 'a' || g.At(2, 1).Ch != 'f' {
		t.Fatalf("grid contents wrong")
	}
}

func TestParseGrid_SGRColors(t *testing.T) {
	// \x1b[31m = red foreground (31 - 30 = 1).
	in := []byte("\x1b[31mX\x1b[0mY")
	g := ParseGrid(in)
	if g.At(0, 0).Foreground != 1 {
		t.Errorf("cell 0 fg = %d, want 1 (red)", g.At(0, 0).Foreground)
	}
	if g.At(1, 0).Foreground != 255 {
		t.Errorf("cell 1 fg = %d, want 255 (default after reset)", g.At(1, 0).Foreground)
	}
}

func TestParseGrid_BoldAndUnderline(t *testing.T) {
	// \x1b[1m = bold on; \x1b[4m = underline on; \x1b[22m = bold off.
	in := []byte("\x1b[1;4mbold\x1b[22mplain")
	g := ParseGrid(in)
	if !g.At(0, 0).Bold || !g.At(0, 0).Underline {
		t.Errorf("cell 0: bold=%v underline=%v, want both true", g.At(0, 0).Bold, g.At(0, 0).Underline)
	}
	if g.At(4, 0).Bold {
		t.Errorf("cell 4: bold = true, want false (after \\x1b[22m)")
	}
	if !g.At(4, 0).Underline {
		t.Errorf("cell 4: underline = false, want true (persists)")
	}
}

func TestParseGrid_BackgroundColor(t *testing.T) {
	// \x1b[44m = blue background (44 - 40 = 4).
	in := []byte("\x1b[44mX\x1b[49mY")
	g := ParseGrid(in)
	if g.At(0, 0).Background != 4 {
		t.Errorf("cell 0 bg = %d, want 4", g.At(0, 0).Background)
	}
	if g.At(1, 0).Background != 255 {
		t.Errorf("cell 1 bg = %d, want 255 (default)", g.At(1, 0).Background)
	}
}

func TestParseGrid_BrightColors(t *testing.T) {
	// \x1b[91m = bright red foreground (9 internally).
	in := []byte("\x1b[91mX")
	g := ParseGrid(in)
	if g.At(0, 0).Foreground != 9 {
		t.Errorf("bright fg = %d, want 9", g.At(0, 0).Foreground)
	}
	// \x1b[104m = bright blue background.
	in2 := []byte("\x1b[104mY")
	g2 := ParseGrid(in2)
	if g2.At(0, 0).Background != 12 {
		t.Errorf("bright bg = %d, want 12", g2.At(0, 0).Background)
	}
}

func TestParseGrid_CursorForwardBackward(t *testing.T) {
	// "ab" + CUF(2) + "cd" should leave a gap at cols 2-3.
	in := []byte("ab\x1b[2Ccd")
	g := ParseGrid(in)
	// Row has 6 visible chars after CUF: a, b, ' ', ' ', c, d.
	if g.At(0, 0).Ch != 'a' || g.At(1, 0).Ch != 'b' {
		t.Errorf("ab cells wrong")
	}
	if g.At(4, 0).Ch != 'c' || g.At(5, 0).Ch != 'd' {
		t.Errorf("cd cells wrong: got '%c' '%c'", g.At(4, 0).Ch, g.At(5, 0).Ch)
	}
}

func TestParseGrid_CursorHorizontalAbsolute(t *testing.T) {
	// "abc" + CHA(1) + "X" → "Xbc" (X overtypes position 0).
	in := []byte("abc\x1b[1GX")
	g := ParseGrid(in)
	if g.At(0, 0).Ch != 'X' {
		t.Errorf("CHA overtype failed: got %c", g.At(0, 0).Ch)
	}
}

func TestParseGrid_EmptyInput(t *testing.T) {
	g := ParseGrid([]byte(""))
	if g.Cols != 0 || g.Rows != 0 {
		t.Fatalf("empty grid = %d×%d, want 0×0", g.Cols, g.Rows)
	}
}

func TestParseGrid_AtOutOfRange(t *testing.T) {
	g := ParseGrid([]byte("abc\n"))
	// Out-of-range returns default space cell.
	if got := g.At(-1, 0); got.Ch != ' ' {
		t.Errorf("negative col = %+v", got)
	}
	if got := g.At(0, -1); got.Ch != ' ' {
		t.Errorf("negative row = %+v", got)
	}
	if got := g.At(100, 100); got.Ch != ' ' {
		t.Errorf("out of range = %+v", got)
	}
}

// ---------------------------------------------------------------------------
// Constructor + interface conformance
// ---------------------------------------------------------------------------

func TestNewTUISnapshotter_UsesStdinFetcher(t *testing.T) {
	r := strings.NewReader("test\n")
	s := NewTUISnapshotter(r)
	f, ok := s.fetcher.(*TUIStdinFetcher)
	if !ok {
		t.Fatalf("fetcher type = %T, want *TUIStdinFetcher", s.fetcher)
	}
	if f.Reader != r {
		t.Fatal("Reader not wired")
	}
}

func TestTUISnapshotter_SatisfiesSnapshotterInterface(t *testing.T) {
	var s Snapshotter = NewTUISnapshotterWithFetcher(&fakeTUIFetcher{data: []byte("x\n")})
	defer s.Close()
	if _, err := s.Snapshot(context.Background()); err != nil {
		t.Fatalf("Snapshot via interface: %v", err)
	}
}
