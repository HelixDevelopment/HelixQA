// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package replay_test

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/replay"
)

func newTestBuffer(t *testing.T) *replay.ReplayBuffer {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "replay.db")
	rb, err := replay.NewReplayBuffer(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rb.Close() })
	return rb
}

func sampleSequence(id, platform, hash string) replay.ActionSequence {
	return replay.ActionSequence{
		ID:       id,
		Platform: platform,
		Actions: []replay.RecordedAction{
			{Type: "dpad_down", Value: "", ScreenHash: hash},
			{Type: "dpad_center", Value: "", ScreenHash: "hash2"},
		},
		CreatedAt:    time.Now(),
		SuccessCount: 0,
	}
}

// TestNewReplayBuffer_CreatesDatabase verifies that
// NewReplayBuffer creates the SQLite database file.
func TestNewReplayBuffer_CreatesDatabase(t *testing.T) {
	rb := newTestBuffer(t)
	assert.Equal(t, 0, rb.Len())
}

// TestNewReplayBuffer_InvalidPath verifies that an invalid
// path returns an error.
func TestNewReplayBuffer_InvalidPath(t *testing.T) {
	rb, err := replay.NewReplayBuffer("/dev/null/impossible/replay.db")
	assert.Error(t, err)
	assert.Nil(t, rb)
}

// TestReplayBuffer_Record_Success verifies that a sequence
// can be recorded and retrieved.
func TestReplayBuffer_Record_Success(t *testing.T) {
	rb := newTestBuffer(t)

	seq := sampleSequence("seq-1", "android", "abc123")
	err := rb.Record(seq)
	require.NoError(t, err)
	assert.Equal(t, 1, rb.Len())

	all := rb.All()
	require.Len(t, all, 1)
	assert.Equal(t, "seq-1", all[0].ID)
	assert.Equal(t, "android", all[0].Platform)
	assert.Len(t, all[0].Actions, 2)
}

// TestReplayBuffer_Record_EmptyID verifies that recording a
// sequence with an empty ID returns an error.
func TestReplayBuffer_Record_EmptyID(t *testing.T) {
	rb := newTestBuffer(t)

	seq := sampleSequence("", "android", "abc123")
	err := rb.Record(seq)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ID must not be empty")
}

// TestReplayBuffer_Record_EmptyActions verifies that recording
// a sequence with no actions returns an error.
func TestReplayBuffer_Record_EmptyActions(t *testing.T) {
	rb := newTestBuffer(t)

	seq := replay.ActionSequence{
		ID:       "seq-empty",
		Platform: "web",
		Actions:  nil,
	}
	err := rb.Record(seq)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one action")
}

// TestReplayBuffer_Record_ReplaceExisting verifies that
// recording a sequence with an existing ID replaces it.
func TestReplayBuffer_Record_ReplaceExisting(t *testing.T) {
	rb := newTestBuffer(t)

	seq1 := sampleSequence("seq-1", "android", "hash-a")
	require.NoError(t, rb.Record(seq1))

	seq2 := replay.ActionSequence{
		ID:       "seq-1",
		Platform: "web",
		Actions: []replay.RecordedAction{
			{Type: "click", Value: "100,200", ScreenHash: "hash-b"},
		},
		SuccessCount: 5,
	}
	require.NoError(t, rb.Record(seq2))

	assert.Equal(t, 1, rb.Len())
	all := rb.All()
	assert.Equal(t, "web", all[0].Platform)
	assert.Equal(t, 5, all[0].SuccessCount)
}

// TestReplayBuffer_FindMatch_Found verifies that FindMatch
// returns the correct sequence when a screen hash matches.
func TestReplayBuffer_FindMatch_Found(t *testing.T) {
	rb := newTestBuffer(t)

	seq := sampleSequence("seq-1", "android", "screen-abc")
	require.NoError(t, rb.Record(seq))

	match := rb.FindMatch("screen-abc", "android")
	require.NotNil(t, match)
	assert.Equal(t, "seq-1", match.ID)
}

