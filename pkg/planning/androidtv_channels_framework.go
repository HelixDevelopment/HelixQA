// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package planning provides generic Android TV Channels testing framework.
// This framework is app-agnostic and can test any Android TV app implementing
// the androidx.tvprovider API for home screen channel integration.
//
// Usage:
//   1. Define your app's ChannelFeatureSpec with URIs, channel names, and capabilities
//   2. Call GenerateAndroidTVChannelsTests(spec) to get comprehensive test cases
//   3. Tests are generated based on standard Android TV provider patterns
//
// The framework supports:
//   - Default/Recommended channels (TYPE_PREVIEW)
//   - Category channels (TYPE_PREVIEW with INTERNAL_PROVIDER_ID)
//   - Watch Next row integration (WATCH_NEXT_TYPE_CONTINUE/NEXT)
//   - Deep link handling (standard Android TV intent URIs)
//   - WorkManager periodic sync
//   - Content cleanup on logout
package planning

import (
	"fmt"
	"strings"
)

// ChannelType represents the Android TV channel type
type ChannelType string

const (
	// ChannelTypePreview - Standard preview channel with programs
	ChannelTypePreview ChannelType = "TYPE_PREVIEW"
	// ChannelTypeSingle - Single program channel (rarely used)
	ChannelTypeSingle ChannelType = "TYPE_SINGLE"
)

// WatchNextType represents the type of Watch Next entry
type WatchNextType string

const (
	// WatchNextTypeContinue - Partially watched content
	WatchNextTypeContinue WatchNextType = "WATCH_NEXT_TYPE_CONTINUE"
	// WatchNextTypeNext - Next episode in series
	WatchNextTypeNext WatchNextType = "WATCH_NEXT_TYPE_NEXT"
	// WatchNextTypeNew - New episode available
	WatchNextTypeNew WatchNextType = "WATCH_NEXT_TYPE_NEW"
)

// DeepLinkAction represents possible actions from channel clicks
type DeepLinkAction string

const (
	// DeepLinkActionDetail - Open detail screen
	DeepLinkActionDetail DeepLinkAction = "detail"
	// DeepLinkActionPlay - Start playback immediately
	DeepLinkActionPlay DeepLinkAction = "play"
	// DeepLinkActionBrowse - Open browse/collection screen
	DeepLinkActionBrowse DeepLinkAction = "browse"
)

// CategoryChannelSpec defines a category-based dynamic channel
// Example: Movies, TV Shows, Music, etc.
type CategoryChannelSpec struct {
	// ID - Unique identifier for the category (used as INTERNAL_PROVIDER_ID)
	ID string
	// DisplayName - Human-readable name shown on home screen
	DisplayName string
	// MediaType - The content type identifier (e.g., "movie", "tv_show", "music")
	MediaType string
	// DeepLinkURI - URI template for channel click (e.g., "myapp://browse/{type}")
	DeepLinkURI string
	// LaunchAction - Default action when selecting items from this channel
	LaunchAction DeepLinkAction
}

// WatchNextConfig defines Watch Next row behavior
type WatchNextConfig struct {
	// Enabled - Whether Watch Next integration is supported
	Enabled bool
	// MinProgress - Minimum watch progress to show (0.0-1.0, default 0.05 = 5%)
	MinProgress float64
	// MaxProgress - Maximum progress before removal (0.0-1.0, default 0.90 = 90%)
	MaxProgress float64
	// StaleThresholdDays - Days before entry is considered stale (default 30)
	StaleThresholdDays int
	// SupportsAutoNextEpisode - Whether next episode auto-surfaces after completion
	SupportsAutoNextEpisode bool
}

// SyncConfig defines channel synchronization behavior
type SyncConfig struct {
	// PeriodicSyncEnabled - Whether WorkManager periodic sync is used
	PeriodicSyncEnabled bool
	// SyncIntervalHours - Hours between periodic syncs (default 6)
	SyncIntervalHours int
	// SyncOnLaunch - Whether to sync on every app launch
	SyncOnLaunch bool
	// ManualSyncSupported - Whether user can trigger manual sync
	ManualSyncSupported bool
}

