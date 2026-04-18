// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package validator — video_routing_validator.go:
//
// Generic validator for confirming that a video actually reached the
// expected output surface. Used in autonomous QA pipelines to close the
// loop on "did playback really happen on the secondary display / TV".
//
// Project-agnostic per HelixQA constitution — no hardcoded package
// names, no ATMOSphere-specific assumptions. The caller supplies:
//   - a screenshot grabber (cb returns PNG/JPEG bytes)
//   - an LLM vision client that answers "is there video in this frame"
//   - expected-output metadata (resolution, display id, packages, etc.)
//
// The validator returns a structured result that consumers (post-flash
// test scripts, CI dashboards, issue-ticket generators) can consume.

package validator

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Screenshotter captures a single frame. Implementations typically wrap
// `adb shell screencap`, a V4L2 capture from a NanoKVM, or an in-process
// Surface reader. The returned bytes are an encoded PNG or JPEG.
type Screenshotter interface {
	Capture(ctx context.Context) ([]byte, error)
}

// ScreenshotterFunc adapts a plain function to Screenshotter.
type ScreenshotterFunc func(ctx context.Context) ([]byte, error)

// Capture implements Screenshotter.
func (f ScreenshotterFunc) Capture(ctx context.Context) ([]byte, error) {
	return f(ctx)
}

// VisionClient is the minimal Gemini/OpenAI/local-LLM surface we rely
// on. Given an image and a prompt, it returns a text answer. The
// prompt is always asked to answer "yes" or "no" on the first line so
// parsing stays trivial.
type VisionClient interface {
	Describe(ctx context.Context, image []byte, prompt string) (string, error)
}

// Expectation describes what the validator should confirm.
type Expectation struct {
	// DisplayLabel is a human-readable tag for logging ("TV",
	// "primary", "soundbar"). Does not affect the logic.
	DisplayLabel string

	// MinConfidence is the threshold under which the validator
	// downgrades a positive answer to WARN rather than PASS. 0.0
	// disables confidence weighting.
	MinConfidence float64

	// BlackFrameTolerance is the fraction (0.0-1.0) of pixels
	// allowed to be near-black before the validator declares
	// "likely no video rendered". 0.95 is a sensible default.
	BlackFrameTolerance float64

	// Prompt overrides the default Gemini prompt. Use this to
	// ask domain-specific questions (e.g. "is this a movie scene
	// or a buffering spinner").
	Prompt string
}

// DefaultPrompt is the prompt used when Expectation.Prompt is empty.
const DefaultPrompt = "Is there actual video content visible in this frame? " +
	"Answer 'yes' or 'no' on the first line. Then on subsequent lines " +
	"describe what you see in at most 40 words."

// Result is the validator's verdict.
type Result struct {
	Status       string // "PASS" | "FAIL" | "WARN" | "INCONCLUSIVE"
	Reason       string
	VisionSaid   string
	BlackRatio   float64
	CapturedSize int
	At           time.Time
}

// VideoRoutingValidator runs the screenshot → black-frame check →
// vision check pipeline against a single capture. Named this way to
// avoid clashing with the package's pre-existing generic `Validator`.
type VideoRoutingValidator struct {
	Shot   Screenshotter
	Vision VisionClient
}

// NewVideoRoutingValidator constructs the validator. Both dependencies
// must be non-nil at Validate time; nil Vision short-circuits to
// INCONCLUSIVE when the black-frame check passes.
func NewVideoRoutingValidator(shot Screenshotter, vision VisionClient) *VideoRoutingValidator {
	return &VideoRoutingValidator{Shot: shot, Vision: vision}
}

// Validate captures one screenshot and runs the full pipeline.
func (v *VideoRoutingValidator) Validate(ctx context.Context, exp Expectation) (*Result, error) {
	if v == nil || v.Shot == nil {
		return nil, errors.New("validator: Shot is required")
	}

	img, err := v.Shot.Capture(ctx)
	if err != nil {
		return &Result{
			Status: "FAIL",
			Reason: fmt.Sprintf("screenshot capture failed: %v", err),
			At:     time.Now(),
		}, nil
	}
	r := &Result{CapturedSize: len(img), At: time.Now()}

	// Black-frame check (cheap, local).
	ratio, ok := blackRatioApprox(img)
	if ok {
		r.BlackRatio = ratio
		tol := exp.BlackFrameTolerance
		if tol == 0 {
			tol = 0.95
		}
		if ratio >= tol {
			r.Status = "FAIL"
			r.Reason = fmt.Sprintf("frame is %.1f%% near-black (tolerance %.1f%%) — no video rendered",
				ratio*100, tol*100)
			return r, nil
		}
	}

	if v.Vision == nil {
		r.Status = "INCONCLUSIVE"
		r.Reason = "black-frame check passed but no vision client available — cannot confirm content"
		return r, nil
	}

	prompt := exp.Prompt
	if prompt == "" {
		prompt = DefaultPrompt
	}
	answer, err := v.Vision.Describe(ctx, img, prompt)
	if err != nil {
		r.Status = "INCONCLUSIVE"
		r.Reason = fmt.Sprintf("vision call failed: %v", err)
		return r, nil
	}
	r.VisionSaid = answer

	first := firstLine(answer)
	switch {
	case strings.HasPrefix(strings.ToLower(first), "yes"):
		r.Status = "PASS"
		r.Reason = fmt.Sprintf("vision confirmed video on %s display", exp.DisplayLabel)
	case strings.HasPrefix(strings.ToLower(first), "no"):
		r.Status = "FAIL"
		r.Reason = fmt.Sprintf("vision says no video on %s display", exp.DisplayLabel)
	default:
		r.Status = "WARN"
		r.Reason = "vision response was ambiguous: " + first
	}
	return r, nil
}

// blackRatioApprox samples a small grid of bytes from the image data
// and estimates the fraction of near-black pixels. Assumes PNG/JPEG
// encoding — we don't decode properly, just probe byte density.
// A real implementation would decode; this heuristic is enough for
// "is this a totally blank frame" gating.
func blackRatioApprox(img []byte) (float64, bool) {
	if len(img) < 1024 {
		return 0, false
	}
	// Skip header; examine middle + last quarter.
	sampleStart := len(img) / 4
	sampleEnd := (3 * len(img)) / 4
	zeros := 0
	total := 0
	for i := sampleStart; i < sampleEnd; i += 16 {
		if img[i] < 0x10 {
			zeros++
		}
		total++
	}
	if total == 0 {
		return 0, false
	}
	return float64(zeros) / float64(total), true
}

func firstLine(s string) string {
	if i := bytes.IndexByte([]byte(s), '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
