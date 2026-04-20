// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package axtree

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"
)

// TUIFetcher reads a raw ANSI-encoded terminal screen dump. The
// default implementation attaches to a running pty via a command
// like `tmux capture-pane -p` or `script /dev/null`; tests inject
// a fake that returns canned byte blobs.
//
// Returned bytes should be the live screen contents at the moment
// of capture, not an entire scrollback — TUI axtree is a SCREEN
// abstraction (what the user sees now), not a history browser.
type TUIFetcher interface {
	Fetch(ctx context.Context) ([]byte, error)
	Close() error
}

// TUIStdinFetcher is the production TUIFetcher — reads from a
// user-supplied io.Reader (typically connected to `tmux
// capture-pane -p <session>` stdout). HelixQA executors set this
// up before invoking Snapshot.
type TUIStdinFetcher struct {
	Reader io.Reader
	// MaxBytes caps a single read at this size. Default 64 KiB —
	// enough for any reasonable terminal screen.
	MaxBytes int
}

// Fetch reads up to MaxBytes from Reader.
func (f *TUIStdinFetcher) Fetch(ctx context.Context) ([]byte, error) {
	if f.Reader == nil {
		return nil, errors.New("axtree/tui: TUIStdinFetcher.Reader must be set")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	max := f.MaxBytes
	if max == 0 {
		max = 64 * 1024
	}
	buf := make([]byte, max)
	n, err := f.Reader.Read(buf)
	if err != nil && err != io.EOF && n == 0 {
		return nil, fmt.Errorf("%w: %v", ErrNotAvailable, err)
	}
	return buf[:n], nil
}

// Close is a no-op (Reader lifetime is caller-owned).
func (*TUIStdinFetcher) Close() error { return nil }

// TUISnapshotter walks a captured terminal screen via a TUIFetcher
// and emits a materialized *Node tree rooted at a "terminal" node.
// Children are the visible rows that contain non-whitespace
// content; each row is labeled with its visible text, stripping
// ANSI escape sequences.
type TUISnapshotter struct {
	fetcher TUIFetcher
	mu      sync.Mutex
	closed  bool

	// Cols + Rows cache the terminal dimensions. Zero → auto-
	// detect from the longest row and total row count in the
	// captured stream.
	Cols int
	Rows int
}

// NewTUISnapshotter builds a stdin-based snapshotter. For tests,
// NewTUISnapshotterWithFetcher injects a mock TUIFetcher.
func NewTUISnapshotter(r io.Reader) *TUISnapshotter {
	return &TUISnapshotter{fetcher: &TUIStdinFetcher{Reader: r}}
}

// NewTUISnapshotterWithFetcher wires a snapshotter to any
// TUIFetcher implementation.
func NewTUISnapshotterWithFetcher(f TUIFetcher) *TUISnapshotter {
	return &TUISnapshotter{fetcher: f}
}

// Snapshot captures the current terminal screen and converts it to
// a *Node tree.
//
// Output shape:
//
//	application "terminal" (root)
//	├── row "line 1 content" (one Node per non-empty row)
//	├── row "line 2 content"
//	└── ...
//
// Each row Node's RawID is "row:<line-number>" (0-indexed); its
// Bounds span the full row's (col_start, row, col_end, row+1) in
// CHARACTER coordinates, not pixels. Upstream layers that want
// pixel coords multiply by the font cell size.
func (s *TUISnapshotter) Snapshot(ctx context.Context) (*Node, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, ErrSnapshotClosed
	}
	s.mu.Unlock()

	raw, err := s.fetcher.Fetch(ctx)
	if err != nil {
		return nil, fmt.Errorf("axtree/tui: Fetch: %w", err)
	}
	if len(raw) == 0 {
		return nil, ErrNoRoot
	}
	plain := StripANSI(raw)
	lines := strings.Split(string(plain), "\n")

	root := &Node{
		Role:     "application",
		Name:     "terminal",
		Platform: PlatformTUI,
		RawID:    "terminal",
	}

	maxCol := 0
	for i, line := range lines {
		// Strip trailing whitespace (cursor-column fills, etc).
		stripped := strings.TrimRight(line, " \t\r\n")
		if stripped == "" {
			continue
		}
		if len(stripped) > maxCol {
			maxCol = len(stripped)
		}
		child := &Node{
			Role:     "text",
			Name:     stripped,
			Platform: PlatformTUI,
			RawID:    fmt.Sprintf("row:%d", i),
			Bounds:   image.Rect(0, i, len(stripped), i+1),
			Enabled:  true,
		}
		root.Children = append(root.Children, child)
	}

	// Fill in the root's bounds using detected dimensions.
	rows := len(lines)
	if s.Rows > 0 {
		rows = s.Rows
	}
	if s.Cols > 0 {
		maxCol = s.Cols
	}
	root.Bounds = image.Rect(0, 0, maxCol, rows)

	return root, nil
}