// DeepLinkConfig defines deep link handling
type DeepLinkConfig struct {
	// Scheme - URI scheme (e.g., "catalogizer", "myapp")
	Scheme string
	// MediaPath - Path for media deep links (e.g., "media/{id}")
	MediaPath string
	// HomePath - Path for home/channel deep links (e.g., "home")
	HomePath string
	// BrowsePathTemplate - Template for browse links (e.g., "browse/{type}")
	BrowsePathTemplate string
	// QueryParamType - Query parameter name for media type (e.g., "type")
	QueryParamType string
	// QueryParamAction - Query parameter name for action override (e.g., "action")
	QueryParamAction string
	// SupportsUnauthenticated - Whether deep links work when logged out
	SupportsUnauthenticated bool
	// UnauthenticatedRedirectScreen - Screen to show when not authenticated
	UnauthenticatedRedirectScreen string
}

// ChannelFeatureSpec defines the complete Android TV Channels feature set for an app.
// This is the main configuration struct used to generate test cases.
type ChannelFeatureSpec struct {
	// AppName - Name of the application (e.g., "Catalogizer")
	AppName string
	// PackageName - Android package name (e.g., "com.example.myapp")
	PackageName string
	
	// ─── Default Channel ────────────────────────────────────────────────────
	// DefaultChannelEnabled - Whether a default/recommended channel exists
	DefaultChannelEnabled bool
	// DefaultChannelName - Display name for default channel (e.g., "Catalogizer Picks")
	DefaultChannelName string
	// DefaultChannelKey - Internal key for default channel (e.g., "default")
	DefaultChannelKey string
	// DefaultChannelContentTypes - Content sources: "continue_watching", "recent", "trending"
	DefaultChannelContentTypes []string
	// DefaultChannelMaxPrograms - Maximum programs in default channel (default 30)
	DefaultChannelMaxPrograms int
	
	// ─── Category Channels ──────────────────────────────────────────────────
	// CategoryChannelsEnabled - Whether per-category channels are supported
	CategoryChannelsEnabled bool
	// CategoryChannels - List of category channel specifications
	CategoryChannels []CategoryChannelSpec
	// AutoCreateCategoryChannels - Whether channels are auto-created based on content
	AutoCreateCategoryChannels bool
	// RemoveEmptyCategoryChannels - Whether to remove channels when category is empty
	RemoveEmptyCategoryChannels bool
	
	// ─── Watch Next ─────────────────────────────────────────────────────────
	WatchNext WatchNextConfig
	
	// ─── Sync Configuration ─────────────────────────────────────────────────
	Sync SyncConfig
	
	// ─── Deep Link Configuration ────────────────────────────────────────────
	DeepLink DeepLinkConfig
	
	// ─── Security & Cleanup ─────────────────────────────────────────────────
	// CleanupOnLogout - Whether to remove all channels on logout
	CleanupOnLogout bool
	// CleanupWatchNextOnLogout - Whether to clear Watch Next on logout
	CleanupWatchNextOnLogout bool
}

// DefaultChannelFeatureSpec returns a sensible default spec for Android TV apps
// with full Channels support. Apps can customize this base configuration.
func DefaultChannelFeatureSpec(appName, packageName, uriScheme string) ChannelFeatureSpec {
	return ChannelFeatureSpec{
		AppName:                    appName,
		PackageName:                packageName,
		DefaultChannelEnabled:      true,
		DefaultChannelName:         appName + " Picks",
		DefaultChannelKey:          "default",
		DefaultChannelContentTypes: []string{"continue_watching", "recent", "trending"},
		DefaultChannelMaxPrograms:  30,
		CategoryChannelsEnabled:    true,
		AutoCreateCategoryChannels: true,
		RemoveEmptyCategoryChannels: true,
		WatchNext: WatchNextConfig{
			Enabled:                 true,
			MinProgress:             0.05,
			MaxProgress:             0.90,
			StaleThresholdDays:      30,
			SupportsAutoNextEpisode: true,
		},
		Sync: SyncConfig{
			PeriodicSyncEnabled: true,
			SyncIntervalHours:   6,
			SyncOnLaunch:        true,
			ManualSyncSupported: true,
		},
		DeepLink: DeepLinkConfig{
			Scheme:                        uriScheme,
			MediaPath:                     "media/{id}",
			HomePath:                      "home",
			BrowsePathTemplate:            "browse/{type}",
			QueryParamType:                "type",
			QueryParamAction:              "action",
			SupportsUnauthenticated:       false,
			UnauthenticatedRedirectScreen: "LoginScreen",
		},
		CleanupOnLogout:          true,
		CleanupWatchNextOnLogout: true,
	}
}

