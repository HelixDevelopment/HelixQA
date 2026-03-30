// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package training_test

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/training"
)

func sampleActions() []training.Action {
	return []training.Action{
		{Type: "dpad_down", Value: "", Reason: "navigate"},
		{Type: "dpad_center", Value: "", Reason: "select"},
	}
}

// TestNewTrainingCollector verifies creation of a collector.
func TestNewTrainingCollector(t *testing.T) {
	dir := t.TempDir()
	tc := training.NewTrainingCollector(dir)
	assert.NotNil(t, tc)
	assert.Equal(t, 0, tc.Len())
}

// TestTrainingCollector_Record_Success verifies that a pair
// is recorded and the screenshot file is written.
func TestTrainingCollector_Record_Success(t *testing.T) {
	dir := t.TempDir()
	tc := training.NewTrainingCollector(dir)

	err := tc.Record(
		[]byte("PNG-DATA"),
		sampleActions(),
		"android", "curiosity", "gemini-2.0", true,
	)
	require.NoError(t, err)
	assert.Equal(t, 1, tc.Len())

	// Verify screenshot file was written.
	pairs := tc.Pairs()
	require.Len(t, pairs, 1)
	assert.FileExists(t, pairs[0].ScreenshotPath)
	assert.Equal(t, "android", pairs[0].Platform)
	assert.Equal(t, "curiosity", pairs[0].Phase)
	assert.Equal(t, "gemini-2.0", pairs[0].ModelUsed)
	assert.True(t, pairs[0].Success)
}

// TestTrainingCollector_Record_EmptyScreenshot verifies that
// an empty screenshot returns an error.
func TestTrainingCollector_Record_EmptyScreenshot(t *testing.T) {
	dir := t.TempDir()
	tc := training.NewTrainingCollector(dir)

	err := tc.Record(
		nil, sampleActions(),
		"android", "curiosity", "model", true,
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "screenshot must not be empty")
}

// TestTrainingCollector_Record_EmptyActions verifies that
// empty actions returns an error.
func TestTrainingCollector_Record_EmptyActions(t *testing.T) {
	dir := t.TempDir()
	tc := training.NewTrainingCollector(dir)

	err := tc.Record(
		[]byte("PNG"), nil,
		"android", "curiosity", "model", true,
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "actions must not be empty")
}

// TestTrainingCollector_Record_Multiple verifies recording
// multiple pairs.
func TestTrainingCollector_Record_Multiple(t *testing.T) {
	dir := t.TempDir()
	tc := training.NewTrainingCollector(dir)

	for i := 0; i < 5; i++ {
		err := tc.Record(
			[]byte("PNG-DATA"),
			sampleActions(),
			"android", "curiosity", "model", i%2 == 0,
		)
		require.NoError(t, err)
	}
	assert.Equal(t, 5, tc.Len())
}

// TestTrainingCollector_Export_Success verifies that Export
// writes a valid JSONL file.
func TestTrainingCollector_Export_Success(t *testing.T) {
	dir := t.TempDir()
	tc := training.NewTrainingCollector(
		filepath.Join(dir, "screenshots"),
	)

	for i := 0; i < 3; i++ {
		err := tc.Record(
			[]byte("PNG-DATA"),
			sampleActions(),
			"android", "curiosity", "gemini", true,
		)
		require.NoError(t, err)
	}

	exportPath := filepath.Join(dir, "export", "data.jsonl")
	count, err := tc.Export(exportPath)
	require.NoError(t, err)
	assert.Equal(t, 3, count)

	// Verify the JSONL file.
	f, err := os.Open(exportPath)
	require.NoError(t, err)
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineCount := 0
	for scanner.Scan() {
		var pair training.TrainingPair
		err := json.Unmarshal(scanner.Bytes(), &pair)
		require.NoError(t, err)
		assert.Equal(t, "android", pair.Platform)
		assert.Len(t, pair.Actions, 2)
		lineCount++
	}
	assert.Equal(t, 3, lineCount)
}

