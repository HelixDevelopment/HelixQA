// Package validators provides asset validation for QA session results
package validators

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Manager orchestrates asset validation for QA sessions
type Manager struct {
	validators []Validator
	results    []*ValidationResult
}

// NewManager creates a new validation manager with all validators
func NewManager() *Manager {
	return &Manager{
		validators: []Validator{
			NewTextValidator(),
			NewImageValidator(),
			NewVideoValidator(""), // Uses "ffprobe" from PATH
		},
		results: make([]*ValidationResult, 0),
	}
}

// NewManagerWithCustomVideo creates a manager with custom ffprobe path
func NewManagerWithCustomVideo(ffprobePath string) *Manager {
	return &Manager{
		validators: []Validator{
			NewTextValidator(),
			NewImageValidator(),
			NewVideoValidator(ffprobePath),
		},
		results: make([]*ValidationResult, 0),
	}
}

// ValidateFile validates a single file
func (m *Manager) ValidateFile(path string) (*ValidationResult, error) {
	for _, validator := range m.validators {
		if validator.Supports(path) {
			result, err := validator.Validate(path)
			if err != nil {
				return nil, err
			}
			m.results = append(m.results, result)
			return result, nil
		}
	}

	// No validator found - return unknown type result
	result := &ValidationResult{
		AssetPath: path,
		AssetType: AssetTypeUnknown,
		IsValid:   true,
		Warnings:  []string{"no validator available for this file type"},
		Metadata:  make(map[string]interface{}),
	}
	m.results = append(m.results, result)
	return result, nil
}

// ValidateDirectory validates all supported files in a directory
func (m *Manager) ValidateDirectory(dirPath string, recursive bool) ([]*ValidationResult, error) {
	var files []string

	if recursive {
		err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	} else {
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				files = append(files, filepath.Join(dirPath, entry.Name()))
			}
		}
	}

	var results []*ValidationResult
	for _, file := range files {
		result, err := m.ValidateFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to validate %s: %w", file, err)
		}
		results = append(results, result)
	}

	return results, nil
}

// ValidateQASession validates all assets in a QA session directory
// Expected structure:
//
//	session-*/
//	  screenshots/     -> Image validation
//	  videos/          -> Video validation
//	  logs/            -> Text validation (logs)
//	  reports/         -> Text validation (markdown, json)
func (m *Manager) ValidateQASession(sessionPath string) (map[string][]*ValidationResult, error) {
	results := make(map[string][]*ValidationResult)

	// Validate screenshots
	screenshotsDir := filepath.Join(sessionPath, "screenshots")
	if _, err := os.Stat(screenshotsDir); err == nil {
		screenshotResults, err := m.ValidateDirectory(screenshotsDir, false)
		if err != nil {
			return nil, fmt.Errorf("failed to validate screenshots: %w", err)
		}
		results["screenshots"] = screenshotResults
	}

	// Validate videos
	videosDir := filepath.Join(sessionPath, "videos")
	if _, err := os.Stat(videosDir); err == nil {
		videoResults, err := m.ValidateDirectory(videosDir, false)
		if err != nil {
			return nil, fmt.Errorf("failed to validate videos: %w", err)
		}
		results["videos"] = videoResults
	}

	// Validate logs
	logsDir := filepath.Join(sessionPath, "logs")
	if _, err := os.Stat(logsDir); err == nil {
		logResults, err := m.ValidateDirectory(logsDir, false)
		if err != nil {
			return nil, fmt.Errorf("failed to validate logs: %w", err)
		}
		results["logs"] = logResults
	}

	// Validate reports
	reportsDir := filepath.Join(sessionPath, "reports")
	if _, err := os.Stat(reportsDir); err == nil {
		reportResults, err := m.ValidateDirectory(reportsDir, false)
		if err != nil {
			return nil, fmt.Errorf("failed to validate reports: %w", err)
		}
		results["reports"] = reportResults
	}

	return results, nil
}

// GetSummary returns a summary of all validation results
func (m *Manager) GetSummary() *ValidationSummary {
	summary := &ValidationSummary{
		TotalFiles:   len(m.results),
		ValidFiles:   0,
		InvalidFiles: 0,
		ByType:       make(map[AssetType]int),
		Errors:       []string{},
		Warnings:     []string{},
	}

	for _, result := range m.results {
		summary.ByType[result.AssetType]++

		if result.IsValid {
			summary.ValidFiles++
		} else {
			summary.InvalidFiles++
		}

		summary.Errors = append(summary.Errors, result.Errors...)
		summary.Warnings = append(summary.Warnings, result.Warnings...)
	}

	return summary
}

// GetResults returns all validation results
func (m *Manager) GetResults() []*ValidationResult {
	return m.results
}

// Clear clears all results
func (m *Manager) Clear() {
	m.results = make([]*ValidationResult, 0)
}

// ValidationSummary provides a summary of validation results
type ValidationSummary struct {
	TotalFiles   int               `json:"total_files"`
	ValidFiles   int               `json:"valid_files"`
	InvalidFiles int               `json:"invalid_files"`
	ByType       map[AssetType]int `json:"by_type"`
	Errors       []string          `json:"errors"`
	Warnings     []string          `json:"warnings"`
}

// HasErrors returns true if there are any validation errors
func (s *ValidationSummary) HasErrors() bool {
	return len(s.Errors) > 0 || s.InvalidFiles > 0
}

// HasWarnings returns true if there are any validation warnings
func (s *ValidationSummary) HasWarnings() bool {
	return len(s.Warnings) > 0
}

// String returns a human-readable summary
func (s *ValidationSummary) String() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Validation Summary:\n"))
	sb.WriteString(fmt.Sprintf("  Total files: %d\n", s.TotalFiles))
	sb.WriteString(fmt.Sprintf("  Valid: %d\n", s.ValidFiles))
	sb.WriteString(fmt.Sprintf("  Invalid: %d\n", s.InvalidFiles))
	sb.WriteString(fmt.Sprintf("  By type:\n"))
	for t, count := range s.ByType {
		sb.WriteString(fmt.Sprintf("    %s: %d\n", t, count))
	}

	if len(s.Errors) > 0 {
		sb.WriteString(fmt.Sprintf("  Errors (%d):\n", len(s.Errors)))
		for _, err := range s.Errors {
			sb.WriteString(fmt.Sprintf("    - %s\n", err))
		}
	}

	if len(s.Warnings) > 0 {
		sb.WriteString(fmt.Sprintf("  Warnings (%d):\n", len(s.Warnings)))
		for _, warn := range s.Warnings {
			sb.WriteString(fmt.Sprintf("    - %s\n", warn))
		}
	}

	return sb.String()
}
