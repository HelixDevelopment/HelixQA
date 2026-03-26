// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package memory_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStore_RecordCoverage verifies that recording the same screen twice
// increments times_tested to 2 and updates last_status.
func TestStore_RecordCoverage(t *testing.T) {
	s := newTestStore(t)

	require.NoError(t, s.RecordCoverage("login", "android", "pass"))
	require.NoError(t, s.RecordCoverage("login", "android", "fail"))

	entry, err := s.GetCoverage("login", "android")
	require.NoError(t, err)
	require.NotNil(t, entry)

	assert.Equal(t, "login", entry.ScreenName)
	assert.Equal(t, "android", entry.Platform)
	assert.Equal(t, 2, entry.TimesTested)
	assert.Equal(t, "fail", entry.LastStatus)
	assert.False(t, entry.LastTested.IsZero(), "LastTested should be set")
}

// TestStore_ListUncoveredScreens verifies that screens not yet recorded for
// a platform are returned as uncovered.
func TestStore_ListUncoveredScreens(t *testing.T) {
	s := newTestStore(t)

	allScreens := []string{"home", "login", "settings", "profile", "search"}

	require.NoError(t, s.RecordCoverage("home", "web", "pass"))
	require.NoError(t, s.RecordCoverage("login", "web", "pass"))

	uncovered := s.ListUncoveredScreens(allScreens, "web")
	assert.Len(t, uncovered, 3)

	// Verify the uncovered set contains exactly the expected screens.
	assert.ElementsMatch(t, []string{"settings", "profile", "search"}, uncovered)
}
