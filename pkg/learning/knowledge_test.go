// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package learning_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/learning"
)

// TestKnowledgeBase_Empty verifies that a newly created KnowledgeBase has
// all slices initialised to empty (not nil) and an empty ProjectName.
func TestKnowledgeBase_Empty(t *testing.T) {
	kb := learning.NewKnowledgeBase()
	require.NotNil(t, kb)

	assert.Empty(t, kb.ProjectName)
	assert.Empty(t, kb.ProjectRoot)

	assert.NotNil(t, kb.Screens)
	assert.Len(t, kb.Screens, 0)

	assert.NotNil(t, kb.APIEndpoints)
	assert.Len(t, kb.APIEndpoints, 0)

	assert.NotNil(t, kb.Docs)
	assert.Len(t, kb.Docs, 0)

	assert.NotNil(t, kb.RecentChanges)
	assert.Len(t, kb.RecentChanges, 0)

	assert.NotNil(t, kb.Components)
	assert.Len(t, kb.Components, 0)

	assert.NotNil(t, kb.Constraints)
	assert.Len(t, kb.Constraints, 0)

	assert.NotNil(t, kb.KnownIssues)
	assert.Len(t, kb.KnownIssues, 0)
}

// TestKnowledgeBase_Summary verifies that the human-readable summary
// includes correct counts after adding 2 screens and 1 endpoint.
func TestKnowledgeBase_Summary(t *testing.T) {
	kb := learning.NewKnowledgeBase()
	kb.ProjectName = "Catalogizer"

	kb.AddScreen(learning.Screen{
		Name:       "Home",
		Platform:   "android",
		Route:      "/home",
		Component:  "HomeScreen",
		SourceFile: "HomeScreen.kt",
	})
	kb.AddScreen(learning.Screen{
		Name:       "Settings",
		Platform:   "web",
		Route:      "/settings",
		Component:  "SettingsPage",
		SourceFile: "SettingsPage.tsx",
	})
	kb.AddEndpoint(learning.APIEndpoint{
		Method:     "GET",
		Path:       "/api/v1/health",
		Handler:    "HealthHandler",
		SourceFile: "health.go",
	})

	summary := kb.Summary()

	assert.True(t, strings.Contains(summary, "2"), "summary should mention 2 screens")
	assert.True(t, strings.Contains(summary, "1"), "summary should mention 1 endpoint")
}

// TestKnowledgeBase_AddScreen verifies that adding the same screen twice
// (same Name + Platform) results in exactly one entry (deduplication).
func TestKnowledgeBase_AddScreen(t *testing.T) {
	kb := learning.NewKnowledgeBase()

	s := learning.Screen{
		Name:       "Dashboard",
		Platform:   "web",
		Route:      "/dashboard",
		Component:  "DashboardPage",
		SourceFile: "DashboardPage.tsx",
	}

	kb.AddScreen(s)
	kb.AddScreen(s) // duplicate

	assert.Len(t, kb.Screens, 1, "duplicate screen should not be added")
}

// TestKnowledgeBase_AddEndpoint verifies that adding the same endpoint twice
// (same Method + Path) results in exactly one entry (deduplication).
func TestKnowledgeBase_AddEndpoint(t *testing.T) {
	kb := learning.NewKnowledgeBase()

	ep := learning.APIEndpoint{
		Method:     "POST",
		Path:       "/api/v1/login",
		Handler:    "LoginHandler",
		SourceFile: "auth.go",
	}

	kb.AddEndpoint(ep)
	kb.AddEndpoint(ep) // duplicate

	assert.Len(t, kb.APIEndpoints, 1, "duplicate endpoint should not be added")
}
