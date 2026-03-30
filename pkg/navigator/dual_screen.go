// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package navigator

import (
	"context"
	"encoding/xml"
	"fmt"
	"strings"

	"digital.vasic.helixqa/pkg/detector"
)

// DualScreenCapture holds both the visual screenshot and the
// UI automator structure for a single screen state. The
// Combined field provides a compact text summary suitable for
// appending to an LLM prompt alongside the screenshot image.
type DualScreenCapture struct {
	// Screenshot is the raw PNG image data.
	Screenshot []byte `json:"-"`

	// UITree is the raw UI automator XML dump.
	UITree string `json:"ui_tree"`

	// Combined is a text summary extracted from UITree:
	// focused element, visible text, clickable elements with
	// bounds.
	Combined string `json:"combined"`
}

// UINode represents a single node in the UI automator XML
// hierarchy.
type UINode struct {
	XMLName     xml.Name `xml:"node"`
	Class       string   `xml:"class,attr"`
	Text        string   `xml:"text,attr"`
	ContentDesc string   `xml:"content-desc,attr"`
	Bounds      string   `xml:"bounds,attr"`
	Clickable   string   `xml:"clickable,attr"`
	Focused     string   `xml:"focused,attr"`
	Enabled     string   `xml:"enabled,attr"`
	Selected    string   `xml:"selected,attr"`
	ResourceID  string   `xml:"resource-id,attr"`
	Children    []UINode `xml:"node"`
}

// UIHierarchy is the root element of a UI automator XML dump.
type UIHierarchy struct {
	XMLName xml.Name `xml:"hierarchy"`
	Nodes   []UINode `xml:"node"`
}

// DualScreenCapturer captures both visual screenshots and UI
// structure from Android devices via ADB.
type DualScreenCapturer struct {
	cmdRunner detector.CommandRunner
}

// NewDualScreenCapturer creates a DualScreenCapturer with the
// given command runner.
func NewDualScreenCapturer(
	runner detector.CommandRunner,
) *DualScreenCapturer {
	return &DualScreenCapturer{cmdRunner: runner}
}

// CaptureDualScreen gets both the screenshot and UI dump from
// an Android device and produces a combined summary.
func (dsc *DualScreenCapturer) CaptureDualScreen(
	ctx context.Context, device string,
) (*DualScreenCapture, error) {
	// 1. Capture screenshot via screencap.
	screenshot, err := captureScreenshot(
		ctx, dsc.cmdRunner, device,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"dual screen: screenshot: %w", err,
		)
	}

	// 2. Capture UI automator dump.
	uiDump, err := captureUITree(
		ctx, dsc.cmdRunner, device,
	)
	if err != nil {
		// UI dump can fail (e.g. "null root node") but we
		// still have the screenshot. Return a partial result.
		return &DualScreenCapture{
			Screenshot: screenshot,
			UITree:     "",
			Combined:   "(UI dump unavailable)",
		}, nil
	}

	// 3. Summarize the UI tree.
	combined := SummarizeUITree(uiDump)

	return &DualScreenCapture{
		Screenshot: screenshot,
		UITree:     uiDump,
		Combined:   combined,
	}, nil
}

// captureScreenshot takes a screenshot from an Android device.
func captureScreenshot(
	ctx context.Context,
	runner detector.CommandRunner,
	device string,
) ([]byte, error) {
	data, err := runner.Run(
		ctx, "adb", "-s", device,
		"exec-out", "screencap", "-p",
	)
	if err != nil {
		return nil, fmt.Errorf("screencap: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("screencap: empty output")
	}
	return data, nil
}

// captureUITree runs uiautomator dump and returns the XML.
func captureUITree(
	ctx context.Context,
	runner detector.CommandRunner,
	device string,
) (string, error) {
	data, err := runner.Run(
		ctx, "adb", "-s", device,
		"exec-out", "uiautomator", "dump", "/dev/tty",
	)
	if err != nil {
		return "", fmt.Errorf("uiautomator dump: %w", err)
	}

	raw := string(data)

	// uiautomator dump /dev/tty outputs the XML followed by
	// "UI hierchary dumped to: /dev/tty" on the last line.
	// Strip that trailer.
	if idx := strings.LastIndex(
		raw, "UI hierchary",
	); idx > 0 {
		raw = raw[:idx]
	}
	if idx := strings.LastIndex(
		raw, "UI hierarchy",
	); idx > 0 {
		raw = raw[:idx]
	}

	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null root node" {
		return "", fmt.Errorf(
			"uiautomator: null root node",
		)
	}

	return raw, nil
}

// SummarizeUITree extracts key information from the UI
// automator XML dump and returns a compact text summary for
// LLM context. Exported for testing.
func SummarizeUITree(xmlDump string) string {
	if xmlDump == "" {
		return "(empty UI tree)"
	}

	var hierarchy UIHierarchy
	if err := xml.Unmarshal(
		[]byte(xmlDump), &hierarchy,
	); err != nil {
		return "(UI tree parse error)"
	}

	var (
		focused    []string
		texts      []string
		clickables []string
	)

	var walk func(node UINode)
	walk = func(node UINode) {
		// Collect focused elements.
		if node.Focused == "true" {
			desc := node.Class
			if node.Text != "" {
				desc += ": " + node.Text
			}
			if node.ResourceID != "" {
				desc += " [" + node.ResourceID + "]"
			}
			focused = append(focused, desc)
		}

		// Collect visible text.
		if node.Text != "" {
			texts = append(texts, node.Text)
		}
		if node.ContentDesc != "" {
			texts = append(texts, node.ContentDesc)
		}

		// Collect clickable elements.
		if node.Clickable == "true" &&
			node.Enabled == "true" {
			desc := node.Class
			if node.Text != "" {
				desc += ": " + node.Text
			} else if node.ContentDesc != "" {
				desc += ": " + node.ContentDesc
			}
			if node.Bounds != "" {
				desc += " " + node.Bounds
			}
			clickables = append(clickables, desc)
		}

		for _, child := range node.Children {
			walk(child)
		}
	}

	for _, root := range hierarchy.Nodes {
		walk(root)
	}

	var sb strings.Builder

	// Focused element(s).
	if len(focused) > 0 {
		sb.WriteString("FOCUSED: ")
		sb.WriteString(strings.Join(focused, "; "))
		sb.WriteString("\n")
	}

	// Visible text (deduplicated, limited to 20).
	if len(texts) > 0 {
		seen := make(map[string]bool)
		sb.WriteString("TEXT: ")
		count := 0
		for _, t := range texts {
			if seen[t] || t == "" {
				continue
			}
			seen[t] = true
			if count > 0 {
				sb.WriteString(" | ")
			}
			sb.WriteString(t)
			count++
			if count >= 20 {
				sb.WriteString(" | ...")
				break
			}
		}
		sb.WriteString("\n")
	}

	// Clickable elements (limited to 15).
	if len(clickables) > 0 {
		sb.WriteString("CLICKABLE: ")
		limit := len(clickables)
		if limit > 15 {
			limit = 15
		}
		for i := 0; i < limit; i++ {
			if i > 0 {
				sb.WriteString(" | ")
			}
			sb.WriteString(clickables[i])
		}
		if len(clickables) > 15 {
			sb.WriteString(
				fmt.Sprintf(
					" | ... (%d more)",
					len(clickables)-15,
				),
			)
		}
		sb.WriteString("\n")
	}

	result := sb.String()
	if result == "" {
		return "(empty UI tree)"
	}
	return strings.TrimRight(result, "\n")
}