// GenerateAndroidTVChannelsTests generates comprehensive test cases based on the spec.
// This is the main entry point for the testing framework.
func GenerateAndroidTVChannelsTests(spec ChannelFeatureSpec) []PlannedTest {
	var tests []PlannedTest
	
	if spec.DefaultChannelEnabled {
		tests = append(tests, generateDefaultChannelTests(spec)...)
	}
	
	if spec.CategoryChannelsEnabled {
		tests = append(tests, generateCategoryChannelTests(spec)...)
	}
	
	if spec.WatchNext.Enabled {
		tests = append(tests, generateWatchNextTests(spec)...)
	}
	
	tests = append(tests, generateSyncTests(spec)...)
	tests = append(tests, generateDeepLinkTests(spec)...)
	tests = append(tests, generateCleanupTests(spec)...)
	tests = append(tests, generateEdgeCaseTests(spec)...)
	
	return tests
}

// generateDefaultChannelTests creates tests for default/recommended channel
func generateDefaultChannelTests(spec ChannelFeatureSpec) []PlannedTest {
	return []PlannedTest{
		{
			ID:          "ATV-CH-001",
			Name:        "Default Channel Auto-Creation on First Launch",
			Description: fmt.Sprintf("Verify '%s' channel is auto-created when app launches for the first time", spec.DefaultChannelName),
			Category:    "functional",
			Priority:    1,
			Platforms:   []string{"androidtv"},
			Screen:      "Android TV Home Screen",
			Steps: []string{
				"Clear app data to simulate fresh install",
				"Launch " + spec.AppName + " app",
				"Press Home to return to Android TV home screen",
				"Navigate to 'Your Apps' channels row",
			},
			Expected: fmt.Sprintf("'%s' channel appears in Your Apps section with app icon", spec.DefaultChannelName),
		},
		{
			ID:          "ATV-CH-002",
			Name:        "Default Channel Content Population",
			Description: "Verify default channel is populated with relevant content from user's library",
			Category:    "functional",
			Priority:    1,
			Platforms:   []string{"androidtv"},
			Screen:      "Android TV Home Screen",
			Steps: []string{
				"Ensure user has content in various states (in-progress, recently added, popular)",
				"Navigate to '" + spec.DefaultChannelName + "' channel",
				"Wait for content to sync",
			},
			Expected: fmt.Sprintf("Channel displays up to %d programs from: %s", 
				spec.DefaultChannelMaxPrograms, 
				strings.Join(spec.DefaultChannelContentTypes, ", ")),
		},
		{
			ID:          "ATV-CH-003",
			Name:        "Default Channel Browsable State",
			Description: "Verify channel is marked as browsable so it appears on home screen",
			Category:    "functional",
			Priority:    2,
			Platforms:   []string{"androidtv"},
			Screen:      "Android TV Home Screen",
			Steps: []string{
				"Create default channel via app",
				"Query TvContractCompat.Channels for " + spec.AppName + " channels",
				"Check COLUMN_BROWSABLE flag value",
			},
			Expected: "Channel COLUMN_BROWSABLE = 1 (visible on home screen)",
		},
		{
			ID:          "ATV-CH-004",
			Name:        "Default Channel Not Duplicated",
			Description: "Verify duplicate default channels are not created on multiple syncs",
			Category:    "functional",
			Priority:    2,
			Platforms:   []string{"androidtv"},
			Screen:      "TvContractCompat.Channels",
			Steps: []string{
				"Trigger channel sync multiple times",
				"Query channels with display name '" + spec.DefaultChannelName + "'",
			},
			Expected: "Exactly one channel with name '" + spec.DefaultChannelName + "' exists",
		},
	}
}

