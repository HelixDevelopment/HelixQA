// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package memory_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/memory"
)

// seedSession inserts a minimal session so findings FK constraint is satisfied.
func seedSession(t *testing.T, s *memory.Store, id string) {
	t.Helper()
	require.NoError(t, s.CreateSession(memory.Session{
		ID:        id,
		StartedAt: time.Now().UTC(),
	}))
}

// TestStore_CreateFinding verifies a finding is persisted and retrievable.
func TestStore_CreateFinding(t *testing.T) {
	s := newTestStore(t)
	seedSession(t, s, "sess-f1")

	f := memory.Finding{
		ID:            "HELIX-001",
		SessionID:     "sess-f1",
		Severity:      "high",
		Category:      "crash",
		Title:         "App crashes on login",
		Description:   "Tapping login button causes immediate crash.",
		ReproSteps:    "1. Open app\n2. Tap Login",
		EvidencePaths: "/tmp/screen.png",
		Platform:      "android",
		Screen:        "LoginScreen",
		Status:        "open",
		FoundDate:     "2026-03-26",
	}

	require.NoError(t, s.CreateFinding(f))

	got, err := s.GetFinding("HELIX-001")
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, f.ID, got.ID)
	assert.Equal(t, f.SessionID, got.SessionID)
	assert.Equal(t, f.Severity, got.Severity)
	assert.Equal(t, f.Category, got.Category)
	assert.Equal(t, f.Title, got.Title)
	assert.Equal(t, f.Description, got.Description)
	assert.Equal(t, f.ReproSteps, got.ReproSteps)
	assert.Equal(t, f.EvidencePaths, got.EvidencePaths)
	assert.Equal(t, f.Platform, got.Platform)
	assert.Equal(t, f.Screen, got.Screen)
	assert.Equal(t, f.Status, got.Status)
	assert.Equal(t, f.FoundDate, got.FoundDate)
}

// TestStore_GetFinding_NotFound returns nil without error for missing IDs.
func TestStore_GetFinding_NotFound(t *testing.T) {
	s := newTestStore(t)

	got, err := s.GetFinding("HELIX-999")
	assert.NoError(t, err)
	assert.Nil(t, got)
}

// TestStore_UpdateFindingStatus verifies that the status field is updated.
func TestStore_UpdateFindingStatus(t *testing.T) {
	s := newTestStore(t)
	seedSession(t, s, "sess-f2")

	require.NoError(t, s.CreateFinding(memory.Finding{
		ID:        "HELIX-002",
		SessionID: "sess-f2",
		Severity:  "medium",
		Category:  "ui",
		Title:     "Button misaligned",
		Status:    "open",
	}))

	require.NoError(t, s.UpdateFindingStatus("HELIX-002", "fixed"))

	got, err := s.GetFinding("HELIX-002")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "fixed", got.Status)
}

// TestStore_ListFindingsByStatus verifies filtering and that only matching
// rows are returned.
func TestStore_ListOpenFindings(t *testing.T) {
	s := newTestStore(t)
	seedSession(t, s, "sess-f3")

	findings := []memory.Finding{
		{ID: "HELIX-010", SessionID: "sess-f3", Severity: "low", Category: "ui", Title: "A", Status: "open"},
		{ID: "HELIX-011", SessionID: "sess-f3", Severity: "low", Category: "ui", Title: "B", Status: "fixed"},
		{ID: "HELIX-012", SessionID: "sess-f3", Severity: "low", Category: "ui", Title: "C", Status: "open"},
	}
	for _, f := range findings {
		require.NoError(t, s.CreateFinding(f))
	}

	open, err := s.ListFindingsByStatus("open")
	require.NoError(t, err)
	assert.Len(t, open, 2)
	for _, f := range open {
		assert.Equal(t, "open", f.Status)
	}

	fixed, err := s.ListFindingsByStatus("fixed")
	require.NoError(t, err)
	assert.Len(t, fixed, 1)
	assert.Equal(t, "HELIX-011", fixed[0].ID)
}