// TestTrainingCollector_Export_EmptyPairs verifies that
// Export returns an error when no pairs have been recorded.
func TestTrainingCollector_Export_EmptyPairs(t *testing.T) {
	dir := t.TempDir()
	tc := training.NewTrainingCollector(dir)

	_, err := tc.Export(filepath.Join(dir, "empty.jsonl"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no pairs to export")
}

// TestTrainingCollector_Stats verifies that Stats returns
// correct statistics.
func TestTrainingCollector_Stats(t *testing.T) {
	dir := t.TempDir()
	tc := training.NewTrainingCollector(dir)

	require.NoError(t, tc.Record(
		[]byte("PNG"), sampleActions(),
		"android", "curiosity", "gemini", true,
	))
	require.NoError(t, tc.Record(
		[]byte("PNG"), sampleActions(),
		"android", "execute", "gemini", false,
	))
	require.NoError(t, tc.Record(
		[]byte("PNG"), sampleActions(),
		"web", "curiosity", "openai", true,
	))

	stats := tc.Stats()
	assert.Equal(t, 3, stats.TotalPairs)
	assert.Equal(t, 2, stats.SuccessPairs)
	assert.Equal(t, 2, stats.ByPlatform["android"])
	assert.Equal(t, 1, stats.ByPlatform["web"])
	assert.Equal(t, 2, stats.ByModel["gemini"])
	assert.Equal(t, 1, stats.ByModel["openai"])
}

// TestTrainingCollector_Stats_Empty verifies that Stats works
// on an empty collector.
func TestTrainingCollector_Stats_Empty(t *testing.T) {
	dir := t.TempDir()
	tc := training.NewTrainingCollector(dir)

	stats := tc.Stats()
	assert.Equal(t, 0, stats.TotalPairs)
	assert.Equal(t, 0, stats.SuccessPairs)
	assert.Empty(t, stats.ByPlatform)
	assert.Empty(t, stats.ByModel)
}

// TestTrainingCollector_Pairs_ReturnsCopy verifies that Pairs
// returns a copy that does not affect the collector.
func TestTrainingCollector_Pairs_ReturnsCopy(t *testing.T) {
	dir := t.TempDir()
	tc := training.NewTrainingCollector(dir)

	require.NoError(t, tc.Record(
		[]byte("PNG"), sampleActions(),
		"android", "curiosity", "model", true,
	))

	pairs := tc.Pairs()
	pairs[0].Platform = "mutated"
	pairs[0].Actions[0].Type = "mutated"

	original := tc.Pairs()
	assert.Equal(t, "android", original[0].Platform)
	assert.Equal(t, "dpad_down", original[0].Actions[0].Type)
}

// TestTrainingCollector_Record_InvalidOutputDir verifies that
// Record returns an error for an impossible directory.
func TestTrainingCollector_Record_InvalidOutputDir(t *testing.T) {
	tc := training.NewTrainingCollector(
		"/dev/null/impossible/screenshots",
	)
	err := tc.Record(
		[]byte("PNG"), sampleActions(),
		"android", "curiosity", "model", true,
	)
	assert.Error(t, err)
}

// TestTrainingCollector_SequentialFileNames verifies that
// screenshot files get sequential names.
func TestTrainingCollector_SequentialFileNames(t *testing.T) {
	dir := t.TempDir()
	tc := training.NewTrainingCollector(dir)

	for i := 0; i < 3; i++ {
		require.NoError(t, tc.Record(
			[]byte("PNG"), sampleActions(),
			"android", "curiosity", "model", true,
		))
	}

	pairs := tc.Pairs()
	require.Len(t, pairs, 3)

	// Verify each file has a different name.
	names := make(map[string]bool)
	for _, p := range pairs {
		base := filepath.Base(p.ScreenshotPath)
		assert.False(t, names[base], "duplicate name: %s", base)
		names[base] = true
	}
}

// TestTrainingCollector_Stress_ConcurrentRecords verifies
// thread safety of Record.
func TestTrainingCollector_Stress_ConcurrentRecords(t *testing.T) {
	dir := t.TempDir()
	tc := training.NewTrainingCollector(dir)

	const goroutines = 8
	const ops = 10

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < ops; i++ {
				_ = tc.Record(
					[]byte("PNG"),
					sampleActions(),
					"android", "curiosity", "model", true,
				)
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, goroutines*ops, tc.Len())
}
