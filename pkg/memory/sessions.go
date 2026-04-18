// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package memory

import (
	"database/sql"
	"fmt"
	"time"
)

// Session represents a single HelixQA test run.
type Session struct {
	ID            string
	StartedAt     time.Time
	EndedAt       time.Time
	Duration      int
	Platforms     string
	CoveragePct   float64
	TotalTests    int
	Passed        int
	Failed        int
	FindingsCount int
	PassNumber    int
	Notes         string
}

// SessionUpdate carries the mutable fields that may be set when a session
// is closed or amended.
type SessionUpdate struct {
	EndedAt       *time.Time
	Duration      int
	TotalTests    int
	Passed        int
	Failed        int
	FindingsCount int
	CoveragePct   float64
	Notes         string
}

// CreateSession inserts a new session row. The ID must be unique.
func (s *Store) CreateSession(sess Session) error {
	const q = `
		INSERT INTO sessions
			(id, started_at, ended_at, duration_seconds, platforms,
			 coverage_pct, total_tests, passed, failed, findings_count,
			 pass_number, notes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	var endedAt sql.NullString
	if !sess.EndedAt.IsZero() {
		endedAt = sql.NullString{String: sess.EndedAt.UTC().Format(time.RFC3339), Valid: true}
	}

	_, err := s.db.Exec(q,
		sess.ID,
		sess.StartedAt.UTC().Format(time.RFC3339),
		endedAt,
		sess.Duration,
		sess.Platforms,
		sess.CoveragePct,
		sess.TotalTests,
		sess.Passed,
		sess.Failed,
		sess.FindingsCount,
		sess.PassNumber,
		sess.Notes,
	)
	if err != nil {
		return fmt.Errorf("memory: create session %q: %w", sess.ID, err)
	}
	return nil
}

// GetSession retrieves a session by ID. Returns (nil, nil) when not found.
func (s *Store) GetSession(id string) (*Session, error) {
	const q = `
		SELECT id, started_at, ended_at, duration_seconds, platforms,
		       coverage_pct, total_tests, passed, failed, findings_count,
		       pass_number, notes
		FROM sessions WHERE id = ?`

	row := s.db.QueryRow(q, id)
	sess, err := scanSession(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("memory: get session %q: %w", id, err)
	}
	return sess, nil
}

// UpdateSession applies the supplied SessionUpdate to the row identified by id.
func (s *Store) UpdateSession(id string, u SessionUpdate) error {
	const q = `
		UPDATE sessions SET
			ended_at         = ?,
			duration_seconds = ?,
			total_tests      = ?,
			passed           = ?,
			failed           = ?,
			findings_count   = ?,
			coverage_pct     = ?,
			notes            = ?
		WHERE id = ?`

	var endedAt sql.NullString
	if u.EndedAt != nil && !u.EndedAt.IsZero() {
		endedAt = sql.NullString{
			String: u.EndedAt.UTC().Format(time.RFC3339),
			Valid:  true,
		}
	}

	_, err := s.db.Exec(q,
		endedAt,
		u.Duration,
		u.TotalTests,
		u.Passed,
		u.Failed,
		u.FindingsCount,
		u.CoveragePct,
		u.Notes,
		id,
	)
	if err != nil {
		return fmt.Errorf("memory: update session %q: %w", id, err)
	}
	return nil
}

// ListSessions returns sessions ordered by started_at DESC (most recent first).
// When limit is 0 all rows are returned; otherwise at most limit rows.
func (s *Store) ListSessions(limit int) ([]Session, error) {
	q := `
		SELECT id, started_at, ended_at, duration_seconds, platforms,
		       coverage_pct, total_tests, passed, failed, findings_count,
		       pass_number, notes
		FROM sessions
		ORDER BY started_at DESC`
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := s.db.Query(q)
	if err != nil {
		return nil, fmt.Errorf("memory: list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		sess, err := scanSession(rows)
		if err != nil {
			return nil, fmt.Errorf("memory: scan session row: %w", err)
		}
		sessions = append(sessions, *sess)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("memory: list sessions rows: %w", err)
	}
	return sessions, nil
}

// LatestPassNumber returns the highest pass_number stored across all sessions,
// or 0 if no sessions exist yet.
func (s *Store) LatestPassNumber() (int, error) {
	const q = `SELECT COALESCE(MAX(pass_number), 0) FROM sessions`
	var n int
	if err := s.db.QueryRow(q).Scan(&n); err != nil {
		return 0, fmt.Errorf("memory: latest pass number: %w", err)
	}
	return n, nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanSession(r rowScanner) (*Session, error) {
	var (
		sess         Session
		endedAtStr   sql.NullString
		durationSecs sql.NullFloat64
		startedAtStr string
	)

	err := r.Scan(
		&sess.ID,
		&startedAtStr,
		&endedAtStr,
		&durationSecs,
		&sess.Platforms,
		&sess.CoveragePct,
		&sess.TotalTests,
		&sess.Passed,
		&sess.Failed,
		&sess.FindingsCount,
		&sess.PassNumber,
		&sess.Notes,
	)
	if err != nil {
		return nil, err
	}

	t, err := time.Parse(time.RFC3339, startedAtStr)
	if err != nil {
		return nil, fmt.Errorf("parse started_at %q: %w", startedAtStr, err)
	}
	sess.StartedAt = t

	if endedAtStr.Valid && endedAtStr.String != "" {
		t2, err := time.Parse(time.RFC3339, endedAtStr.String)
		if err != nil {
			return nil, fmt.Errorf("parse ended_at %q: %w", endedAtStr.String, err)
		}
		sess.EndedAt = t2
	}

	if durationSecs.Valid {
		sess.Duration = int(durationSecs.Float64)
	}

	return &sess, nil
}