// Close releases the fetcher. Idempotent.
func (s *TUISnapshotter) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	return s.fetcher.Close()
}

// ---------------------------------------------------------------------------
// ANSI-escape stripping
// ---------------------------------------------------------------------------

// StripANSI removes every ANSI escape sequence from the input,
// leaving only the visible characters. Handles:
//
//   - CSI (Control Sequence Introducer) sequences:
//     ESC [ <params> <letter>  — colors, cursor motion, clears.
//   - OSC (Operating System Command) sequences:
//     ESC ] <params> BEL|ST    — window title, hyperlinks.
//   - Single-char escapes:
//     ESC M, ESC 7, ESC 8      — scroll / save-restore cursor.
//   - Sub-escape backspace that the terminal uses to overtype
//     (common in `man` output for bold/underline):
//     "x" + 0x08 + "x" → "x"
//
// The output is pure UTF-8 text suitable for row-by-row parsing.
func StripANSI(in []byte) []byte {
	var out bytes.Buffer
	i := 0
	for i < len(in) {
		b := in[i]
		if b == 0x1b && i+1 < len(in) {
			// ESC sequence.
			next := in[i+1]
			switch next {
			case '[':
				// CSI — consume until final byte in range @-~.
				j := i + 2
				for j < len(in) {
					c := in[j]
					if c >= 0x40 && c <= 0x7e {
						break
					}
					j++
				}
				if j < len(in) {
					i = j + 1 // skip final byte
				} else {
					i = j
				}
				continue
			case ']':
				// OSC — consume until BEL (0x07) or ST (ESC \).
				j := i + 2
				for j < len(in) {
					if in[j] == 0x07 {
						j++
						break
					}
					if in[j] == 0x1b && j+1 < len(in) && in[j+1] == '\\' {
						j += 2
						break
					}
					j++
				}
				i = j
				continue
			default:
				// Single-char escape (ESC c, ESC M, etc.).
				i += 2
				continue
			}
		}
		if b == 0x08 && out.Len() > 0 {
			// Backspace — consume both the BS and the next byte
			// (assumed to be the overtype character; the last
			// buffered byte is discarded).
			out.Truncate(out.Len() - 1)
			i++
			continue
		}
		// Printable or ordinary control char — preserve.
		out.WriteByte(b)
		i++
	}
	return out.Bytes()
}

// ---------------------------------------------------------------------------
// GridCell + ParseGrid — exposed for callers that want a full
// typed screen grid rather than just the row-list in Snapshot.
// ---------------------------------------------------------------------------

// GridCell is one character cell in a styled terminal grid.
type GridCell struct {
	Ch         rune
	Foreground uint8 // basic ANSI color code (0-15, 255 = default)
	Background uint8
	Bold       bool
	Underline  bool
}

// Grid is a 2-D matrix of styled cells.
type Grid struct {
	Cols  int
	Rows  int
	Cells []GridCell // row-major; len == Cols*Rows
}

// At returns the cell at (col, row), or a default GridCell if out
// of range.
func (g Grid) At(col, row int) GridCell {
	if col < 0 || col >= g.Cols || row < 0 || row >= g.Rows {
		return GridCell{Ch: ' ', Foreground: 255, Background: 255}
	}
	return g.Cells[row*g.Cols+col]
}

