// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package validator provides step-by-step validation during
// test execution. It wraps the detector to capture evidence
// (screenshots, logs) at each step and prevents false
// positives by correlating crash detection with step state.
package validator

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"digital.vasic.helixqa/pkg/config"
	"digital.vasic.helixqa/pkg/detector"
)

// StepStatus describes the outcome of a validated step.
type StepStatus string

const (
	// StepPassed indicates the step completed without issues.
	StepPassed StepStatus = "passed"
	// StepFailed indicates a crash or ANR was detected.
	StepFailed StepStatus = "failed"
	// StepSkipped indicates the step was skipped.
	StepSkipped StepStatus = "skipped"
	// StepError indicates an error during validation.
	StepError StepStatus = "error"
)

// StepResult captures the outcome of a single validated step.
type StepResult struct {
	// StepName identifies the step that was validated.
	StepName string `json:"step_name"`

	// Status is the validation outcome.
	Status StepStatus `json:"status"`

	// Platform is the platform the step ran on.
	Platform config.Platform `json:"platform"`

	// Detection holds the crash/ANR detection result.
	Detection *detector.DetectionResult `json:"detection,omitempty"`

	// PreScreenshot is the path to the screenshot taken
	// before the step.
	PreScreenshot string `json:"pre_screenshot,omitempty"`

	// PostScreenshot is the path to the screenshot taken
	// after the step.
	PostScreenshot string `json:"post_screenshot,omitempty"`

	// StartTime is when validation began.
	StartTime time.Time `json:"start_time"`

	// EndTime is when validation completed.
	EndTime time.Time `json:"end_time"`

	// Duration is the wall-clock validation time.
	Duration time.Duration `json:"duration"`

	// Error contains any error message.
	Error string `json:"error,omitempty"`
}

// ScreenshotFunc captures a screenshot and returns its path.
type ScreenshotFunc func(
	ctx context.Context, name string,
) (string, error)

// Validator performs step-by-step validation with crash
// detection and evidence collection.
type Validator struct {
	mu          sync.Mutex
	det         *detector.Detector
	evidenceDir string
	screenshot  ScreenshotFunc
	results     []*StepResult
}

// Option configures a Validator.
type Option func(*Validator)

// WithEvidenceDir sets the evidence output directory.
func WithEvidenceDir(dir string) Option {
	return func(v *Validator) {
		v.evidenceDir = dir
	}
}

// WithScreenshotFunc sets a custom screenshot function.
func WithScreenshotFunc(fn ScreenshotFunc) Option {
	return func(v *Validator) {
		v.screenshot = fn
	}
}

// New creates a Validator with the given detector and options.
func New(
	det *detector.Detector,
	opts ...Option,
) *Validator {
	v := &Validator{
		det:         det,
		evidenceDir: "evidence",
	}
	for _, opt := range opts {
		opt(v)
	}
	return v
}

// ValidateStep performs pre- and post-step crash detection,
// capturing evidence at each phase.
func (v *Validator) ValidateStep(
	ctx context.Context,
	stepName string,
	platform config.Platform,
) (*StepResult, error) {
	result := &StepResult{
		StepName:  stepName,
		Platform:  platform,
		StartTime: time.Now(),
	}

	// 1. Take pre-step screenshot if available.
	if v.screenshot != nil {
		preName := fmt.Sprintf(
			"%s-pre-%d",
			stepName,
			time.Now().UnixMilli(),
		)
		prePath, err := v.screenshot(ctx, preName)
		if err == nil {
			result.PreScreenshot = prePath
		}
	}

	// 2. Run crash detection.
	detection, err := v.det.CheckApp(ctx, platform)
	if err != nil {
		result.Status = StepError
		result.Error = fmt.Sprintf(
			"detection failed: %v", err,
		)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		v.addResult(result)
		return result, nil
	}
	result.Detection = detection

	// 3. Evaluate detection results.
	if detection.HasCrash || detection.HasANR {
		result.Status = StepFailed
		if detection.HasCrash {
			result.Error = "crash detected"
		}
		if detection.HasANR {
			if result.Error != "" {
				result.Error += "; "
			}
			result.Error += "ANR detected"
		}
		// Use the detection screenshot as evidence.
		if detection.ScreenshotPath != "" {
			result.PostScreenshot = detection.ScreenshotPath
		}
	} else {
		result.Status = StepPassed
		// 4. Take post-step screenshot on success.
		if v.screenshot != nil {
			postName := fmt.Sprintf(
				"%s-post-%d",
				stepName,
				time.Now().UnixMilli(),
			)
			postPath, screenshotErr := v.screenshot(
				ctx, postName,
			)
			if screenshotErr == nil {
				result.PostScreenshot = postPath
			}
		}
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	v.addResult(result)
	return result, nil
}

// Results returns all step results collected so far.
func (v *Validator) Results() []*StepResult {
	v.mu.Lock()
	defer v.mu.Unlock()
	out := make([]*StepResult, len(v.results))
	copy(out, v.results)
	return out
}

// PassedCount returns the number of passed steps.
func (v *Validator) PassedCount() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	count := 0
	for _, r := range v.results {
		if r.Status == StepPassed {
			count++
		}
	}
	return count
}

// FailedCount returns the number of failed steps.
func (v *Validator) FailedCount() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	count := 0
	for _, r := range v.results {
		if r.Status == StepFailed {
			count++
		}
	}
	return count
}

// TotalCount returns the total number of validated steps.
func (v *Validator) TotalCount() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return len(v.results)
}

// EvidenceDir returns the configured evidence directory.
func (v *Validator) EvidenceDir() string {
	return v.evidenceDir
}

// EvidencePath returns a full path within the evidence
// directory.
func (v *Validator) EvidencePath(name string) string {
	return filepath.Join(v.evidenceDir, name)
}

// Reset clears all collected results.
func (v *Validator) Reset() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.results = nil
}

func (v *Validator) addResult(r *StepResult) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.results = append(v.results, r)
}
