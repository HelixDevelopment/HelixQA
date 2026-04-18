// Package session provides QA session management including cleanup functionality
package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CleanupConfig holds configuration for session cleanup
type CleanupConfig struct {
	// CleanAllOnNewRun removes all past QA sessions when starting a new run
	CleanAllOnNewRun bool
	// QAResultsDir is the base directory for QA results
	QAResultsDir string
}

// LoadCleanupConfig loads cleanup configuration from environment
func LoadCleanupConfig() *CleanupConfig {
	return &CleanupConfig{
		CleanAllOnNewRun: getEnvBool("HELIX_QA_CLEAN_ALL_ON_NEW_RUN", false),
		QAResultsDir:     getEnvString("HELIX_QA_RESULTS_DIR", "qa-results"),
	}
}

// CleanupPastSessions removes all past QA session directories if CleanAllOnNewRun is enabled
func (c *CleanupConfig) CleanupPastSessions() error {
	if !c.CleanAllOnNewRun {
		return nil // Cleanup disabled
	}

	// Find and remove all session directories
	pattern := filepath.Join(c.QAResultsDir, "session-*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to glob session directories: %w", err)
	}

	removedCount := 0
	for _, sessionDir := range matches {
		info, err := os.Stat(sessionDir)
		if err != nil || !info.IsDir() {
			continue
		}

		// Remove the session directory and all contents
		if err := os.RemoveAll(sessionDir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to remove session dir %s: %v\n", sessionDir, err)
			continue
		}
		removedCount++
	}

	// Also clean up loose files in the qa-results directory
	looseFiles := []string{
		"*.mp4",
		"*.png",
		"*.md",
		"*.json",
		"*.log",
	}

	for _, pattern := range looseFiles {
		matches, err := filepath.Glob(filepath.Join(c.QAResultsDir, pattern))
		if err != nil {
			continue
		}
		for _, file := range matches {
			info, err := os.Stat(file)
			if err != nil || info.IsDir() {
				continue
			}
			if err := os.Remove(file); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to remove file %s: %v\n", file, err)
			}
		}
	}

	fmt.Printf("HelixQA: Cleaned up %d past session directories\n", removedCount)
	return nil
}

// ShouldCleanSession checks if a specific session should be cleaned up
func (c *CleanupConfig) ShouldCleanSession(sessionDir string) bool {
	if !c.CleanAllOnNewRun {
		return false
	}

	// Don't clean the current session (check if it's newer than 1 minute)
	info, err := os.Stat(sessionDir)
	if err != nil {
		return true // If we can't stat it, assume it should be cleaned
	}

	// Keep sessions created in the last minute (current session)
	if time.Since(info.ModTime()) < time.Minute {
		return false
	}

	return true
}

// GetCurrentSessionDir returns the directory for the current session
func (c *CleanupConfig) GetCurrentSessionDir() string {
	timestamp := time.Now().Format("20060102-150405")
	return filepath.Join(c.QAResultsDir, fmt.Sprintf("session-%s", timestamp))
}

// ArchiveSession moves a session to an archive directory instead of deleting
func (c *CleanupConfig) ArchiveSession(sessionDir string) error {
	archiveDir := filepath.Join(c.QAResultsDir, "archive")
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return fmt.Errorf("failed to create archive directory: %w", err)
	}

	baseName := filepath.Base(sessionDir)
	archivePath := filepath.Join(archiveDir, baseName)

	// If archive path already exists, add timestamp
	if _, err := os.Stat(archivePath); err == nil {
		archivePath = fmt.Sprintf("%s-%d", archivePath, time.Now().Unix())
	}

	return os.Rename(sessionDir, archivePath)
}

// ListSessions returns a list of all session directories
func (c *CleanupConfig) ListSessions() ([]string, error) {
	pattern := filepath.Join(c.QAResultsDir, "session-*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to glob sessions: %w", err)
	}

	var sessions []string
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil || !info.IsDir() {
			continue
		}
		sessions = append(sessions, match)
	}

	return sessions, nil
}

// GetSessionStats returns statistics about QA sessions
func (c *CleanupConfig) GetSessionStats() (*SessionStats, error) {
	sessions, err := c.ListSessions()
	if err != nil {
		return nil, err
	}

	stats := &SessionStats{
		TotalSessions: len(sessions),
	}

	var totalSize int64
	for _, session := range sessions {
		size, err := getDirSize(session)
		if err != nil {
			continue
		}
		totalSize += size
	}
	stats.TotalSizeBytes = totalSize
	stats.TotalSizeHuman = formatBytes(totalSize)

	return stats, nil
}

// SessionStats holds statistics about QA sessions
type SessionStats struct {
	TotalSessions  int    `json:"total_sessions"`
	TotalSizeBytes int64  `json:"total_size_bytes"`
	TotalSizeHuman string `json:"total_size_human"`
}

// Helper functions

func getEnvBool(key string, defaultValue bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return defaultValue
	}

	val = strings.ToLower(strings.TrimSpace(val))
	switch val {
	case "true", "1", "yes", "on", "enabled":
		return true
	case "false", "0", "no", "off", "disabled":
		return false
	default:
		return defaultValue
	}
}

func getEnvString(key, defaultValue string) string {
	val := os.Getenv(key)
	if val == "" {
		return defaultValue
	}
	return val
}

func getDirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