// ParseGrid parses an ANSI stream into a styled grid. The grid
// size is derived by a two-pass walk: pass 1 tracks virtual cursor
// motion (CUF / CUB / CHA) to find the max column the stream
// reaches; pass 2 renders. SGR parameters (color + bold /
// underline) are tracked in pass 2. Vertical cursor motion
// (CUU / CUD / position reset) is NOT handled — HelixQA's TUI
// targets emit plain top-to-bottom streams, and full cursor
// emulation would demand a complete terminal state machine
// (~1000 LoC).
func ParseGrid(in []byte) Grid {
	// Pass 1: walk with cursor tracking to find real dimensions.
	maxCol, rows := measureGrid(in)

	// Allocate grid + fill default.
	g := Grid{Cols: maxCol, Rows: rows, Cells: make([]GridCell, maxCol*rows)}
	defaultCell := GridCell{Ch: ' ', Foreground: 255, Background: 255}
	for i := range g.Cells {
		g.Cells[i] = defaultCell
	}

	// Second pass: walk the escape stream, writing styled runes.
	state := sgrState{fg: 255, bg: 255}
	col, row := 0, 0
	i := 0
	for i < len(in) && row < rows {
		b := in[i]
		if b == 0x1b && i+1 < len(in) && in[i+1] == '[' {
			// CSI — parse params + final byte.
			j := i + 2
			paramStart := j
			for j < len(in) {
				c := in[j]
				if c >= 0x40 && c <= 0x7e {
					break
				}
				j++
			}
			if j >= len(in) {
				i = j
				continue
			}
			paramStr := string(in[paramStart:j])
			final := in[j]
			switch final {
			case 'm':
				state.applySGR(paramStr)
			case 'C': // CUF — move cursor forward
				n := parseCSIN(paramStr, 1)
				col += n
				if col >= g.Cols {
					col = g.Cols - 1
				}
			case 'D': // CUB — cursor back
				n := parseCSIN(paramStr, 1)
				col -= n
				if col < 0 {
					col = 0
				}
			case 'G': // CHA — cursor horizontal absolute (1-based)
				n := parseCSIN(paramStr, 1)
				col = n - 1
				if col < 0 {
					col = 0
				}
				if col >= g.Cols {
					col = g.Cols - 1
				}
			}
			i = j + 1
			continue
		}
		if b == 0x1b && i+1 < len(in) && in[i+1] == ']' {
			// OSC — skip (same logic as StripANSI).
			j := i + 2
			for j < len(in) {
				if in[j] == 0x07 {
					j++
					break
				}
				if in[j] == 0x1b && j+1 < len(in) && in[j+1] == '\\' {
					j += 2
					break
				}
				j++
			}
			i = j
			continue
		}
		if b == 0x1b {
			i += 2
			continue
		}
		if b == '\n' {
			col = 0
			row++
			i++
			continue
		}
		if b == '\r' {
			col = 0
			i++
			continue
		}
		if b == 0x08 {
			if col > 0 {
				col--
			}
			i++
			continue
		}
		// Printable.
		if col < g.Cols && row < g.Rows {
			g.Cells[row*g.Cols+col] = GridCell{
				Ch:         rune(b),
				Foreground: state.fg,
				Background: state.bg,
				Bold:       state.bold,
				Underline:  state.underline,
			}
		}
		col++
		i++
	}
	return g
}

