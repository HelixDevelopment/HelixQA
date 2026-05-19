// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// ADB wraps the platform-tools adb binary for a single device.
type ADB struct {
	Path   string
	Device string
}

// run executes `adb -s <device> <args...>` and captures stdout.
func (a *ADB) run(ctx context.Context, args ...string) ([]byte, error) {
	full := append([]string{"-s", a.Device}, args...)
	cmd := exec.CommandContext(ctx, a.Path, full...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("adb %s: %w (stderr=%s)", strings.Join(args, " "), err, stderr.String())
	}
	return stdout.Bytes(), nil
}

// Ping verifies the device is reachable and reports state=device.
func (a *ADB) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := a.run(ctx, "get-state")
	if err != nil {
		return err
	}
	state := strings.TrimSpace(string(out))
	if state != "device" {
		return fmt.Errorf("device %s not in 'device' state (got %q)", a.Device, state)
	}
	return nil
}

// Tap sends an `input tap` to the given absolute coordinates.
func (a *ADB) Tap(x, y int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := a.run(ctx, "shell", "input", "tap", fmt.Sprintf("%d", x), fmt.Sprintf("%d", y))
	return err
}

// Type sends keystrokes via `input text`. Spaces are %s-escaped per
// the standard adb convention.
func (a *ADB) Type(text string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// `input text` requires no embedded spaces; replace with %s placeholder
	// which the input command interprets as a space character.
	escaped := strings.ReplaceAll(text, " ", "%s")
	_, err := a.run(ctx, "shell", "input", "text", escaped)
	return err
}

// LaunchActivity calls `am start -n <pkg>/<activity>`.
func (a *ADB) LaunchActivity(component string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_, err := a.run(ctx, "shell", "am", "start", "-n", component)
	return err
}

// ForceStop kills the named package.
func (a *ADB) ForceStop(pkg string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := a.run(ctx, "shell", "am", "force-stop", pkg)
	return err
}

// Screenshot captures the device screen to `outPath` (PNG).
func (a *ADB) Screenshot(outPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	data, err := a.run(ctx, "exec-out", "screencap", "-p")
	if err != nil {
		return err
	}
	return os.WriteFile(outPath, data, 0o644)
}

// UIHierarchy is a minimal parse of the uiautomator-dumped XML tree.
type UIHierarchy struct {
	Nodes []UINode
}

// UINode represents a single visible node.
type UINode struct {
	Class       string
	Text        string
	ContentDesc string
	Bounds      string // "[x1,y1][x2,y2]"
}

type rawNode struct {
	Class       string    `xml:"class,attr"`
	Text        string    `xml:"text,attr"`
	ContentDesc string    `xml:"content-desc,attr"`
	Bounds      string    `xml:"bounds,attr"`
	Children    []rawNode `xml:"node"`
}

type rawHierarchy struct {
	XMLName xml.Name  `xml:"hierarchy"`
	Root    []rawNode `xml:"node"`
}

// Dump pulls a fresh uiautomator XML from the device + parses it.
func (a *ADB) Dump() (*UIHierarchy, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	// uiautomator dump writes to /sdcard/window_dump.xml; pull via cat.
	if _, err := a.run(ctx, "shell", "uiautomator", "dump", "--compressed", "/sdcard/window_dump.xml"); err != nil {
		return nil, fmt.Errorf("uiautomator dump: %w", err)
	}
	out, err := a.run(ctx, "shell", "cat", "/sdcard/window_dump.xml")
	if err != nil {
		return nil, fmt.Errorf("cat dump: %w", err)
	}
	var raw rawHierarchy
	if err := xml.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parse dump: %w", err)
	}
	var hier UIHierarchy
	for _, n := range raw.Root {
		flatten(n, &hier)
	}
	return &hier, nil
}

func flatten(n rawNode, h *UIHierarchy) {
	h.Nodes = append(h.Nodes, UINode{
		Class:       n.Class,
		Text:        n.Text,
		ContentDesc: n.ContentDesc,
		Bounds:      n.Bounds,
	})
	for _, c := range n.Children {
		flatten(c, h)
	}
}

// FindByText returns the first node whose text == target (or nil).
func (h *UIHierarchy) FindByText(target string) *UINode {
	for i, n := range h.Nodes {
		if n.Text == target {
			return &h.Nodes[i]
		}
	}
	return nil
}

// FindByDesc returns the first node whose content-desc == target.
func (h *UIHierarchy) FindByDesc(target string) *UINode {
	for i, n := range h.Nodes {
		if n.ContentDesc == target {
			return &h.Nodes[i]
		}
	}
	return nil
}

// HasText is true iff any node has the given text value.
func (h *UIHierarchy) HasText(target string) bool {
	return h.FindByText(target) != nil
}

// HasDesc is true iff any node has the given content-desc value.
func (h *UIHierarchy) HasDesc(target string) bool {
	return h.FindByDesc(target) != nil
}

var boundsRe = regexp.MustCompile(`\[(\d+),(\d+)\]\[(\d+),(\d+)\]`)

// Center returns the midpoint of the node's bounds. The bounds attribute
// is "[x1,y1][x2,y2]".
func (n *UINode) Center() (x, y int, ok bool) {
	m := boundsRe.FindStringSubmatch(n.Bounds)
	if len(m) != 5 {
		return 0, 0, false
	}
	x1, y1, x2, y2 := atoi(m[1]), atoi(m[2]), atoi(m[3]), atoi(m[4])
	return (x1 + x2) / 2, (y1 + y2) / 2, true
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return n
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// CurrentActivity returns the focused activity from dumpsys window.
func (a *ADB) CurrentActivity() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := a.run(ctx, "shell", "dumpsys", "window")
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile(`mFocusedApp=ActivityRecord\{[^ ]+ [^ ]+ ([^ ]+)`)
	m := re.FindStringSubmatch(string(out))
	if len(m) >= 2 {
		return m[1], nil
	}
	return "", fmt.Errorf("could not find mFocusedApp in dumpsys window")
}