// TestStore_NextFindingID verifies sequential HELIX-NNN generation.
func TestStore_NextFindingID(t *testing.T) {
	s := newTestStore(t)

	// Empty store → first ID is HELIX-001.
	id, err := s.NextFindingID()
	require.NoError(t, err)
	assert.Equal(t, "HELIX-001", id)

	seedSession(t, s, "sess-f4")
	require.NoError(t, s.CreateFinding(memory.Finding{
		ID: "HELIX-001", SessionID: "sess-f4",
		Severity: "low", Category: "ui", Title: "x", Status: "open",
	}))
	require.NoError(t, s.CreateFinding(memory.Finding{
		ID: "HELIX-002", SessionID: "sess-f4",
		Severity: "low", Category: "ui", Title: "y", Status: "open",
	}))

	id, err = s.NextFindingID()
	require.NoError(t, err)
	assert.Equal(t, "HELIX-003", id)
}

// TestFinding_ToMarkdown verifies that the markdown output contains required
// sections: YAML frontmatter delimiters, the title, and the repro steps.
func TestFinding_ToMarkdown(t *testing.T) {
	f := memory.Finding{
		ID:          "HELIX-042",
		Severity:    "critical",
		Category:    "crash",
		Title:       "App crashes on startup",
		Description: "The application terminates immediately after launch.",
		ReproSteps:  "1. Install\n2. Open\n3. Observe crash",
		Platform:    "android",
		Screen:      "SplashScreen",
		Status:      "open",
		FoundDate:   "2026-03-26",
	}

	md := f.ToMarkdown()

	assert.Contains(t, md, "---", "should have YAML frontmatter delimiter")
	assert.Contains(t, md, "id: HELIX-042")
	assert.Contains(t, md, "severity: critical")
	assert.Contains(t, md, "App crashes on startup")
	assert.Contains(t, md, "The application terminates immediately after launch.")
	assert.Contains(t, md, "1. Install")
	assert.Contains(t, md, "2. Open")
}

// TestStore_CreateFinding_WithAcceptanceCriteria verifies the acceptance
// criteria field is persisted and rendered in markdown.
func TestStore_CreateFinding_WithAcceptanceCriteria(t *testing.T) {
	s := newTestStore(t)
	seedSession(t, s, "sess-ac")

	f := memory.Finding{
		ID:                 "HELIX-AC-001",
		SessionID:          "sess-ac",
		Severity:           "high",
		Category:           "functional",
		Title:              "Missing acceptance criteria test",
		Description:        "Test description",
		AcceptanceCriteria: "The feature works when X returns 200 OK",
		Status:             "open",
	}

	require.NoError(t, s.CreateFinding(f))

	got, err := s.GetFinding("HELIX-AC-001")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "The feature works when X returns 200 OK", got.AcceptanceCriteria)

	md := got.ToMarkdown()
	assert.Contains(t, md, "## Acceptance Criteria")
	assert.Contains(t, md, "The feature works when X returns 200 OK")
}

// TestFinding_WriteToDir verifies the file is created with the expected name
// and non-empty content.
func TestFinding_WriteToDir(t *testing.T) {
	dir := t.TempDir()

	f := memory.Finding{
		ID:       "HELIX-007",
		Severity: "high",
		Category: "network",
		Title:    "Request timeout on slow connection",
		Status:   "open",
	}

	path, err := f.WriteToDir(dir)
	require.NoError(t, err)

	// File must exist under the given directory.
	assert.True(t, strings.HasPrefix(path, dir))
	assert.True(t, strings.HasSuffix(path, ".md"))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	assert.Contains(t, string(data), "HELIX-007")

	// The filename should contain the finding ID.
	base := filepath.Base(path)
	assert.Contains(t, base, "HELIX-007")
}