// measureGrid walks the ANSI stream once tracking virtual cursor
// position to find the true grid dimensions (cols = highest col
// reached, rows = count of lines that received content).
func measureGrid(in []byte) (cols, rows int) {
	col, row := 0, 0
	maxCol := 0
	maxRow := 0
	i := 0
	for i < len(in) {
		b := in[i]
		if b == 0x1b && i+1 < len(in) && in[i+1] == '[' {
			j := i + 2
			paramStart := j
			for j < len(in) {
				c := in[j]
				if c >= 0x40 && c <= 0x7e {
					break
				}
				j++
			}
			if j >= len(in) {
				i = j
				continue
			}
			paramStr := string(in[paramStart:j])
			final := in[j]
			switch final {
			case 'C': // CUF
				col += parseCSIN(paramStr, 1)
			case 'D': // CUB
				col -= parseCSIN(paramStr, 1)
				if col < 0 {
					col = 0
				}
			case 'G': // CHA (1-based)
				col = parseCSIN(paramStr, 1) - 1
				if col < 0 {
					col = 0
				}
			}
			i = j + 1
			continue
		}
		if b == 0x1b && i+1 < len(in) && in[i+1] == ']' {
			j := i + 2
			for j < len(in) {
				if in[j] == 0x07 {
					j++
					break
				}
				if in[j] == 0x1b && j+1 < len(in) && in[j+1] == '\\' {
					j += 2
					break
				}
				j++
			}
			i = j
			continue
		}
		if b == 0x1b {
			i += 2
			continue
		}
		if b == '\n' {
			if col > maxCol {
				maxCol = col
			}
			if row > maxRow {
				maxRow = row
			}
			col = 0
			row++
			i++
			continue
		}
		if b == '\r' {
			col = 0
			i++
			continue
		}
		if b == 0x08 {
			if col > 0 {
				col--
			}
			i++
			continue
		}
		// Printable — advances col.
		col++
		if col > maxCol {
			maxCol = col
		}
		if row+1 > maxRow {
			maxRow = row + 1
		}
		i++
	}
	return maxCol, maxRow
}

// sgrState tracks the current SGR attributes while parsing.
type sgrState struct {
	fg, bg    uint8
	bold      bool
	underline bool
}

// applySGR updates the state from a CSI-m parameter string
// ("0;31;1", etc.).
func (s *sgrState) applySGR(paramStr string) {
	if paramStr == "" {
		// Bare ESC[m = ESC[0m = reset.
		*s = sgrState{fg: 255, bg: 255}
		return
	}
	for _, p := range strings.Split(paramStr, ";") {
		n, err := strconv.Atoi(p)
		if err != nil {
			continue
		}
		switch {
		case n == 0:
			*s = sgrState{fg: 255, bg: 255}
		case n == 1:
			s.bold = true
		case n == 4:
			s.underline = true
		case n == 22:
			s.bold = false
		case n == 24:
			s.underline = false
		case n >= 30 && n <= 37:
			s.fg = uint8(n - 30)
		case n == 39:
			s.fg = 255
		case n >= 40 && n <= 47:
			s.bg = uint8(n - 40)
		case n == 49:
			s.bg = 255
		case n >= 90 && n <= 97:
			s.fg = uint8(n - 90 + 8)
		case n >= 100 && n <= 107:
			s.bg = uint8(n - 100 + 8)
		}
	}
}

// parseCSIN extracts the leading integer from a CSI parameter
// string. Returns defaultN when the string is empty or malformed.
func parseCSIN(paramStr string, defaultN int) int {
	if paramStr == "" {
		return defaultN
	}
	parts := strings.Split(paramStr, ";")
	n, err := strconv.Atoi(parts[0])
	if err != nil {
		return defaultN
	}
	return n
}

// ---------------------------------------------------------------------------
// Timeout-aware helper (matches the pattern in other axtree files)
// ---------------------------------------------------------------------------

// deadlineFromCtx returns the absolute deadline from ctx, or
// time.Time{} if ctx has no deadline. Currently unused by
// TUIStdinFetcher (reader-backed is synchronous) but present for
// future extensions.
func deadlineFromCtx(ctx context.Context) time.Time {
	if d, ok := ctx.Deadline(); ok {
		return d
	}
	return time.Time{}
}

// ensure deadlineFromCtx is reachable from tests / future code
var _ = deadlineFromCtx

// ---------------------------------------------------------------------------
// Platform constant
// ---------------------------------------------------------------------------

// PlatformTUI identifies the terminal-UI backend. Added via a
// separate constant file (this one) to keep node.go's Platform
// block focused on the canonical six.
const PlatformTUI Platform = "tui-ansi"

// Compile-time guards.
var (
	_ Snapshotter = (*TUISnapshotter)(nil)
	_ TUIFetcher  = (*TUIStdinFetcher)(nil)
)