// generateCategoryChannelTests creates tests for category channels
func generateCategoryChannelTests(spec ChannelFeatureSpec) []PlannedTest {
	tests := []PlannedTest{
		{
			ID:          "ATV-CH-005",
			Name:        "Dynamic Category Channel Creation",
			Description: "Verify per-category channels are created based on available content types",
			Category:    "functional",
			Priority:    1,
			Platforms:   []string{"androidtv"},
			Screen:      "Android TV Home Screen",
			Steps: []string{
				"Add content for multiple categories to user's library",
				"Trigger channel sync",
				"Navigate to Android TV home screen",
				"Scroll through channels row",
			},
			Expected: "Separate channels appear for each content category with content",
		},
		{
			ID:          "ATV-CH-006",
			Name:        "Category Channel Display Names",
			Description: "Verify category channels display correct localized names",
			Category:    "functional",
			Priority:    2,
			Platforms:   []string{"androidtv"},
			Screen:      "Android TV Home Screen",
			Steps: []string{
				"Create channels for various content types",
				"Verify each channel shows appropriate display name",
			},
			Expected: "Channel names match configured display names for each category",
		},
		{
			ID:          "ATV-CH-007",
			Name:        "Empty Category Channel Suppression",
			Description: "Verify channels are not created for categories with no content",
			Category:    "functional",
			Priority:    2,
			Platforms:   []string{"androidtv"},
			Screen:      "Android TV Home Screen",
			Steps: []string{
				"Ensure some content categories have zero items",
				"Trigger channel sync",
				"Check for channels in empty categories",
			},
			Expected: "No channel is created for content categories with zero items",
		},
	}
	
	if spec.RemoveEmptyCategoryChannels {
		tests = append(tests, PlannedTest{
			ID:          "ATV-CH-008",
			Name:        "Stale Category Channel Removal",
			Description: "Verify channels are removed when category becomes empty",
			Category:    "functional",
			Priority:    2,
			Platforms:   []string{"androidtv"},
			Screen:      "Android TV Home Screen",
			Steps: []string{
				"Create content in a category to generate its channel",
				"Verify channel appears on home screen",
				"Remove all content from that category",
				"Trigger channel sync",
				"Re-check home screen channels",
			},
			Expected: "Channel is automatically removed when category becomes empty",
		})
	}
	
	return tests
}

// generateWatchNextTests creates tests for Watch Next row integration
func generateWatchNextTests(spec ChannelFeatureSpec) []PlannedTest {
	minPct := int(spec.WatchNext.MinProgress * 100)
	maxPct := int(spec.WatchNext.MaxProgress * 100)
	
	tests := []PlannedTest{
		{
			ID:          "ATV-CH-009",
			Name:        "Watch Next Row - Continue Watching",
			Description: fmt.Sprintf("Verify partially watched items (%d%%-%d%%) appear in Watch Next", minPct, maxPct),
			Category:    "functional",
			Priority:    1,
			Platforms:   []string{"androidtv"},
			Screen:      "Android TV Home Screen - Watch Next Row",
			Steps: []string{
				"Start playing content",
				fmt.Sprintf("Watch to %d%% progress", minPct+10),
				"Exit player",
				"Return to Android TV home screen",
				"Check Watch Next row",
			},
			Expected: "Item appears in Watch Next with 'Continue Watching' label and progress bar",
		},
		{
			ID:          "ATV-CH-010",
			Name:        "Watch Next Row - Completed Item Removal",
			Description: fmt.Sprintf("Verify items >%d%% watched are removed from Watch Next", maxPct),
			Category:    "functional",
			Priority:    2,
			Platforms:   []string{"androidtv"},
			Screen:      "Android TV Home Screen - Watch Next Row",
			Steps: []string{
				"Have item in Watch Next (partially watched)",
				fmt.Sprintf("Resume and finish watching (>%d%%)", maxPct),
				"Exit player",
				"Check Watch Next row",
			},
			Expected: "Completed item is removed from Watch Next row",
		},
		{
			ID:          "ATV-CH-011",
			Name:        "Watch Next Row - Minimum Threshold",
			Description: fmt.Sprintf("Verify items <%d%% watched don't appear in Watch Next", minPct),
			Category:    "functional",
			Priority:    2,
			Platforms:   []string{"androidtv"},
			Screen:      "Android TV Home Screen - Watch Next Row",
			Steps: []string{
				"Start content",
				fmt.Sprintf("Watch to %d%% (below threshold)", minPct-2),
				"Exit",
				"Check Watch Next",
			},
			Expected: fmt.Sprintf("Item does NOT appear in Watch Next (min threshold is %d%%)", minPct),
		},
	}
	
	if spec.WatchNext.SupportsAutoNextEpisode {
		tests = append(tests, PlannedTest{
			ID:          "ATV-CH-012",
			Name:        "Watch Next Row - Auto Next Episode",
			Description: "Verify next episode auto-surfaces after completing current episode",
			Category:    "functional",
			Priority:    1,
			Platforms:   []string{"androidtv"},
			Screen:      "Android TV Home Screen - Watch Next Row",
			Steps: []string{
				"Complete watching a TV episode",
				"Exit player",
				"Trigger Watch Next sync",
				"Check Watch Next row",
			},
			Expected: "Next episode appears in Watch Next with 'Next Episode' type",
		})
	}
	
	if spec.WatchNext.StaleThresholdDays > 0 {
		tests = append(tests, PlannedTest{
			ID:          "ATV-CH-013",
			Name:        "Watch Next Row - Stale Entry Cleanup",
			Description: fmt.Sprintf("Verify entries older than %d days are removed", spec.WatchNext.StaleThresholdDays),
			Category:    "edge_case",
			Priority:    3,
			Platforms:   []string{"androidtv"},
			Screen:      "Android TV Home Screen - Watch Next Row",
			Steps: []string{
				"Mock last engagement time to exceed threshold",
				"Trigger Watch Next refresh",
			},
			Expected: fmt.Sprintf("Stale entries (%d+ days old) are removed", spec.WatchNext.StaleThresholdDays),
		})
	}
	
	return tests
}

