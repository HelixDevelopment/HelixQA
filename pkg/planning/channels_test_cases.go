// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package planning

import "strings"

// AndroidTVChannelsTestCases returns mandatory test cases for Android TV Channels
// feature validation. These tests ensure the home screen integration works correctly.
// Reference: catalogizer-androidtv/data/tv/TvChannelRepository.kt, WatchNextManager.kt
func AndroidTVChannelsTestCases() []PlannedTest {
	return []PlannedTest{
		// ─── Default Channel Tests ─────────────────────────────────────────────
		{
			ID:          "ATV-CH-001",
			Name:        "Default Channel Auto-Creation on Launch",
			Description: "Verify 'Catalogizer Picks' channel is auto-created on first app launch",
			Category:    "functional",
			Priority:    1, // Critical
			Platforms:   []string{"androidtv"},
			Screen:      "Android TV Home Screen",
			Steps: []string{
				"Clear app data to simulate fresh install",
				"Launch Catalogizer app",
				"Press Home to return to Android TV home screen",
				"Scroll to channels row",
				"Verify 'Catalogizer Picks' channel appears in 'Your Apps' section",
			},
			Expected: "Default 'Catalogizer Picks' channel is visible with app icon and name",
		},
		{
			ID:          "ATV-CH-002",
			Name:        "Default Channel Content Population",
			Description: "Verify default channel is populated with continue watching, recent, and trending items",
			Category:    "functional",
			Priority:    1,
			Platforms:   []string{"androidtv"},
			Screen:      "Android TV Home Screen",
			Steps: []string{
				"Ensure user has partially watched content (5%-90% progress)",
				"Ensure user has recently added content",
				"Navigate to 'Catalogizer Picks' channel",
				"Wait for channel to sync (max 6h or manual refresh)",
				"Verify programs appear in channel",
			},
			Expected: "Channel displays mix of continue watching, recently added, and trending items (max 30 programs)",
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
				"Query TvContractCompat.Channels for Catalogizer channels",
				"Check COLUMN_BROWSABLE flag",
			},
			Expected: "Channel has COLUMN_BROWSABLE = 1, making it visible on home screen",
		},

		// ─── Category Channel Tests ────────────────────────────────────────────
		{
			ID:          "ATV-CH-004",
			Name:        "Dynamic Category Channel Creation",
			Description: "Verify per-media-type channels are created dynamically based on available content",
			Category:    "functional",
			Priority:    1,
			Platforms:   []string{"androidtv"},
			Screen:      "Android TV Home Screen",
			Steps: []string{
				"Add movies to library via API",
				"Add TV shows to library",
				"Trigger channel sync (or wait for WorkManager)",
				"Navigate to Android TV home screen",
				"Scroll through channels",
			},
			Expected: "Separate channels appear for 'Movies' and 'TV Shows' with relevant content",
		},
		{
			ID:          "ATV-CH-005",
			Name:        "Category Channel Naming",
			Description: "Verify category channels use correct display names from MediaType enum",
			Category:    "functional",
			Priority:    2,
			Platforms:   []string{"androidtv"},
			Screen:      "Android TV Home Screen",
			Steps: []string{
				"Add content for each media type: movie, tv_show, music_artist, game",
				"Trigger channel sync",
				"Verify channel names match MediaType.displayName values",
			},
			Expected: "Channels display as: Movies, TV Shows, Music Artists, Games, etc.",
		},
		{
			ID:          "ATV-CH-006",
			Name:        "Empty Category Channel Suppression",
			Description: "Verify channels are not created for categories with no content",
			Category:    "functional",
			Priority:    2,
			Platforms:   []string{"androidtv"},
			Screen:      "Android TV Home Screen",
			Steps: []string{
				"Ensure no books/comics exist in library",
				"Trigger channel sync",
				"Check for 'Books' or 'Comics' channels",
			},
			Expected: "No channel is created for media types with zero items",
		},
		{
			ID:          "ATV-CH-007",
			Name:        "Stale Category Channel Removal",
			Description: "Verify channels are removed when category becomes empty",
			Category:    "functional",
			Priority:    2,
			Platforms:   []string{"androidtv"},
			Screen:      "Android TV Home Screen",
			Steps: []string{
				"Add games to create 'Games' channel",
				"Verify channel appears on home screen",
				"Remove all games from library via API",
				"Trigger channel sync",
				"Re-check home screen channels",
			},
			Expected: "'Games' channel is automatically removed from home screen",
		},

		// ─── Watch Next Row Tests ──────────────────────────────────────────────
		{
			ID:          "ATV-CH-008",
			Name:        "Watch Next Row Population - Continue Watching",
			Description: "Verify partially watched items (5%-90%) appear in system Watch Next row",
			Category:    "functional",
			Priority:    1,
			Platforms:   []string{"androidtv"},
			Screen:      "Android TV Home Screen - Watch Next Row",
			Steps: []string{
				"Start playing a movie",
				"Seek to 25% progress",
				"Exit player (back button)",
				"Return to Android TV home screen",
				"Check Watch Next row at top",
			},
			Expected: "Movie appears in Watch Next row with 'Continue Watching' label and progress indicator",
		},
		{
			ID:          "ATV-CH-009",
			Name:        "Watch Next Row - TV Series Auto-Next Episode",
			Description: "Verify next episode auto-surfaces after completing current episode",
			Category:    "functional",
			Priority:    1,
			Platforms:   []string{"androidtv"},
			Screen:      "Android TV Home Screen - Watch Next Row",
			Steps: []string{
				"Complete watching TV episode (progress >90%)",
				"Exit player",
				"Trigger Watch Next sync",
				"Check Watch Next row",
			},
			Expected: "Next episode appears in Watch Next with 'Next Episode' type and episode number",
		},
		{
			ID:          "ATV-CH-010",
			Name:        "Watch Next Row - Completed Item Removal",
			Description: "Verify items are removed from Watch Next when fully watched (>90%)",
			Category:    "functional",
			Priority:    2,
			Platforms:   []string{"androidtv"},
			Screen:      "Android TV Home Screen - Watch Next Row",
			Steps: []string{
				"Have an item in Watch Next (partially watched)",
				"Resume and finish watching (>90% progress)",
				"Exit player",
				"Check Watch Next row",
			},
			Expected: "Completed item is removed from Watch Next row",
		},
		{
			ID:          "ATV-CH-011",
			Name:        "Watch Next Row - Stale Entry Cleanup",
			Description: "Verify entries older than 30 days are cleaned up",
			Category:    "edge_case",
			Priority:    3,
			Platforms:   []string{"androidtv"},
			Screen:      "Android TV Home Screen - Watch Next Row",
			Steps: []string{
				"Mock last engagement time to 31 days ago",
				"Trigger Watch Next refresh",
			},
			Expected: "Stale entries (30+ days old) are removed from Watch Next",
		},

		// ─── Deep Link Tests ───────────────────────────────────────────────────
		{
			ID:          "ATV-CH-012",
			Name:        "Channel Deep Link - Detail Navigation",
			Description: "Verify clicking channel item opens media detail screen",
			Category:    "integration",
			Priority:    1,
			Platforms:   []string{"androidtv"},
			Screen:      "ChannelDeepLinkActivity -> MediaDetailScreen",
			Steps: []string{
				"Ensure category is set to 'detail' launch action in settings",
				"Navigate to 'Catalogizer Picks' channel on home screen",
				"Select a movie poster",
				"Press OK/Enter",
			},
			Expected: "App opens with MediaDetailScreen showing selected movie metadata",
		},
		{
			ID:          "ATV-CH-013",
			Name:        "Channel Deep Link - Immediate Play",
			Description: "Verify clicking channel item starts playback for audio content",
			Category:    "integration",
			Priority:    1,
			Platforms:   []string{"androidtv"},
			Screen:      "ChannelDeepLinkActivity -> Player",
			Steps: []string{
				"Add music without external metadata",
				"Ensure category is set to 'immediate_play' launch action",
				"Navigate to channel",
				"Select music item",
				"Press OK",
			},
			Expected: "Player launches immediately (not detail screen) for audio without context",
		},
		{
			ID:          "ATV-CH-014",
			Name:        "Channel Deep Link - Resume Playback",
			Description: "Verify Watch Next items resume from saved progress",
			Category:    "integration",
			Priority:    1,
			Platforms:   []string{"androidtv"},
			Screen:      "ChannelDeepLinkActivity -> Player",
			Steps: []string{
				"Have a partially watched item in Watch Next",
				"Click item in Watch Next row",
				"Wait for player to open",
			},
			Expected: "Player opens and seeks to saved progress position automatically",
		},
		{
			ID:          "ATV-CH-015",
			Name:        "Deep Link - Unauthenticated Redirect",
			Description: "Verify deep links redirect to login when user not authenticated",
			Category:    "security",
			Priority:    2,
			Platforms:   []string{"androidtv"},
			Screen:      "ChannelDeepLinkActivity -> LoginScreen",
			Steps: []string{
				"Logout from app",
				"Click channel item from Android TV home screen",
			},
			Expected: "LoginScreen appears with pending deep link preserved for post-login navigation",
		},
		{
			ID:          "ATV-CH-016",
			Name:        "Deep Link - Invalid Media ID Handling",
			Description: "Verify graceful handling of invalid/missing media ID in deep link",
			Category:    "edge_case",
			Priority:    2,
			Platforms:   []string{"androidtv"},
			Screen:      "ChannelDeepLinkActivity",
			Steps: []string{
				"Trigger deep link with invalid URI: catalogizer://media/invalid",
				"Observe app behavior",
			},
			Expected: "App launches main activity without crash, logs warning about invalid deep link",
		},

		// ─── Sync & WorkManager Tests ──────────────────────────────────────────
		{
			ID:          "ATV-CH-017",
			Name:        "Periodic Sync via WorkManager",
			Description: "Verify channels sync periodically every 6 hours via WorkManager",
			Category:    "functional",
			Priority:    2,
			Platforms:   []string{"androidtv"},
			Screen:      "Background - TvChannelSyncWorker",
			Steps: []string{
				"Check WorkManager scheduled jobs",
				"Verify TvChannelSyncWorker is registered",
				"Check repeat interval",
			},
			Expected: "TvChannelSyncWorker scheduled with 6-hour repeat interval",
		},
		{
			ID:          "ATV-CH-018",
			Name:        "Sync on App Launch",
			Description: "Verify channels sync triggers on every app launch",
			Category:    "functional",
			Priority:    2,
			Platforms:   []string{"androidtv"},
			Screen:      "MainActivity -> TvChannelRepository",
			Steps: []string{
				"Add new content via API while app is closed",
				"Launch app",
				"Check home screen channels within seconds",
			},
			Expected: "New content appears in channels shortly after app launch",
		},
		{
			ID:          "ATV-CH-019",
			Name:        "Sync Service Manual Trigger",
			Description: "Verify channels can be manually synced via SyncService",
			Category:    "functional",
			Priority:    3,
			Platforms:   []string{"androidtv"},
			Screen:      "Settings -> Sync Channels",
			Steps: []string{
				"Add new content",
				"Navigate to app settings",
				"Trigger 'Sync Channels' manually",
				"Check home screen",
			},
			Expected: "Channels update immediately after manual sync",
		},

		// ─── Program Content Tests ─────────────────────────────────────────────
		{
			ID:          "ATV-CH-020",
			Name:        "Channel Program Metadata",
			Description: "Verify programs display correct metadata: title, poster, description",
			Category:    "functional",
			Priority:    2,
			Platforms:   []string{"androidtv"},
			Screen:      "Android TV Home Screen - Channel",
			Steps: []string{
				"Navigate to any Catalogizer channel",
				"Select a program (do not click)",
				"Observe preview card",
			},
			Expected: "Program shows title, poster art, and description in preview",
		},
		{
			ID:          "ATV-CH-021",
			Name:        "Channel Program Limit",
			Description: "Verify channels respect MAX_PROGRAMS_PER_CHANNEL limit (30)",
			Category:    "edge_case",
			Priority:    3,
			Platforms:   []string{"androidtv"},
			Screen:      "Android TV Home Screen - Channel",
			Steps: []string{
				"Add 50+ items to a single category",
				"Trigger channel sync",
				"Count programs in channel",
			},
			Expected: "Only 30 most recent/relevant programs are shown (MAX_PROGRAMS_PER_CHANNEL)",
		},
		{
			ID:          "ATV-CH-022",
			Name:        "Channel Refresh - Program Update",
			Description: "Verify existing programs are updated on refresh, not duplicated",
			Category:    "functional",
			Priority:    2,
			Platforms:   []string{"androidtv"},
			Screen:      "Android TV Home Screen - Channel",
			Steps: []string{
				"Note current programs in channel",
				"Update metadata for one item via API (change title)",
				"Trigger channel sync",
				"Check channel programs",
			},
			Expected: "Program title is updated, no duplicate entries exist",
		},

		// ─── Logout & Cleanup Tests ────────────────────────────────────────────
		{
			ID:          "ATV-CH-023",
			Name:        "Channel Cleanup on Logout",
			Description: "Verify all channels are removed from home screen on logout",
			Category:    "security",
			Priority:    1,
			Platforms:   []string{"androidtv"},
			Screen:      "Settings -> Logout",
			Steps: []string{
				"Ensure channels exist on home screen",
				"Navigate to Settings",
				"Click Logout",
				"Confirm logout",
				"Return to Android TV home screen",
			},
			Expected: "All Catalogizer channels are removed from home screen",
		},
		{
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
			Expected: "All Catalogizer entries removed from Watch Next row",
		},
		{
			ID:          "ATV-CH-025",
			Name:        "Re-authentication Channel Restoration",
			Description: "Verify channels are recreated after re-login",
			Category:    "functional",
			Priority:    2,
			Platforms:   []string{"androidtv"},
			Screen:      "LoginScreen -> MainActivity",
			Steps: []string{
				"Logout (channels removed)",
				"Login again",
				"Wait for sync",
				"Check home screen",
			},
			Expected: "All channels are recreated and populated with user's content",
		},

		// ─── App Link Intent URI Tests ─────────────────────────────────────────
		{
			ID:          "ATV-CH-026",
			Name:        "Channel App Link Intent URI",
			Description: "Verify channels have correct app link intent URIs for navigation",
			Category:    "integration",
			Priority:    2,
			Platforms:   []string{"androidtv"},
			Screen:      "Android TV Home Screen - Channel Properties",
			Steps: []string{
				"Query TvContractCompat.Channels for Catalogizer channels",
				"Check COLUMN_APP_LINK_INTENT_URI value",
			},
			Expected: "Default channel: catalogizer://home, Category channels: catalogizer://browse/{type}",
		},
		{
			ID:          "ATV-CH-027",
			Name:        "Program Intent URI Deep Link",
			Description: "Verify program intents use correct deep link format",
			Category:    "integration",
			Priority:    2,
			Platforms:   []string{"androidtv"},
			Screen:      "Android TV Home Screen - Program",
			Steps: []string{
				"Query TvContractCompat.PreviewPrograms",
				"Check COLUMN_INTENT_URI for programs",
			},
			Expected: "Intent URIs follow format: catalogizer://media/{id}?type={type}",
		},

		// ─── Error Handling & Edge Cases ───────────────────────────────────────
		{
			ID:          "ATV-CH-028",
			Name:        "Channel Creation with No Server Connection",
			Description: "Verify graceful handling when server is unreachable during sync",
			Category:    "edge_case",
			Priority:    2,
			Platforms:   []string{"androidtv"},
			Screen:      "TvChannelSyncWorker",
			Steps: []string{
				"Disconnect server or disable network",
				"Trigger channel sync",
				"Observe app behavior",
			},
			Expected: "Sync fails gracefully, existing channels preserved, error logged",
		},
		{
			ID:          "ATV-CH-029",
			Name:        "Duplicate Channel Prevention",
			Description: "Verify duplicate channels are not created on multiple syncs",
			Category:    "functional",
			Priority:    2,
			Platforms:   []string{"androidtv"},
			Screen:      "TvChannelRepository",
			Steps: []string{
				"Create default channel",
				"Trigger sync again",
				"Query TvContract for channels with same name",
			},
			Expected: "Only one 'Catalogizer Picks' channel exists regardless of sync count",
		},
		{
			ID:          "ATV-CH-030",
			Name:        "Channel Internal Provider ID",
			Description: "Verify category channels use media type as internal provider ID",
			Category:    "functional",
			Priority:    3,
			Platforms:   []string{"androidtv"},
			Screen:      "TvContractCompat.Channels",
			Steps: []string{
				"Create category channels",
				"Query COLUMN_INTERNAL_PROVIDER_ID",
			},
			Expected: "INTERNAL_PROVIDER_ID matches media type value (e.g., 'movie', 'tv_show')",
		},
	}
}

// HasAndroidTVChannelsSupport checks if the platforms list includes androidtv
func HasAndroidTVChannelsSupport(platforms []string) bool {
	for _, p := range platforms {
		if strings.EqualFold(p, "androidtv") {
			return true
		}
	}
	return false
}


