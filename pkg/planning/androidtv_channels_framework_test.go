// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package planning

import (
	"strings"
	"testing"
)

func TestDefaultChannelFeatureSpec(t *testing.T) {
	spec := DefaultChannelFeatureSpec("TestApp", "com.test.app", "testapp")

	if spec.AppName != "TestApp" {
		t.Errorf("expected AppName='TestApp', got: %s", spec.AppName)
	}
	if spec.PackageName != "com.test.app" {
		t.Errorf("expected PackageName='com.test.app', got: %s", spec.PackageName)
	}
	if spec.DefaultChannelName != "TestApp Picks" {
		t.Errorf("expected DefaultChannelName='TestApp Picks', got: %s", spec.DefaultChannelName)
	}
	if spec.DeepLink.Scheme != "testapp" {
		t.Errorf("expected Scheme='testapp', got: %s", spec.DeepLink.Scheme)
	}
	if !spec.DefaultChannelEnabled {
		t.Error("expected DefaultChannelEnabled to be true")
	}
	if !spec.CategoryChannelsEnabled {
		t.Error("expected CategoryChannelsEnabled to be true")
	}
	if !spec.WatchNext.Enabled {
		t.Error("expected WatchNext.Enabled to be true")
	}
}

func TestHasAndroidTVChannelsSupport(t *testing.T) {
	tests := []struct {
		name      string
		platforms []string
		expected  bool
	}{
		{
			name:      "with androidtv",
			platforms: []string{"web", "androidtv"},
			expected:  true,
		},
		{
			name:      "without androidtv",
			platforms: []string{"web", "android"},
			expected:  false,
		},
		{
			name:      "empty platforms",
			platforms: []string{},
			expected:  false,
		},
		{
			name:      "case insensitive",
			platforms: []string{"AndroidTV"},
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasAndroidTVChannelsSupport(tt.platforms)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGenerateAndroidTVChannelsTests(t *testing.T) {
	spec := DefaultChannelFeatureSpec("TestApp", "com.test.app", "testapp")
	tests := GenerateAndroidTVChannelsTests(spec)

	if len(tests) == 0 {
		t.Fatal("expected tests to be generated")
	}

	// Check that we have tests for all categories
	categories := make(map[string]int)
	for _, test := range tests {
		categories[test.Category]++
	}

	expectedCategories := []string{"functional", "integration", "security", "edge_case"}
	for _, cat := range expectedCategories {
		if categories[cat] == 0 {
			t.Errorf("expected at least one %s test", cat)
		}
	}

	// Check test ID format
	for _, test := range tests {
		if !strings.HasPrefix(test.ID, "ATV-CH-") {
			t.Errorf("expected test ID to start with 'ATV-CH-', got: %s", test.ID)
		}
	}

	// Check platforms
	for _, test := range tests {
		found := false
		for _, p := range test.Platforms {
			if p == "androidtv" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("test %s should include 'androidtv' platform", test.ID)
		}
	}
}

func TestGenerateDefaultChannelTests(t *testing.T) {
	spec := DefaultChannelFeatureSpec("Test", "com.test", "test")
	tests := generateDefaultChannelTests(spec)

	if len(tests) == 0 {
		t.Fatal("expected default channel tests")
	}

	// Should have specific test IDs
	expectedIDs := []string{"ATV-CH-001", "ATV-CH-002", "ATV-CH-003", "ATV-CH-004"}
	for _, id := range expectedIDs {
		found := false
		for _, test := range tests {
			if test.ID == id {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected test %s to be generated", id)
		}
	}
}

func TestGenerateCategoryChannelTests(t *testing.T) {
	spec := DefaultChannelFeatureSpec("Test", "com.test", "test")
	tests := generateCategoryChannelTests(spec)

	if len(tests) == 0 {
		t.Fatal("expected category channel tests")
	}

	// Should contain tests about dynamic creation and removal
	hasCreationTest := false
	hasRemovalTest := false
	for _, test := range tests {
		if strings.Contains(test.Name, "Creation") {
			hasCreationTest = true
		}
		if strings.Contains(test.Name, "Removal") {
			hasRemovalTest = true
		}
	}

	if !hasCreationTest {
		t.Error("expected test for channel creation")
	}
	if !hasRemovalTest {
		t.Error("expected test for channel removal")
	}
}

func TestGenerateWatchNextTests(t *testing.T) {
	spec := DefaultChannelFeatureSpec("Test", "com.test", "test")
	tests := generateWatchNextTests(spec)

	if len(tests) == 0 {
		t.Fatal("expected Watch Next tests")
	}

	// Should contain continue watching test
	hasContinueWatching := false
	for _, test := range tests {
		if strings.Contains(test.Name, "Continue") {
			hasContinueWatching = true
			break
		}
	}

	if !hasContinueWatching {
		t.Error("expected test for continue watching")
	}
}

func TestGenerateWatchNextTests_WithAutoNext(t *testing.T) {
	spec := DefaultChannelFeatureSpec("Test", "com.test", "test")
	spec.WatchNext.SupportsAutoNextEpisode = true
	tests := generateWatchNextTests(spec)

	hasAutoNext := false
	for _, test := range tests {
		if strings.Contains(test.Name, "Auto Next") {
			hasAutoNext = true
			break
		}
	}

	if !hasAutoNext {
		t.Error("expected test for auto next episode when enabled")
	}
}

func TestGenerateSyncTests(t *testing.T) {
	spec := DefaultChannelFeatureSpec("Test", "com.test", "test")
	tests := generateSyncTests(spec)

	if len(tests) == 0 {
		t.Fatal("expected sync tests")
	}

	// Should contain periodic sync test
	hasPeriodicSync := false
	for _, test := range tests {
		if strings.Contains(test.Name, "Periodic") {
			hasPeriodicSync = true
			break
		}
	}

	if !hasPeriodicSync {
		t.Error("expected test for periodic sync")
	}
}

func TestGenerateDeepLinkTests(t *testing.T) {
	spec := DefaultChannelFeatureSpec("Test", "com.test", "test")
	tests := generateDeepLinkTests(spec)

	if len(tests) == 0 {
		t.Fatal("expected deep link tests")
	}

	// Should contain detail navigation test
	hasDetailNav := false
	for _, test := range tests {
		if strings.Contains(test.Name, "Detail") {
			hasDetailNav = true
			break
		}
	}

	if !hasDetailNav {
		t.Error("expected test for detail navigation")
	}
}

func TestGenerateDeepLinkTests_Unauthenticated(t *testing.T) {
	spec := DefaultChannelFeatureSpec("Test", "com.test", "test")
	spec.DeepLink.SupportsUnauthenticated = false
	tests := generateDeepLinkTests(spec)

	hasAuthRedirect := false
	for _, test := range tests {
		if strings.Contains(test.Name, "Unauthenticated") {
			hasAuthRedirect = true
			break
		}
	}

	if !hasAuthRedirect {
		t.Error("expected test for unauthenticated redirect when not supported")
	}
}

func TestGenerateCleanupTests(t *testing.T) {
	spec := DefaultChannelFeatureSpec("Test", "com.test", "test")
	tests := generateCleanupTests(spec)

	if len(tests) == 0 {
		t.Fatal("expected cleanup tests")
	}

	// Should contain logout cleanup test
	hasLogoutCleanup := false
	for _, test := range tests {
		if strings.Contains(test.Name, "Logout") {
			hasLogoutCleanup = true
			break
		}
	}

	if !hasLogoutCleanup {
		t.Error("expected test for logout cleanup")
	}
}

func TestGenerateEdgeCaseTests(t *testing.T) {
	spec := DefaultChannelFeatureSpec("Test", "com.test", "test")
	tests := generateEdgeCaseTests(spec)

	if len(tests) == 0 {
		t.Fatal("expected edge case tests")
	}

	// Should contain invalid ID handling test
	hasInvalidID := false
	for _, test := range tests {
		if strings.Contains(test.Name, "Invalid") {
			hasInvalidID = true
			break
		}
	}

	if !hasInvalidID {
		t.Error("expected test for invalid ID handling")
	}
}

func TestInjectAndroidTVChannelsTests(t *testing.T) {
	// Test with androidtv platform
	tests := []PlannedTest{
		{ID: "EXISTING-001", Name: "Existing Test"},
	}
	platforms := []string{"androidtv"}

	result := InjectAndroidTVChannelsTests(tests, platforms, "TestApp", "testapp")

	if len(result) <= len(tests) {
		t.Error("expected channel tests to be injected")
	}

	// Check that existing tests are preserved
	foundExisting := false
	for _, test := range result {
		if test.ID == "EXISTING-001" {
			foundExisting = true
			break
		}
	}
	if !foundExisting {
		t.Error("expected existing tests to be preserved")
	}
}

func TestInjectAndroidTVChannelsTests_NoAndroidTV(t *testing.T) {
	tests := []PlannedTest{{ID: "TEST-001", Name: "Test"}}
	platforms := []string{"web", "android"}

	result := InjectAndroidTVChannelsTests(tests, platforms, "TestApp", "testapp")

	if len(result) != len(tests) {
		t.Error("should not inject tests when androidtv is not in platforms")
	}
}

func TestInjectAndroidTVChannelsTests_AlreadyHasChannels(t *testing.T) {
	tests := []PlannedTest{
		{ID: "ATV-CH-001", Name: "Already Present"},
	}
	platforms := []string{"androidtv"}

	result := InjectAndroidTVChannelsTests(tests, platforms, "TestApp", "testapp")

	// Should not add duplicate tests
	channelTestCount := 0
	for _, test := range result {
		if strings.HasPrefix(test.ID, "ATV-CH-") {
			channelTestCount++
		}
	}

	if channelTestCount != 1 {
		t.Errorf("expected 1 channel test, got: %d", channelTestCount)
	}
}

func TestChannelType_Constants(t *testing.T) {
	if ChannelTypePreview != "TYPE_PREVIEW" {
		t.Errorf("expected TYPE_PREVIEW, got: %s", ChannelTypePreview)
	}
	if ChannelTypeSingle != "TYPE_SINGLE" {
		t.Errorf("expected TYPE_SINGLE, got: %s", ChannelTypeSingle)
	}
}

func TestWatchNextType_Constants(t *testing.T) {
	if WatchNextTypeContinue != "WATCH_NEXT_TYPE_CONTINUE" {
		t.Errorf("expected WATCH_NEXT_TYPE_CONTINUE, got: %s", WatchNextTypeContinue)
	}
	if WatchNextTypeNext != "WATCH_NEXT_TYPE_NEXT" {
		t.Errorf("expected WATCH_NEXT_TYPE_NEXT, got: %s", WatchNextTypeNext)
	}
}

func BenchmarkGenerateAndroidTVChannelsTests(b *testing.B) {
	spec := DefaultChannelFeatureSpec("Test", "com.test", "test")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GenerateAndroidTVChannelsTests(spec)
	}
}
