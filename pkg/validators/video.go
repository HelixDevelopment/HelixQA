// Package validators provides asset validation for QA session results
package validators

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// VideoValidator validates video files
type VideoValidator struct {
	ffprobePath string
}

// NewVideoValidator creates a new video validator
// ffprobePath can be empty to use "ffprobe" from PATH
func NewVideoValidator(ffprobePath string) *VideoValidator {
	if ffprobePath == "" {
		ffprobePath = "ffprobe"
	}
	return &VideoValidator{ffprobePath: ffprobePath}
}

// Validate validates a video file
func (v *VideoValidator) Validate(path string) (*ValidationResult, error) {
	result := &ValidationResult{
		AssetPath: path,
		AssetType: AssetTypeVideo,
		IsValid:   true,
		Metadata:  make(map[string]interface{}),
	}

	// Check if ffprobe is available
	if !v.isFFprobeAvailable() {
		result.IsValid = false
		result.Errors = append(result.Errors, "ffprobe not found - cannot validate video file")
		result.Warnings = append(result.Warnings, "install ffmpeg to enable video validation")
		return result, nil
	}

	// Get file info
	stat, err := os.Stat(path)
	if err != nil {
		result.IsValid = false
		result.Errors = append(result.Errors, fmt.Sprintf("failed to stat file: %v", err))
		return result, nil
	}

	if stat.Size() == 0 {
		result.IsValid = false
		result.Errors = append(result.Errors, "video file is empty (0 bytes)")
		return result, nil
	}

	result.Metadata["size_bytes"] = stat.Size()
	result.Metadata["size_human"] = formatBytes(stat.Size())

	// Run ffprobe
	probeData, err := v.runFFprobe(path)
	if err != nil {
		result.IsValid = false
		result.Errors = append(result.Errors, fmt.Sprintf("ffprobe failed: %v", err))
		return result, nil
	}

	// Parse video stream info
	if videoInfo, ok := probeData["video"]; ok {
		result.Metadata["width"] = videoInfo["width"]
		result.Metadata["height"] = videoInfo["height"]
		result.Metadata["codec"] = videoInfo["codec_name"]
		result.Metadata["duration"] = videoInfo["duration"]
		result.Metadata["bitrate"] = videoInfo["bit_rate"]
		result.Metadata["fps"] = videoInfo["r_frame_rate"]

		// Check for common issues
		width, _ := strconv.Atoi(videoInfo["width"])
		height, _ := strconv.Atoi(videoInfo["height"])
		
		if width == 0 || height == 0 {
			result.Warnings = append(result.Warnings, "video has invalid dimensions")
		}
		
		// Check duration
		duration, _ := strconv.ParseFloat(videoInfo["duration"], 64)
		if duration == 0 {
			result.Warnings = append(result.Warnings, "video has zero duration (may be corrupted)")
		} else if duration < 1 {
			result.Warnings = append(result.Warnings, fmt.Sprintf("video is very short (%.2f seconds)", duration))
		}
		
		result.Metadata["duration_seconds"] = duration
	} else {
		result.Warnings = append(result.Warnings, "no video stream found")
	}

	// Parse audio stream info
	if audioInfo, ok := probeData["audio"]; ok {
		result.Metadata["audio_codec"] = audioInfo["codec_name"]
		result.Metadata["audio_channels"] = audioInfo["channels"]
		result.Metadata["audio_sample_rate"] = audioInfo["sample_rate"]
	} else {
		result.Warnings = append(result.Warnings, "no audio stream found")
	}

	// Check format
	if formatInfo, ok := probeData["format"]; ok {
		result.Metadata["format"] = formatInfo["format_name"]
		result.Metadata["format_long"] = formatInfo["format_long_name"]
	}

	return result, nil
}

// Supports checks if this validator supports the given file
func (v *VideoValidator) Supports(path string) bool {
	type_ := DetectAssetType(path)
	return type_ == AssetTypeVideo
}

func (v *VideoValidator) isFFprobeAvailable() bool {
	cmd := exec.Command(v.ffprobePath, "-version")
	err := cmd.Run()
	return err == nil
}

func (v *VideoValidator) runFFprobe(path string) (map[string]map[string]string, error) {
	// Run ffprobe with JSON output
	cmd := exec.Command(
		v.ffprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		path,
	)
	
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe execution failed: %v", err)
	}

	// Parse the JSON output
	// This is a simplified parser - in production, use encoding/json
	result := make(map[string]map[string]string)
	result["video"] = make(map[string]string)
	result["audio"] = make(map[string]string)
	result["format"] = make(map[string]string)

	outputStr := string(output)
	
	// Extract format info
	if idx := strings.Index(outputStr, "\"format\":"); idx != -1 {
		formatSection := outputStr[idx:]
		result["format"]["format_name"] = extractJSONString(formatSection, "format_name")
		result["format"]["format_long_name"] = extractJSONString(formatSection, "format_long_name")
		result["format"]["duration"] = extractJSONString(formatSection, "duration")
		result["format"]["bit_rate"] = extractJSONString(formatSection, "bit_rate")
	}

	// Extract stream info
	if idx := strings.Index(outputStr, "\"streams\":"); idx != -1 {
		streamsSection := outputStr[idx:]
		
		// Find video stream
		if vidx := strings.Index(streamsSection, "\"codec_type\": \"video\""); vidx != -1 {
			videoSection := streamsSection[vidx:]
			result["video"]["codec_name"] = extractJSONString(videoSection, "codec_name")
			result["video"]["width"] = extractJSONString(videoSection, "width")
			result["video"]["height"] = extractJSONString(videoSection, "height")
			result["video"]["duration"] = extractJSONString(videoSection, "duration")
			result["video"]["bit_rate"] = extractJSONString(videoSection, "bit_rate")
			result["video"]["r_frame_rate"] = extractJSONString(videoSection, "r_frame_rate")
		}
		
		// Find audio stream
		if aidx := strings.Index(streamsSection, "\"codec_type\": \"audio\""); aidx != -1 {
			audioSection := streamsSection[aidx:]
			result["audio"]["codec_name"] = extractJSONString(audioSection, "codec_name")
			result["audio"]["channels"] = extractJSONString(audioSection, "channels")
			result["audio"]["sample_rate"] = extractJSONString(audioSection, "sample_rate")
		}
	}

	return result, nil
}

func extractJSONString(data, key string) string {
	// Very simple JSON string extraction
	searchStr := fmt.Sprintf("\"%s\": \"", key)
	idx := strings.Index(data, searchStr)
	if idx == -1 {
		// Try without quotes for numbers
		searchStr = fmt.Sprintf("\"%s\": ", key)
		idx = strings.Index(data, searchStr)
		if idx == -1 {
			return ""
		}
		start := idx + len(searchStr)
		end := strings.IndexAny(data[start:], ",}\n")
		if end == -1 {
			return strings.TrimSpace(data[start:])
		}
		return strings.TrimSpace(data[start : start+end])
	}
	
	start := idx + len(searchStr)
	end := strings.Index(data[start:], "\"")
	if end == -1 {
		return ""
	}
	return data[start : start+end]
}

// ValidateVideoDirectory validates all video files in a directory
func ValidateVideoDirectory(dirPath string) ([]*ValidationResult, error) {
	validator := NewVideoValidator("")
	
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	var results []*ValidationResult
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		path := filepath.Join(dirPath, entry.Name())
		if validator.Supports(path) {
			result, err := validator.Validate(path)
			if err != nil {
				return nil, err
			}
			results = append(results, result)
		}
	}

	return results, nil
}
