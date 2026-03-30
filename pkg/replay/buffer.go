// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package replay provides an action replay buffer that records
// successful navigation sequences and can replay them in future
// sessions when encountering a matching screen state. Sequences
// are persisted to SQLite so they survive process restarts.
package replay

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// ActionSequence is a recorded sequence of successful actions.
type ActionSequence struct {
	// ID uniquely identifies this sequence.
	ID string `json:"id"`

	// Platform is the target platform (e.g. "android",
	// "androidtv", "web").
	Platform string `json:"platform"`

	// Actions is the ordered list of recorded actions.
	Actions []RecordedAction `json:"actions"`

	// CreatedAt is when this sequence was first recorded.
	CreatedAt time.Time `json:"created_at"`

	// SuccessCount tracks how many times this sequence has
	// been replayed successfully.
	SuccessCount int `json:"success_count"`
}

// RecordedAction is a single action in a recorded sequence.
type RecordedAction struct {
	// Type is the action type (e.g. "dpad_down", "type",
	// "tap", "back").
	Type string `json:"type"`

	// Value is the action value (e.g. text to type or
	// coordinates).
	Value string `json:"value,omitempty"`

	// ScreenHash is the SHA-256 hash of the screenshot
	// captured before this action was performed.
	ScreenHash string `json:"screen_hash"`
}

// ReplayBuffer stores and retrieves known-good action
// sequences. All public methods are safe for concurrent use.
type ReplayBuffer struct {
	mu        sync.RWMutex
	sequences []ActionSequence
	db        *sql.DB
	dbPath    string
	closed    bool
	once      sync.Once
}

// NewReplayBuffer opens (or creates) a SQLite database at
// dbPath and loads any previously persisted sequences. Parent
// directories are created as needed.
func NewReplayBuffer(dbPath string) (*ReplayBuffer, error) {
	if err := os.MkdirAll(
		filepath.Dir(dbPath), 0o755,
	); err != nil {
		return nil, fmt.Errorf(
			"replay: create parent dirs for %q: %w",
			dbPath, err,
		)
	}

	dsn := dbPath +
		"?_journal_mode=WAL" +
		"&_busy_timeout=5000"

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf(
			"replay: open sqlite3 %q: %w", dbPath, err,
		)
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf(
			"replay: ping %q: %w", dbPath, err,
		)
	}

	rb := &ReplayBuffer{
		db:     db,
		dbPath: dbPath,
	}
	if err := rb.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf(
			"replay: migrate %q: %w", dbPath, err,
		)
	}

	if err := rb.loadAll(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf(
			"replay: load sequences: %w", err,
		)
	}

	return rb, nil
}

// Record saves a successful action sequence to memory and
// persists it to the database. If a sequence with the same ID
// already exists it is replaced.
func (rb *ReplayBuffer) Record(seq ActionSequence) error {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.closed {
		return fmt.Errorf("replay: buffer is closed")
	}

	if seq.ID == "" {
		return fmt.Errorf("replay: sequence ID must not be empty")
	}
	if len(seq.Actions) == 0 {
		return fmt.Errorf(
			"replay: sequence must have at least one action",
		)
	}
	if seq.CreatedAt.IsZero() {
		seq.CreatedAt = time.Now()
	}

	actionsJSON, err := json.Marshal(seq.Actions)
	if err != nil {
		return fmt.Errorf(
			"replay: marshal actions: %w", err,
		)
	}

	// Compute the starting screen hash from the first
	// action for efficient lookup.
	startHash := ""
	if len(seq.Actions) > 0 {
		startHash = seq.Actions[0].ScreenHash
	}

	_, err = rb.db.Exec(`
		INSERT OR REPLACE INTO replay_sequences
			(id, platform, actions, start_hash,
			 created_at, success_count)
		VALUES (?, ?, ?, ?, ?, ?)`,
		seq.ID,
		seq.Platform,
		string(actionsJSON),
		startHash,
		seq.CreatedAt.Format(time.RFC3339),
		seq.SuccessCount,
	)
	if err != nil {
		return fmt.Errorf(
			"replay: insert sequence %q: %w", seq.ID, err,
		)
	}

	// Update in-memory cache.
	replaced := false
	for i, s := range rb.sequences {
		if s.ID == seq.ID {
			rb.sequences[i] = seq
			replaced = true
			break
		}
	}
	if !replaced {
		rb.sequences = append(rb.sequences, seq)
	}
	return nil
}

// FindMatch looks for a known sequence whose first action
// starts from a screen matching the given hash and platform.
// Returns the sequence with the highest success count, or nil
// if no match is found.
func (rb *ReplayBuffer) FindMatch(
	screenHash string, platform string,
) *ActionSequence {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if screenHash == "" {
		return nil
	}

	var best *ActionSequence
	for i := range rb.sequences {
		seq := &rb.sequences[i]
		if seq.Platform != platform {
			continue
		}
		if len(seq.Actions) == 0 {
			continue
		}
		if seq.Actions[0].ScreenHash != screenHash {
			continue
		}
		if best == nil ||
			seq.SuccessCount > best.SuccessCount {
			best = seq
		}
	}

	if best == nil {
		return nil
	}

	// Return a copy to prevent races.
	result := *best
	result.Actions = make(
		[]RecordedAction, len(best.Actions),
	)
	copy(result.Actions, best.Actions)
	return &result
}

