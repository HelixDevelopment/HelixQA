// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package ticket

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"

	automation "digital.vasic.helixqa/pkg/nexus/automation"
	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// ParseReplayScript inverts BuildReplayScript. It accepts the raw DSL
// text produced by BuildReplayScript (one action per line, in the format
// "<kind>:<k>=<v>[:<k>=<v>]…") and returns the reconstructed Action
// slice.
//
// Lines that start with '#' or are blank are ignored. Lines that cannot
// be parsed produce a warning entry in the returned warnings slice but
// do not abort parsing — the remaining lines are still processed.
//
// The second return value collects human-readable warning strings of the
// form "line N: <reason>". The error return is non-nil only when the
// underlying scanner fails (I/O error), not for individual malformed
// lines.
func ParseReplayScript(data []byte) ([]automation.Action, []string, error) {
	var actions []automation.Action
	var warnings []string
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for lineNo := 1; scanner.Scan(); lineNo++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		a, err := parseReplayLine(line)
		if err != nil {
			warnings = append(warnings,
				fmt.Sprintf("line %d: %v", lineNo, err))
			continue
		}
		actions = append(actions, a)
	}
	if err := scanner.Err(); err != nil {
		return actions, warnings, fmt.Errorf("scan: %w", err)
	}
	return actions, warnings, nil
}

// parseReplayLine parses a single DSL line into an Action.
// The expected format is "<kind>[:<k>=<v>…]".
// The "from" key (used by drag) maps to Action.At.
func parseReplayLine(line string) (automation.Action, error) {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) == 0 || parts[0] == "" {
		return automation.Action{}, fmt.Errorf("empty line")
	}
	a := automation.Action{Kind: automation.ActionKind(parts[0])}
	if len(parts) == 1 {
		// Kinds with no extra fields (capture, analyze).
		return a, nil
	}
	// Parse the remaining "k=v" segments. Colons inside quoted
	// strings (text="a:b") are handled by scanning for unquoted colons.
	segments := splitKVSegments(parts[1])
	for _, seg := range segments {
		eq := strings.IndexByte(seg, '=')
		if eq < 0 {
			continue
		}
		key := seg[:eq]
		val := seg[eq+1:]
		switch key {
		case "at", "from":
			x, y, err := parsePair(val)
			if err != nil {
				return a, fmt.Errorf("key %q: %w", key, err)
			}
			a.At = contracts.Point{X: x, Y: y}
		case "to":
			x, y, err := parsePair(val)
			if err != nil {
				return a, fmt.Errorf("key %q: %w", key, err)
			}
			a.To = contracts.Point{X: x, Y: y}
		case "dx":
			v, err := strconv.Atoi(val)
			if err != nil {
				return a, fmt.Errorf("key dx: %w", err)
			}
			a.DX = v
		case "dy":
			v, err := strconv.Atoi(val)
			if err != nil {
				return a, fmt.Errorf("key dy: %w", err)
			}
			a.DY = v
		case "text":
			// BuildReplayScript uses %q, so the value is
			// a Go-quoted string (with surrounding double quotes).
			unquoted, err := strconv.Unquote(val)
			if err != nil {
				// Fallback: strip surrounding quotes manually.
				unquoted = strings.Trim(val, `"`)
			}
			a.Text = unquoted
		case "key":
			a.Key = contracts.KeyCode(val)
		case "button":
			n, err := strconv.Atoi(val)
			if err != nil {
				return a, fmt.Errorf("key button: %w", err)
			}
			a.Button = contracts.MouseButton(n)
		case "around":
			v, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				return a, fmt.Errorf("key around: %w", err)
			}
			a.ClipAround = v
		case "window":
			v, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				return a, fmt.Errorf("key window: %w", err)
			}
			a.ClipWindow = v
		}
	}
	return a, nil
}

// splitKVSegments splits the key=value portion of a DSL line on colons,
// while respecting double-quoted values (e.g. text="hello:world" must
// not split at the colon inside the quotes).
func splitKVSegments(s string) []string {
	var segs []string
	var cur strings.Builder
	inQuote := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '"' {
			inQuote = !inQuote
			cur.WriteByte(ch)
			continue
		}
		if ch == ':' && !inQuote {
			segs = append(segs, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteByte(ch)
	}
	if cur.Len() > 0 {
		segs = append(segs, cur.String())
	}
	return segs
}

// parsePair parses "X,Y" into two integers.
func parsePair(val string) (int, int, error) {
	ix := strings.IndexByte(val, ',')
	if ix < 0 {
		return 0, 0, fmt.Errorf("pair %q: missing comma", val)
	}
	x, err := strconv.Atoi(val[:ix])
	if err != nil {
		return 0, 0, fmt.Errorf("pair %q X: %w", val, err)
	}
	y, err := strconv.Atoi(val[ix+1:])
	if err != nil {
		return 0, 0, fmt.Errorf("pair %q Y: %w", val, err)
	}
	return x, y, nil
}