// generateSyncTests creates tests for channel synchronization
func generateSyncTests(spec ChannelFeatureSpec) []PlannedTest {
	tests := []PlannedTest{
		{
			ID:          "ATV-CH-014",
			Name:        "Channel Content Refresh",
			Description: "Verify channel content updates correctly on refresh",
			Category:    "functional",
			Priority:    2,
			Platforms:   []string{"androidtv"},
			Screen:      "Android TV Home Screen",
			Steps: []string{
				"Note current programs in channel",
				"Add new content to library",
				"Trigger channel sync",
				"Check channel content",
			},
			Expected: "New content appears, old content positions may shift, no duplicates",
		},
	}
	
	if spec.Sync.PeriodicSyncEnabled {
		tests = append(tests, PlannedTest{
			ID:          "ATV-CH-015",
			Name:        "Periodic Sync via WorkManager",
			Description: fmt.Sprintf("Verify channels sync every %d hours via WorkManager", spec.Sync.SyncIntervalHours),
			Category:    "functional",
			Priority:    2,
			Platforms:   []string{"androidtv"},
			Screen:      "Background - ChannelSyncWorker",
			Steps: []string{
				"Check WorkManager scheduled periodic work",
				"Verify sync worker is registered",
				"Check repeat interval configuration",
			},
			Expected: fmt.Sprintf("Channel sync worker scheduled with %d-hour repeat interval", spec.Sync.SyncIntervalHours),
		})
	}
	
	if spec.Sync.SyncOnLaunch {
		tests = append(tests, PlannedTest{
			ID:          "ATV-CH-016",
			Name:        "Sync on App Launch",
			Description: "Verify channels sync triggers on every app launch",
			Category:    "functional",
			Priority:    2,
			Platforms:   []string{"androidtv"},
			Screen:      "MainActivity",
			Steps: []string{
				"Add new content while app is backgrounded",
				"Launch app",
				"Check channel content shortly after",
			},
			Expected: "New content appears in channels shortly after app launch",
		})
	}
	
	if spec.Sync.ManualSyncSupported {
		tests = append(tests, PlannedTest{
			ID:          "ATV-CH-017",
			Name:        "Manual Sync Trigger",
			Description: "Verify channels can be manually synced from settings",
			Category:    "functional",
			Priority:    3,
			Platforms:   []string{"androidtv"},
			Screen:      "Settings",
			Steps: []string{
				"Add new content",
				"Navigate to app settings",
				"Trigger 'Sync Channels' manually",
				"Check home screen channels",
			},
			Expected: "Channels update immediately after manual sync trigger",
		})
	}
	
	return tests
}