// TestReplayBuffer_FindMatch_WrongPlatform verifies that
// FindMatch does not return sequences for a different
// platform.
func TestReplayBuffer_FindMatch_WrongPlatform(t *testing.T) {
	rb := newTestBuffer(t)

	seq := sampleSequence("seq-1", "android", "screen-abc")
	require.NoError(t, rb.Record(seq))

	match := rb.FindMatch("screen-abc", "web")
	assert.Nil(t, match)
}

// TestReplayBuffer_FindMatch_NoMatch verifies that FindMatch
// returns nil when no screen hash matches.
func TestReplayBuffer_FindMatch_NoMatch(t *testing.T) {
	rb := newTestBuffer(t)

	seq := sampleSequence("seq-1", "android", "screen-abc")
	require.NoError(t, rb.Record(seq))

	match := rb.FindMatch("screen-xyz", "android")
	assert.Nil(t, match)
}

// TestReplayBuffer_FindMatch_EmptyHash verifies that
// FindMatch returns nil for an empty hash.
func TestReplayBuffer_FindMatch_EmptyHash(t *testing.T) {
	rb := newTestBuffer(t)
	match := rb.FindMatch("", "android")
	assert.Nil(t, match)
}

// TestReplayBuffer_FindMatch_BestSuccessCount verifies that
// FindMatch returns the sequence with the highest success
// count when multiple sequences match.
func TestReplayBuffer_FindMatch_BestSuccessCount(t *testing.T) {
	rb := newTestBuffer(t)

	seq1 := sampleSequence("seq-low", "android", "same-hash")
	seq1.SuccessCount = 2
	require.NoError(t, rb.Record(seq1))

	seq2 := sampleSequence("seq-high", "android", "same-hash")
	seq2.SuccessCount = 10
	require.NoError(t, rb.Record(seq2))

	match := rb.FindMatch("same-hash", "android")
	require.NotNil(t, match)
	assert.Equal(t, "seq-high", match.ID)
	assert.Equal(t, 10, match.SuccessCount)
}

// TestReplayBuffer_MarkSuccess verifies that MarkSuccess
// increments the success count.
func TestReplayBuffer_MarkSuccess(t *testing.T) {
	rb := newTestBuffer(t)

	seq := sampleSequence("seq-1", "android", "hash-a")
	require.NoError(t, rb.Record(seq))

	require.NoError(t, rb.MarkSuccess("seq-1"))
	require.NoError(t, rb.MarkSuccess("seq-1"))

	all := rb.All()
	require.Len(t, all, 1)
	assert.Equal(t, 2, all[0].SuccessCount)
}

// TestReplayBuffer_MarkSuccess_NotFound verifies that
// MarkSuccess returns an error for an unknown sequence.
func TestReplayBuffer_MarkSuccess_NotFound(t *testing.T) {
	rb := newTestBuffer(t)
	err := rb.MarkSuccess("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestReplayBuffer_Delete verifies that Delete removes a
// sequence.
func TestReplayBuffer_Delete(t *testing.T) {
	rb := newTestBuffer(t)

	seq := sampleSequence("seq-1", "android", "hash-a")
	require.NoError(t, rb.Record(seq))
	assert.Equal(t, 1, rb.Len())

	require.NoError(t, rb.Delete("seq-1"))
	assert.Equal(t, 0, rb.Len())
}

// TestReplayBuffer_Persistence verifies that sequences
// survive closing and reopening the database.
func TestReplayBuffer_Persistence(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "replay.db")

	// Create and populate.
	rb1, err := replay.NewReplayBuffer(dbPath)
	require.NoError(t, err)

	seq := sampleSequence("seq-persist", "android", "hash-p")
	seq.SuccessCount = 3
	require.NoError(t, rb1.Record(seq))
	require.NoError(t, rb1.Close())

	// Reopen and verify.
	rb2, err := replay.NewReplayBuffer(dbPath)
	require.NoError(t, err)
	defer rb2.Close()

	assert.Equal(t, 1, rb2.Len())
	match := rb2.FindMatch("hash-p", "android")
	require.NotNil(t, match)
	assert.Equal(t, "seq-persist", match.ID)
	assert.Equal(t, 3, match.SuccessCount)
}

// TestReplayBuffer_Close_DoubleClose verifies that closing
// twice does not panic.
func TestReplayBuffer_Close_DoubleClose(t *testing.T) {
	rb := newTestBuffer(t)
	require.NoError(t, rb.Close())
	assert.NotPanics(t, func() {
		_ = rb.Close()
	})
}

// TestReplayBuffer_RecordAfterClose verifies that Record
// returns an error after the buffer is closed.
func TestReplayBuffer_RecordAfterClose(t *testing.T) {
	rb := newTestBuffer(t)
	require.NoError(t, rb.Close())

	seq := sampleSequence("seq-late", "android", "hash-l")
	err := rb.Record(seq)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "closed")
}

