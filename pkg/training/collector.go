// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package training collects screenshot + action pairs during
// autonomous QA sessions for future vision model fine-tuning.
// It does NOT perform actual fine-tuning (that requires ML
// infrastructure); it collects and exports the training data
// in JSONL format suitable for supervised learning pipelines.
package training

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// TrainingPair represents a single screenshot + correct-action
// observation collected during an autonomous QA session.
type TrainingPair struct {
	// ScreenshotPath is the path to the screenshot PNG file.
	ScreenshotPath string `json:"screenshot"`

	// Actions is the list of actions the model decided to
	// take when it saw this screenshot.
	Actions []Action `json:"actions"`

	// Platform is the target platform (e.g. "android",
	// "androidtv", "web").
	Platform string `json:"platform"`

	// Phase is the pipeline phase (e.g. "curiosity",
	// "execute").
	Phase string `json:"phase"`

	// Timestamp is when the observation was recorded.
	Timestamp time.Time `json:"timestamp"`

	// ModelUsed is the name/identifier of the vision model
	// that produced the actions.
	ModelUsed string `json:"model"`

	// Success indicates whether the actions led to a
	// successful outcome (screen transition, no crash, etc).
	Success bool `json:"success"`
}

// Action is a single navigation action within a training pair.
type Action struct {
	// Type is the action type (e.g. "dpad_down", "type",
	// "tap").
	Type string `json:"type"`

	// Value is the action value (e.g. text to type or
	// coordinates).
	Value string `json:"value,omitempty"`

	// Reason is the model's explanation for choosing this
	// action.
	Reason string `json:"reason,omitempty"`
}

// TrainingStats summarizes the collected training data.
type TrainingStats struct {
	// TotalPairs is the total number of recorded pairs.
	TotalPairs int `json:"total_pairs"`

	// SuccessPairs is the number of pairs where Success is
	// true.
	SuccessPairs int `json:"success_pairs"`

	// ByPlatform counts pairs per platform.
	ByPlatform map[string]int `json:"by_platform"`

	// ByModel counts pairs per vision model.
	ByModel map[string]int `json:"by_model"`
}

// TrainingCollector records screenshot + action pairs during
// autonomous QA sessions. All public methods are safe for
// concurrent use.
type TrainingCollector struct {
	outputDir string
	pairs     []TrainingPair
	mu        sync.Mutex
	seqNum    int
}

// NewTrainingCollector creates a TrainingCollector that writes
// screenshots to outputDir. The directory is created if it
// does not exist.
func NewTrainingCollector(outputDir string) *TrainingCollector {
	return &TrainingCollector{
		outputDir: outputDir,
		pairs:     make([]TrainingPair, 0, 256),
	}
}

// Record saves a screenshot file and records the associated
// action pair. The screenshot bytes are written to a
// sequentially numbered PNG file under the output directory.
func (tc *TrainingCollector) Record(
	screenshot []byte,
	actions []Action,
	platform, phase, model string,
	success bool,
) error {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if len(screenshot) == 0 {
		return fmt.Errorf(
			"training: screenshot must not be empty",
		)
	}
	if len(actions) == 0 {
		return fmt.Errorf(
			"training: actions must not be empty",
		)
	}

	// Ensure the output directory exists.
	if err := os.MkdirAll(tc.outputDir, 0o755); err != nil {
		return fmt.Errorf(
			"training: create output dir %q: %w",
			tc.outputDir, err,
		)
	}

	tc.seqNum++
	fname := fmt.Sprintf(
		"%s-%s-%04d.png", platform, phase, tc.seqNum,
	)
	fpath := filepath.Join(tc.outputDir, fname)

	if err := os.WriteFile(
		fpath, screenshot, 0o644,
	); err != nil {
		return fmt.Errorf(
			"training: write screenshot %q: %w",
			fpath, err,
		)
	}

	tc.pairs = append(tc.pairs, TrainingPair{
		ScreenshotPath: fpath,
		Actions:        actions,
		Platform:       platform,
		Phase:          phase,
		Timestamp:      time.Now(),
		ModelUsed:      model,
		Success:        success,
	})
	return nil
}

// Export writes all collected pairs to a JSONL file (one JSON
// object per line) suitable for training pipelines. Returns
// the number of pairs exported.
func (tc *TrainingCollector) Export(path string) (int, error) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if len(tc.pairs) == 0 {
		return 0, fmt.Errorf(
			"training: no pairs to export",
		)
	}

	if err := os.MkdirAll(
		filepath.Dir(path), 0o755,
	); err != nil {
		return 0, fmt.Errorf(
			"training: create parent dir for %q: %w",
			path, err,
		)
	}

	f, err := os.Create(path)
	if err != nil {
		return 0, fmt.Errorf(
			"training: create file %q: %w", path, err,
		)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, pair := range tc.pairs {
		if err := enc.Encode(pair); err != nil {
			return 0, fmt.Errorf(
				"training: encode pair: %w", err,
			)
		}
	}

	return len(tc.pairs), nil
}

// Stats returns collection statistics.
func (tc *TrainingCollector) Stats() TrainingStats {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	stats := TrainingStats{
		TotalPairs: len(tc.pairs),
		ByPlatform: make(map[string]int),
		ByModel:    make(map[string]int),
	}

	for _, pair := range tc.pairs {
		if pair.Success {
			stats.SuccessPairs++
		}
		stats.ByPlatform[pair.Platform]++
		stats.ByModel[pair.ModelUsed]++
	}
	return stats
}

// Len returns the number of collected pairs.
func (tc *TrainingCollector) Len() int {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	return len(tc.pairs)
}

// Pairs returns a copy of all collected pairs.
func (tc *TrainingCollector) Pairs() []TrainingPair {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	result := make([]TrainingPair, len(tc.pairs))
	for i, p := range tc.pairs {
		result[i] = p
		result[i].Actions = make([]Action, len(p.Actions))
		copy(result[i].Actions, p.Actions)
	}
	return result
}
