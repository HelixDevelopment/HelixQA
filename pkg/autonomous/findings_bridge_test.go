// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package autonomous_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/analysis"
	"digital.vasic.helixqa/pkg/autonomous"
	"digital.vasic.helixqa/pkg/memory"
)

// newBridgeStore creates a temporary memory.Store backed by a real SQLite
// database inside t.TempDir(). A cleanup function is registered to close
// the store when the test finishes.
func newBridgeStore(t *testing.T) *memory.Store {
	t.Helper()
	s, err := memory.NewStore(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err, "newBridgeStore: NewStore should not error")
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// seedBridgeSession inserts a minimal session row so the findings FK
// constraint is satisfied.
func seedBridgeSession(t *testing.T, s *memory.Store, id string) {
	t.Helper()
	require.NoError(t, s.CreateSession(memory.Session{
		ID:        id,
		StartedAt: time.Now().UTC(),
	}))
}

// TestFindingsBridge_Process_TwoFindings verifies that processing two
// AnalysisFinding values persists both to the store and returns two
// distinct HELIX-NNN identifiers. It also verifies the first stored
// finding carries the expected title, severity, and status.
func TestFindingsBridge_Process_TwoFindings(t *testing.T) {
	const sessionID = "sess-bridge-001"
	s := newBridgeStore(t)
	seedBridgeSession(t, s, sessionID)

	bridge := autonomous.NewFindingsBridge(s, "", sessionID)

	findings := []analysis.AnalysisFinding{
		{
			Category:    analysis.CategoryVisual,
			Severity:    analysis.SeverityHigh,
			Title:       "Button rendered off-screen",
			Description: "The submit button is clipped at the right edge.",
			Platform:    "android",
			Screen:      "checkout",
		},
		{
			Category:    analysis.CategoryAccessibility,
			Severity:    analysis.SeverityMedium,
			Title:       "Low contrast text on banner",
			Description: "Banner text fails WCAG AA contrast ratio.",
			Platform:    "web",
			Screen:      "home",
		},
	}

	ids, err := bridge.Process(findings)
	require.NoError(t, err)
	require.Len(t, ids, 2, "should return one ID per finding")
	assert.NotEqual(t, ids[0], ids[1], "IDs must be distinct")

	// Verify first finding is retrievable with correct fields.
	got, err := s.GetFinding(ids[0])
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, ids[0], got.ID)
	assert.Equal(t, "Button rendered off-screen", got.Title)
	assert.Equal(t, "high", got.Severity)
	assert.Equal(t, "open", got.Status)
	assert.Equal(t, sessionID, got.SessionID)
}

// TestFindingsBridge_Process_EmptyFindings verifies that calling Process
// with an empty slice returns zero IDs and no error.
func TestFindingsBridge_Process_EmptyFindings(t *testing.T) {
	s := newBridgeStore(t)
	seedBridgeSession(t, s, "sess-bridge-empty")

	bridge := autonomous.NewFindingsBridge(s, "", "sess-bridge-empty")

	ids, err := bridge.Process([]analysis.AnalysisFinding{})
	require.NoError(t, err)
	assert.Empty(t, ids, "empty input should produce zero IDs")
}

// TestFindingsBridge_Process_NilStore verifies that a bridge backed by a
// nil store is a safe no-op: no panic, no error, no IDs returned.
func TestFindingsBridge_Process_NilStore(t *testing.T) {
	bridge := autonomous.NewFindingsBridge(nil, "", "sess-bridge-nil")

	findings := []analysis.AnalysisFinding{
		{
			Category: analysis.CategoryFunctional,
			Severity: analysis.SeverityCritical,
			Title:    "App crashes on launch",
		},
	}

	assert.NotPanics(t, func() {
		ids, err := bridge.Process(findings)
		assert.NoError(t, err)
		assert.Empty(t, ids)
	})
}
