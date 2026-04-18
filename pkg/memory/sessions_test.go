// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package memory_test

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/memory"
)

// newTestStore creates a temporary Store backed by a real SQLite file
// inside t.TempDir(). It registers a cleanup that closes the store when
// the test finishes.
func newTestStore(t *testing.T) *memory.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := memory.NewStore(filepath.Join(dir, "helixqa.db"))
	require.NoError(t, err, "newTestStore: NewStore should not error")
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// TestStore_CreateSession verifies that a session is persisted and can be
// retrieved by ID.
func TestStore_CreateSession(t *testing.T) {
	s := newTestStore(t)

	sess := memory.Session{
		ID:         "sess-001",
		StartedAt:  time.Now().UTC().Truncate(time.Second),
		Platforms:  "android,web",
		TotalTests: 42,
		Passed:     40,
		Failed:     2,
		PassNumber: 1,
		Notes:      "smoke run",
	}

	require.NoError(t, s.CreateSession(sess))

	got, err := s.GetSession("sess-001")
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, sess.ID, got.ID)
	assert.Equal(t, sess.Platforms, got.Platforms)
	assert.Equal(t, sess.TotalTests, got.TotalTests)
	assert.Equal(t, sess.Passed, got.Passed)
	assert.Equal(t, sess.Failed, got.Failed)
	assert.Equal(t, sess.PassNumber, got.PassNumber)
	assert.Equal(t, sess.Notes, got.Notes)
	assert.WithinDuration(t, sess.StartedAt, got.StartedAt, time.Second)
}

// TestStore_GetSession_NotFound verifies that requesting a missing session
// returns a nil pointer and no error (not-found is a zero-value result, not
// an error condition).
func TestStore_GetSession_NotFound(t *testing.T) {
	s := newTestStore(t)

	got, err := s.GetSession("does-not-exist")
	assert.NoError(t, err)
	assert.Nil(t, got)
}

// TestStore_UpdateSession verifies that UpdateSession mutates the stored row.
func TestStore_UpdateSession(t *testing.T) {
	s := newTestStore(t)

	require.NoError(t, s.CreateSession(memory.Session{
		ID:         "sess-upd",
		StartedAt:  time.Now().UTC(),
		PassNumber: 2,
	}))

	endedAt := time.Now().UTC().Add(5 * time.Minute).Truncate(time.Second)
	update := memory.SessionUpdate{
		EndedAt:       &endedAt,
		Duration:      300,
		TotalTests:    10,
		Passed:        9,
		Failed:        1,
		FindingsCount: 3,
		CoveragePct:   87.5,
		Notes:         "updated notes",
	}

	require.NoError(t, s.UpdateSession("sess-upd", update))

	got, err := s.GetSession("sess-upd")
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.WithinDuration(t, endedAt, got.EndedAt, time.Second)
	assert.Equal(t, 300, got.Duration)
	assert.Equal(t, 10, got.TotalTests)
	assert.Equal(t, 9, got.Passed)
	assert.Equal(t, 1, got.Failed)
	assert.Equal(t, 3, got.FindingsCount)
	assert.InDelta(t, 87.5, got.CoveragePct, 0.01)
	assert.Equal(t, "updated notes", got.Notes)
}

// TestStore_ListSessions verifies that ListSessions returns rows ordered
// most-recent first and that the limit parameter is respected.
func TestStore_ListSessions(t *testing.T) {
	s := newTestStore(t)

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		require.NoError(t, s.CreateSession(memory.Session{
			ID:         fmt.Sprintf("sess-%02d", i),
			StartedAt:  base.Add(time.Duration(i) * time.Hour),
			PassNumber: i + 1,
		}))
	}

	all, err := s.ListSessions(0)
	require.NoError(t, err)
	assert.Len(t, all, 5)
	// Most recent first: sess-04 should be first.
	assert.Equal(t, "sess-04", all[0].ID)
	assert.Equal(t, "sess-00", all[4].ID)

	limited, err := s.ListSessions(3)
	require.NoError(t, err)
	assert.Len(t, limited, 3)
	assert.Equal(t, "sess-04", limited[0].ID)
}

// TestStore_LatestPassNumber verifies the incrementing pass counter.
func TestStore_LatestPassNumber(t *testing.T) {
	s := newTestStore(t)

	// Empty store returns 0.
	n, err := s.LatestPassNumber()
	require.NoError(t, err)
	assert.Equal(t, 0, n)

	require.NoError(t, s.CreateSession(memory.Session{
		ID: "p1", StartedAt: time.Now().UTC(), PassNumber: 3,
	}))
	require.NoError(t, s.CreateSession(memory.Session{
		ID: "p2", StartedAt: time.Now().UTC(), PassNumber: 7,
	}))

	n, err = s.LatestPassNumber()
	require.NoError(t, err)
	assert.Equal(t, 7, n)
}