// generateDeepLinkTests creates tests for deep link handling
func generateDeepLinkTests(spec ChannelFeatureSpec) []PlannedTest {
	scheme := spec.DeepLink.Scheme
	
	tests := []PlannedTest{
		{
			ID:          "ATV-CH-018",
			Name:        "Channel Deep Link - Detail Navigation",
			Description: "Verify clicking channel item opens media detail screen",
			Category:    "integration",
			Priority:    1,
			Platforms:   []string{"androidtv"},
			Screen:      "ChannelDeepLinkActivity",
			Steps: []string{
				"Navigate to " + spec.AppName + " channel on home screen",
				"Select a content poster",
				"Press OK/Enter",
			},
			Expected: "App opens with media detail screen showing selected content",
		},
		{
			ID:          "ATV-CH-019",
			Name:        "Channel Deep Link - Resume Playback",
			Description: "Verify Watch Next items resume from saved progress",
			Category:    "integration",
			Priority:    1,
			Platforms:   []string{"androidtv"},
			Screen:      "ChannelDeepLinkActivity -> Player",
			Steps: []string{
				"Have partially watched item in Watch Next",
				"Select item in Watch Next row",
				"Press OK",
			},
			Expected: "Player opens and seeks to saved progress position",
		},
		{
			ID:          "ATV-CH-020",
			Name:        "Deep Link URI Format",
			Description: "Verify program intent URIs follow correct format",
			Category:    "integration",
			Priority:    2,
			Platforms:   []string{"androidtv"},
			Screen:      "TvContractCompat.PreviewPrograms",
			Steps: []string{
				"Query PreviewPrograms content",
				"Check COLUMN_INTENT_URI values",
			},
			Expected: fmt.Sprintf("Intent URIs follow format: %s://%s?%s={type}", 
				scheme, spec.DeepLink.MediaPath, spec.DeepLink.QueryParamType),
		},
		{
			ID:          "ATV-CH-021",
			Name:        "Channel App Link Intent URI",
			Description: "Verify channels have correct app link intent URIs",
			Category:    "integration",
			Priority:    2,
			Platforms:   []string{"androidtv"},
			Screen:      "TvContractCompat.Channels",
			Steps: []string{
				"Query Channels for " + spec.AppName,
				"Check COLUMN_APP_LINK_INTENT_URI",
			},
			Expected: fmt.Sprintf("Default: %s://%s, Category: %s://%s", 
				scheme, spec.DeepLink.HomePath, scheme, spec.DeepLink.BrowsePathTemplate),
		},
	}
	
	if !spec.DeepLink.SupportsUnauthenticated {
		tests = append(tests, PlannedTest{
			ID:          "ATV-CH-022",
			Name:        "Deep Link - Unauthenticated Redirect",
			Description: "Verify deep links redirect to login when user not authenticated",
			Category:    "security",
			Priority:    1,
			Platforms:   []string{"androidtv"},
			Screen:      "ChannelDeepLinkActivity",
			Steps: []string{
				"Logout from app",
				"Click channel item from Android TV home screen",
			},
			Expected: spec.DeepLink.UnauthenticatedRedirectScreen + " appears with pending deep link preserved",
		})
	}
	
	return tests
}

// generateCleanupTests creates tests for logout cleanup
func generateCleanupTests(spec ChannelFeatureSpec) []PlannedTest {
	var tests []PlannedTest
	
	if spec.CleanupOnLogout {
		tests = append(tests, PlannedTest{
			ID:          "ATV-CH-023",
			Name:        "Channel Cleanup on Logout",
			Description: "Verify all channels are removed from home screen on logout",
			Category:    "security",
			Priority:    1,
			Platforms:   []string{"androidtv"},
			Screen:      "Settings -> Logout",
			Steps: []string{
				"Ensure channels exist on home screen",
				"Navigate to Settings -> Logout",
				"Confirm logout",
				"Return to Android TV home screen",
			},
			Expected: "All " + spec.AppName + " channels removed from home screen",
		})
	}
	
	if spec.CleanupWatchNextOnLogout {
		tests = append(tests, PlannedTest{
			ID:          "ATV-CH-024",
			Name:        "Watch Next Cleanup on Logout",
			Description: "Verify Watch Next entries are cleared on logout",
			Category:    "security",
			Priority:    1,
			Platforms:   []string{"androidtv"},
			Screen:      "Settings -> Logout",
			Steps: []string{
				"Have items in Watch Next row",
				"Logout from app",
				"Check Watch Next row on home screen",
			},
			Expected: "All " + spec.AppName + " entries removed from Watch Next",
		})
	}
	
	tests = append(tests, PlannedTest{
		ID:          "ATV-CH-025",
		Name:        "Re-authentication Channel Restoration",
		Description: "Verify channels are recreated after user logs back in",
		Category:    "functional",
		Priority:    2,
		Platforms:   []string{"androidtv"},
		Screen:      "LoginScreen",
		Steps: []string{
			"Logout (channels removed)",
			"Login again",
			"Wait for sync",
			"Check home screen",
		},
		Expected: "All channels recreated and populated with user's content",
	})
	
	return tests
}

