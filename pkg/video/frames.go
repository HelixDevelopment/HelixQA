// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package video

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// outputPatternName is the printf pattern used for
// extracted frame filenames.
const outputPatternName = "frame_%04d.png"

// FrameExtractor extracts individual frames from a video
// file using ffmpeg.
type FrameExtractor struct {
	ffmpegPath string
}

// NewFrameExtractor creates a FrameExtractor that invokes
// ffmpeg at the given path. Pass "ffmpeg" to use the
// system PATH.
func NewFrameExtractor(path string) *FrameExtractor {
	if path == "" {
		path = "ffmpeg"
	}
	return &FrameExtractor{ffmpegPath: path}
}

// ExtractFPS extracts frames from videoPath into outputDir
// at the given frames-per-second rate. It returns the
// sorted list of extracted frame file paths.
func (f *FrameExtractor) ExtractFPS(
	ctx context.Context,
	videoPath, outputDir string,
	fps int,
) ([]string, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf(
			"create output dir: %w", err,
		)
	}

	args := f.buildArgs(videoPath, outputDir, fps)
	cmd := exec.CommandContext(ctx, f.ffmpegPath, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf(
			"ffmpeg extract fps: %w: %s", err, out,
		)
	}

	return f.listFrames(outputDir)
}

// ExtractSceneChanges extracts frames at detected scene
// changes from videoPath into outputDir. threshold is a
// value between 0 and 1 (e.g. 0.4) controlling scene
// change sensitivity. Returns extracted frame paths.
func (f *FrameExtractor) ExtractSceneChanges(
	ctx context.Context,
	videoPath, outputDir string,
	threshold float64,
) ([]string, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf(
			"create output dir: %w", err,
		)
	}

	args := f.buildSceneArgs(videoPath, outputDir, threshold)
	cmd := exec.CommandContext(ctx, f.ffmpegPath, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf(
			"ffmpeg extract scenes: %w: %s", err, out,
		)
	}

	return f.listFrames(outputDir)
}

// buildArgs constructs the ffmpeg argument list for
// constant-fps frame extraction.
func (f *FrameExtractor) buildArgs(
	videoPath, outputDir string,
	fps int,
) []string {
	pattern := f.outputPattern(outputDir)
	return []string{
		"-i", videoPath,
		"-vf", fmt.Sprintf("fps=%d", fps),
		"-vsync", "vfr",
		"-q:v", "2",
		"-y",
		pattern,
	}
}

// buildSceneArgs constructs the ffmpeg argument list for
// scene-change frame extraction.
func (f *FrameExtractor) buildSceneArgs(
	videoPath, outputDir string,
	threshold float64,
) []string {
	pattern := f.outputPattern(outputDir)
	selectFilter := fmt.Sprintf(
		"select=gt(scene\\,%g)", threshold,
	)
	return []string{
		"-i", videoPath,
		"-vf", selectFilter,
		"-vsync", "vfr",
		"-q:v", "2",
		"-y",
		pattern,
	}
}

// outputPattern returns the ffmpeg output file pattern
// for the given directory.
func (f *FrameExtractor) outputPattern(
	outputDir string,
) string {
	return filepath.Join(outputDir, outputPatternName)
}

// listFrames returns all PNG frame files in outputDir,
// sorted by name.
func (f *FrameExtractor) listFrames(
	outputDir string,
) ([]string, error) {
	pattern := filepath.Join(outputDir, "frame_*.png")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf(
			"list frames: %w", err,
		)
	}
	return matches, nil
}

// ExtractFrameAt extracts a single frame at the given timestamp
// from videoPath and saves it to outputPath.
// Timestamp format: "00:00:05.000" or seconds as float.
// This is used for screenshot replacement - capture video continuously,
// then extract frames at specific timestamps for analysis.
func (f *FrameExtractor) ExtractFrameAt(
	ctx context.Context,
	videoPath, outputPath string,
	timestamp string,
) error {
	args := []string{
		"-ss", timestamp, // Seek to timestamp
		"-i", videoPath,
		"-frames:v", "1", // Extract 1 frame
		"-q:v", "1", // Highest quality
		"-y", // Overwrite output
		outputPath,
	}
	cmd := exec.CommandContext(ctx, f.ffmpegPath, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf(
			"ffmpeg extract frame at %s: %w: %s", timestamp, err, out,
		)
	}
	return nil
}

// ExtractLatestFrame extracts the most recent frame from a video
// that's currently being recorded. It seeks to near the end and
// extracts the last available frame.
func (f *FrameExtractor) ExtractLatestFrame(
	ctx context.Context,
	videoPath, outputPath string,
) error {
	// Seek to 1 second before end to get a recent frame
	// Uses -sseof (seek from end) for partially written files
	args := []string{
		"-sseof", "-1", // 1 second from end
		"-i", videoPath,
		"-frames:v", "1",
		"-q:v", "1",
		"-y",
		outputPath,
	}
	cmd := exec.CommandContext(ctx, f.ffmpegPath, args...)
	if _, err := cmd.CombinedOutput(); err != nil {
		// Fallback: try extracting from start if seeking fails
		// (file might be too short)
		return f.ExtractFrameAt(ctx, videoPath, outputPath, "00:00:00")
	}
	return nil
}