// MarkSuccess increments the success count for a sequence
// both in memory and in the database.
func (rb *ReplayBuffer) MarkSuccess(seqID string) error {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.closed {
		return fmt.Errorf("replay: buffer is closed")
	}

	found := false
	for i := range rb.sequences {
		if rb.sequences[i].ID == seqID {
			rb.sequences[i].SuccessCount++
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf(
			"replay: sequence %q not found", seqID,
		)
	}

	_, err := rb.db.Exec(`
		UPDATE replay_sequences
		SET success_count = success_count + 1
		WHERE id = ?`, seqID,
	)
	if err != nil {
		return fmt.Errorf(
			"replay: update success count for %q: %w",
			seqID, err,
		)
	}
	return nil
}

// Len returns the number of stored sequences.
func (rb *ReplayBuffer) Len() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return len(rb.sequences)
}

// All returns a copy of all stored sequences.
func (rb *ReplayBuffer) All() []ActionSequence {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	result := make([]ActionSequence, len(rb.sequences))
	for i, seq := range rb.sequences {
		result[i] = seq
		result[i].Actions = make(
			[]RecordedAction, len(seq.Actions),
		)
		copy(result[i].Actions, seq.Actions)
	}
	return result
}

// Delete removes a sequence by ID from both memory and the
// database.
func (rb *ReplayBuffer) Delete(seqID string) error {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.closed {
		return fmt.Errorf("replay: buffer is closed")
	}

	_, err := rb.db.Exec(
		`DELETE FROM replay_sequences WHERE id = ?`,
		seqID,
	)
	if err != nil {
		return fmt.Errorf(
			"replay: delete sequence %q: %w", seqID, err,
		)
	}

	for i, s := range rb.sequences {
		if s.ID == seqID {
			rb.sequences = append(
				rb.sequences[:i], rb.sequences[i+1:]...,
			)
			break
		}
	}
	return nil
}

// Close releases the database connection. Safe to call
// multiple times.
func (rb *ReplayBuffer) Close() error {
	var closeErr error
	rb.once.Do(func() {
		rb.mu.Lock()
		defer rb.mu.Unlock()
		rb.closed = true
		closeErr = rb.db.Close()
	})
	return closeErr
}

// ScreenHash computes a SHA-256 hash of screenshot data.
// This is a convenience function for callers that need to
// compute the hash before calling FindMatch or Record.
func ScreenHash(screenshot []byte) string {
	if len(screenshot) == 0 {
		return ""
	}
	h := sha256.Sum256(screenshot)
	return hex.EncodeToString(h[:])
}

// migrate creates the replay_sequences table if it does not
// exist.
func (rb *ReplayBuffer) migrate() error {
	_, err := rb.db.Exec(`
		CREATE TABLE IF NOT EXISTS replay_sequences (
			id            TEXT PRIMARY KEY,
			platform      TEXT NOT NULL,
			actions       TEXT NOT NULL,
			start_hash    TEXT NOT NULL,
			created_at    TEXT NOT NULL,
			success_count INTEGER NOT NULL DEFAULT 0
		)`)
	if err != nil {
		return fmt.Errorf("create replay_sequences: %w", err)
	}

	_, err = rb.db.Exec(`
		CREATE INDEX IF NOT EXISTS
			idx_replay_sequences_start_hash
		ON replay_sequences(start_hash, platform)`)
	if err != nil {
		return fmt.Errorf("create index: %w", err)
	}

	return nil
}

// loadAll reads all persisted sequences into memory.
func (rb *ReplayBuffer) loadAll() error {
	rows, err := rb.db.Query(`
		SELECT id, platform, actions, created_at,
			   success_count
		FROM replay_sequences
		ORDER BY success_count DESC`)
	if err != nil {
		return fmt.Errorf("query sequences: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id           string
			platform     string
			actionsJSON  string
			createdAtStr string
			successCount int
		)
		if err := rows.Scan(
			&id, &platform, &actionsJSON,
			&createdAtStr, &successCount,
		); err != nil {
			return fmt.Errorf("scan sequence row: %w", err)
		}

		var actions []RecordedAction
		if err := json.Unmarshal(
			[]byte(actionsJSON), &actions,
		); err != nil {
			// Skip corrupted rows.
			continue
		}

		createdAt, _ := time.Parse(
			time.RFC3339, createdAtStr,
		)

		rb.sequences = append(rb.sequences, ActionSequence{
			ID:           id,
			Platform:     platform,
			Actions:      actions,
			CreatedAt:    createdAt,
			SuccessCount: successCount,
		})
	}
	return rows.Err()
}