// generateEdgeCaseTests creates edge case and error handling tests
func generateEdgeCaseTests(spec ChannelFeatureSpec) []PlannedTest {
	return []PlannedTest{
		{
			ID:          "ATV-CH-026",
			Name:        "Deep Link - Invalid Media ID Handling",
			Description: "Verify graceful handling of invalid/missing media ID",
			Category:    "edge_case",
			Priority:    2,
			Platforms:   []string{"androidtv"},
			Screen:      "ChannelDeepLinkActivity",
			Steps: []string{
				"Trigger deep link with invalid URI: " + spec.DeepLink.Scheme + "://media/invalid",
				"Observe app behavior",
			},
			Expected: "App launches main activity without crash, warning logged",
		},
		{
			ID:          "ATV-CH-027",
			Name:        "Channel Sync - No Server Connection",
			Description: "Verify graceful handling when backend is unreachable",
			Category:    "edge_case",
			Priority:    2,
			Platforms:   []string{"androidtv"},
			Screen:      "Background Sync",
			Steps: []string{
				"Disconnect network or stop backend",
				"Trigger channel sync",
				"Observe behavior",
			},
			Expected: "Sync fails gracefully, existing channels preserved, error logged",
		},
		{
			ID:          "ATV-CH-028",
			Name:        "Channel Program Limit",
			Description: fmt.Sprintf("Verify channels respect max programs limit (%d)", spec.DefaultChannelMaxPrograms),
			Category:    "edge_case",
			Priority:    3,
			Platforms:   []string{"androidtv"},
			Screen:      "Android TV Home Screen",
			Steps: []string{
				fmt.Sprintf("Add %d+ items to a category", spec.DefaultChannelMaxPrograms+10),
				"Trigger channel sync",
				"Count programs in channel",
			},
			Expected: fmt.Sprintf("Only %d most relevant programs shown", spec.DefaultChannelMaxPrograms),
		},
		{
			ID:          "ATV-CH-029",
			Name:        "Channel Internal Provider ID",
			Description: "Verify category channels use correct internal provider IDs",
			Category:    "functional",
			Priority:    3,
			Platforms:   []string{"androidtv"},
			Screen:      "TvContractCompat.Channels",
			Steps: []string{
				"Create category channels",
				"Query COLUMN_INTERNAL_PROVIDER_ID",
			},
			Expected: "INTERNAL_PROVIDER_ID matches category identifiers",
		},
		{
			ID:          "ATV-CH-030",
			Name:        "Channel Program Metadata Complete",
			Description: "Verify programs display complete metadata",
			Category:    "functional",
			Priority:    2,
			Platforms:   []string{"androidtv"},
			Screen:      "Android TV Home Screen",
			Steps: []string{
				"Navigate to any " + spec.AppName + " channel",
				"Select a program (focus but don't click)",
				"Observe preview card",
			},
			Expected: "Program shows title, poster/artwork, and description in preview",
		},
	}
}

// InjectAndroidTVChannelsTests adds generic Android TV Channels tests to a plan.
// Uses DefaultChannelFeatureSpec with minimal customization for quick setup.
func InjectAndroidTVChannelsTests(tests []PlannedTest, platforms []string, appName, uriScheme string) []PlannedTest {
	if !HasAndroidTVChannelsSupport(platforms) {
		return tests
	}
	
	// Check if channel tests already exist
	for _, t := range tests {
		if strings.HasPrefix(t.ID, "ATV-CH-") {
			return tests
		}
	}
	
	spec := DefaultChannelFeatureSpec(appName, "", uriScheme)
	channelTests := GenerateAndroidTVChannelsTests(spec)
	
	return append(tests, channelTests...)
}