// TestScreenHash verifies the ScreenHash helper function.
func TestScreenHash(t *testing.T) {
	h1 := replay.ScreenHash([]byte("screenshot-data-1"))
	h2 := replay.ScreenHash([]byte("screenshot-data-2"))
	h3 := replay.ScreenHash([]byte("screenshot-data-1"))

	assert.NotEmpty(t, h1)
	assert.NotEmpty(t, h2)
	assert.NotEqual(t, h1, h2)
	assert.Equal(t, h1, h3)
	assert.Len(t, h1, 64) // SHA-256 hex = 64 chars
}

// TestScreenHash_Empty verifies that an empty screenshot
// returns an empty hash.
func TestScreenHash_Empty(t *testing.T) {
	assert.Equal(t, "", replay.ScreenHash(nil))
	assert.Equal(t, "", replay.ScreenHash([]byte{}))
}

// TestReplayBuffer_All_ReturnsCopy verifies that All returns
// a copy that does not affect the buffer.
func TestReplayBuffer_All_ReturnsCopy(t *testing.T) {
	rb := newTestBuffer(t)

	seq := sampleSequence("seq-1", "android", "hash-a")
	require.NoError(t, rb.Record(seq))

	all := rb.All()
	all[0].ID = "mutated"
	all[0].Actions[0].Type = "mutated"

	original := rb.All()
	assert.Equal(t, "seq-1", original[0].ID)
	assert.Equal(t, "dpad_down", original[0].Actions[0].Type)
}

// TestReplayBuffer_FindMatch_ReturnsCopy verifies that
// FindMatch returns a copy.
func TestReplayBuffer_FindMatch_ReturnsCopy(t *testing.T) {
	rb := newTestBuffer(t)

	seq := sampleSequence("seq-1", "android", "hash-a")
	require.NoError(t, rb.Record(seq))

	match := rb.FindMatch("hash-a", "android")
	require.NotNil(t, match)
	match.Actions[0].Type = "mutated"

	match2 := rb.FindMatch("hash-a", "android")
	assert.Equal(t, "dpad_down", match2.Actions[0].Type)
}

// TestReplayBuffer_Stress_ConcurrentOperations verifies
// thread safety under concurrent access.
func TestReplayBuffer_Stress_ConcurrentOperations(t *testing.T) {
	rb := newTestBuffer(t)
	const goroutines = 10
	const ops = 20

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(gID int) {
			defer wg.Done()
			for i := 0; i < ops; i++ {
				seq := replay.ActionSequence{
					ID:       fmt.Sprintf("seq-%d-%d", gID, i),
					Platform: "android",
					Actions: []replay.RecordedAction{
						{
							Type:       "dpad_down",
							ScreenHash: fmt.Sprintf("hash-%d", gID),
						},
					},
				}
				_ = rb.Record(seq)
				_ = rb.FindMatch(
					fmt.Sprintf("hash-%d", gID), "android",
				)
				_ = rb.Len()
				_ = rb.All()
			}
		}(g)
	}
	wg.Wait()

	assert.Equal(t, goroutines*ops, rb.Len())
}
