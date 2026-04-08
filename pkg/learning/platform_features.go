// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package learning

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// PlatformFeature represents a detected capability of a platform implementation
type PlatformFeature struct {
	// Name is the feature identifier (e.g., "androidtv_channels")
	Name string
	// Platform is the target platform (e.g., "androidtv")
	Platform string
	// Description explains what the feature does
	Description string
	// SourceFiles lists files implementing this feature
	SourceFiles []string
	// Metadata contains feature-specific data (e.g., channel count, URI scheme)
	Metadata map[string]string
}

// PlatformFeatureDetector scans codebase for platform-specific features
type PlatformFeatureDetector struct {
	root string
}

// NewPlatformFeatureDetector creates a detector for the given project root
func NewPlatformFeatureDetector(root string) *PlatformFeatureDetector {
	return &PlatformFeatureDetector{root: root}
}

// DetectAndroidTVChannels scans Android TV source files for Channels integration
// Looks for androidx.tvprovider API usage patterns
func (d *PlatformFeatureDetector) DetectAndroidTVChannels() *PlatformFeature {
	androidTVPath := filepath.Join(d.root, "catalogizer-androidtv")
	
	// If no Android TV directory, check for any androidtv directory pattern
	if _, err := os.Stat(androidTVPath); os.IsNotExist(err) {
		androidTVPath = d.findAndroidTVDir()
		if androidTVPath == "" {
			return nil
		}
	}

	var sourceFiles []string
	var hasTvProvider bool
	var hasWatchNext bool
	var hasDeepLinkActivity bool
	var uriScheme string
	var defaultChannelName string
	
	// Regex patterns for detecting Channels features
	tvProviderRegex := regexp.MustCompile(`androidx\.tvprovider\.media\.tv\.TvContractCompat`)
	watchNextRegex := regexp.MustCompile(`WatchNextPrograms|WatchNextManager`)
	deepLinkRegex := regexp.MustCompile(`ChannelDeepLinkActivity|Intent\.ACTION_VIEW.*tvprovider`)
	uriSchemeRegex := regexp.MustCompile(`"([a-z]+)://[^"]*"`)
	channelNameRegex := regexp.MustCompile(`COLUMN_DISPLAY_NAME[^"]*"([^"]+)"|DEFAULT_CHANNEL_DISPLAY_NAME[^"]*"([^"]+)"`)
	
	err := filepath.WalkDir(androidTVPath, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		
		// Only scan Kotlin files
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".kt") {
			return nil
		}
		
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		
		contentStr := string(content)
		foundFeature := false
		
		if tvProviderRegex.MatchString(contentStr) {
			hasTvProvider = true
			foundFeature = true
		}
		if watchNextRegex.MatchString(contentStr) {
			hasWatchNext = true
			foundFeature = true
		}
		if deepLinkRegex.MatchString(contentStr) {
			hasDeepLinkActivity = true
			foundFeature = true
		}
		
		// Extract URI scheme
		if uriScheme == "" {
			if matches := uriSchemeRegex.FindStringSubmatch(contentStr); len(matches) > 1 {
				scheme := strings.Split(matches[1], "://")[0]
				if scheme != "https" && scheme != "http" {
					uriScheme = scheme
				}
			}
		}
		
		// Extract default channel name
		if defaultChannelName == "" {
			if matches := channelNameRegex.FindStringSubmatch(contentStr); len(matches) > 1 {
				for i := 1; i < len(matches); i++ {
					if matches[i] != "" {
						defaultChannelName = matches[i]
						break
					}
				}
			}
		}
		
		if foundFeature {
			sourceFiles = append(sourceFiles, path)
		}
		
		return nil
	})
	
	if err != nil || !hasTvProvider {
		return nil
	}
	
	// Default values if not detected
	if uriScheme == "" {
		uriScheme = "app"
	}
	if defaultChannelName == "" {
		defaultChannelName = "Recommended"
	}
	
	metadata := map[string]string{
		"uri_scheme":          uriScheme,
		"default_channel":     defaultChannelName,
		"has_watch_next":      boolToString(hasWatchNext),
		"has_deep_linking":    boolToString(hasDeepLinkActivity),
		"tvprovider_api":      "androidx.tvprovider.media.tv",
	}
	
	return &PlatformFeature{
		Name:        "androidtv_channels",
		Platform:    "androidtv",
		Description: "Android TV Home Screen Channels integration via androidx.tvprovider",
		SourceFiles: sourceFiles,
		Metadata:    metadata,
	}
}

// findAndroidTVDir searches for Android TV directories with various naming patterns
func (d *PlatformFeatureDetector) findAndroidTVDir() string {
	patterns := []string{
		"catalogizer-androidtv",
		"androidtv",
		"tv",
		"android-tv",
		"*androidtv*",
	}
	
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(filepath.Join(d.root, pattern))
		if len(matches) > 0 {
			return matches[0]
		}
	}
	
	// Try to find any directory containing tvprovider references
	entries, _ := os.ReadDir(d.root)
	for _, entry := range entries {
		if entry.IsDir() && strings.Contains(strings.ToLower(entry.Name()), "tv") {
			return filepath.Join(d.root, entry.Name())
		}
	}
	
	return ""
}

// DetectAllPlatformFeatures scans the codebase for all supported platform features
func (d *PlatformFeatureDetector) DetectAllPlatformFeatures() []PlatformFeature {
	var features []PlatformFeature
	
	if f := d.DetectAndroidTVChannels(); f != nil {
		features = append(features, *f)
	}
	
	return features
}

// boolToString converts boolean to string
func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
