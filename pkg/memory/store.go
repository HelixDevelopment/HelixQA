// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package memory provides a SQLite-backed persistent store for
// HelixQA's photographic memory: sessions, test results, findings,
// screenshots, metrics, knowledge, and coverage data.
package memory

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

// Store wraps a SQLite database with schema lifecycle management.
// All public methods are safe for concurrent use.
type Store struct {
	db     *sql.DB
	closed bool
	mu     sync.Mutex
	once   sync.Once
}

// NewStore opens (or creates) a SQLite database at dbPath, enables
// WAL journal mode and a busy timeout, then runs idempotent schema
// migrations. Parent directories are created as needed.
//
// The caller is responsible for calling Close when done.
func NewStore(dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("memory: create parent dirs for %q: %w",
			dbPath, err)
	}

	dsn := dbPath +
		"?_journal_mode=WAL" +
		"&_busy_timeout=5000"

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("memory: open sqlite3 %q: %w", dbPath, err)
	}

	// Verify the connection is actually usable.
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("memory: ping %q: %w", dbPath, err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("memory: migrate %q: %w", dbPath, err)
	}

	return s, nil
}

// DB returns the underlying *sql.DB for direct queries.
// The caller must not close the returned handle; use Store.Close instead.
func (s *Store) DB() *sql.DB {
	return s.db
}

// Close releases the database connection. It is safe to call Close
// multiple times; subsequent calls are no-ops and return nil.
func (s *Store) Close() error {
	var closeErr error
	s.once.Do(func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.closed = true
		closeErr = s.db.Close()
	})
	return closeErr
}

// migrate creates all HelixQA schema tables and indexes.
// All statements use CREATE TABLE IF NOT EXISTS / CREATE INDEX IF NOT EXISTS
// so the function is safe to call on an existing database.
func (s *Store) migrate() error {
	stmts := []string{
		// ── sessions ────────────────────────────────────────────────
		`CREATE TABLE IF NOT EXISTS sessions (
			id               TEXT    PRIMARY KEY,
			started_at       TEXT    NOT NULL,
			ended_at         TEXT,
			duration_seconds REAL,
			platforms        TEXT,
			coverage_pct     REAL,
			total_tests      INTEGER NOT NULL DEFAULT 0,
			passed           INTEGER NOT NULL DEFAULT 0,
			failed           INTEGER NOT NULL DEFAULT 0,
			findings_count   INTEGER NOT NULL DEFAULT 0,
			pass_number      INTEGER NOT NULL DEFAULT 0,
			notes            TEXT
		)`,

		// ── test_results ────────────────────────────────────────────
		`CREATE TABLE IF NOT EXISTS test_results (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id     TEXT    NOT NULL REFERENCES sessions(id),
			test_case_id   TEXT    NOT NULL,
			platform       TEXT    NOT NULL,
			status         TEXT    NOT NULL,
			duration_ms    INTEGER NOT NULL DEFAULT 0,
			evidence_paths TEXT,
			error_message  TEXT,
			created_at     TEXT    NOT NULL
		)`,

		// ── findings ────────────────────────────────────────────────
		`CREATE TABLE IF NOT EXISTS findings (
			id                  TEXT    PRIMARY KEY,
			session_id          TEXT    NOT NULL REFERENCES sessions(id),
			severity            TEXT    NOT NULL,
			category            TEXT    NOT NULL,
			title               TEXT    NOT NULL,
			description         TEXT,
			repro_steps         TEXT,
			acceptance_criteria TEXT,
			evidence_paths      TEXT,
			platform            TEXT,
			screen              TEXT,
			status              TEXT    NOT NULL DEFAULT 'open',
			found_date          TEXT,
			fixed_date          TEXT,
			verified_date       TEXT,
			created_at          TEXT    NOT NULL,
			updated_at          TEXT    NOT NULL
		)`,

		// ── screenshots ─────────────────────────────────────────────
		`CREATE TABLE IF NOT EXISTS screenshots (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id  TEXT    NOT NULL REFERENCES sessions(id),
			screen_name TEXT    NOT NULL,
			platform    TEXT    NOT NULL,
			file_path   TEXT    NOT NULL,
			width       INTEGER,
			height      INTEGER,
			hash        TEXT,
			created_at  TEXT    NOT NULL
		)`,

		// ── metrics ─────────────────────────────────────────────────
		`CREATE TABLE IF NOT EXISTS metrics (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id  TEXT    NOT NULL REFERENCES sessions(id),
			platform    TEXT    NOT NULL,
			metric_type TEXT    NOT NULL,
			value       REAL    NOT NULL,
			timestamp   TEXT    NOT NULL
		)`,

		// ── knowledge ───────────────────────────────────────────────
		`CREATE TABLE IF NOT EXISTS knowledge (
			key           TEXT PRIMARY KEY,
			value         TEXT NOT NULL,
			source        TEXT,
			last_verified TEXT
		)`,

		// ── coverage ────────────────────────────────────────────────
		`CREATE TABLE IF NOT EXISTS coverage (
			screen_name  TEXT NOT NULL,
			platform     TEXT NOT NULL,
			last_tested  TEXT,
			times_tested INTEGER NOT NULL DEFAULT 0,
			last_status  TEXT,
			PRIMARY KEY (screen_name, platform)
		)`,

		// ── indexes ─────────────────────────────────────────────────
		`CREATE INDEX IF NOT EXISTS idx_test_results_session_id
			ON test_results(session_id)`,

		`CREATE INDEX IF NOT EXISTS idx_findings_session_id
			ON findings(session_id)`,

		`CREATE INDEX IF NOT EXISTS idx_findings_status
			ON findings(status)`,

		`CREATE INDEX IF NOT EXISTS idx_screenshots_session_id
			ON screenshots(session_id)`,

		`CREATE INDEX IF NOT EXISTS idx_metrics_session_id
			ON metrics(session_id)`,

		// ── schema migrations (idempotent) ──────────────────────────
		`ALTER TABLE findings ADD COLUMN acceptance_criteria TEXT`,
	}

	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			// SQLite ALTER TABLE ADD COLUMN is not natively
			// idempotent; ignore duplicate column errors.
			if strings.Contains(err.Error(), "duplicate column name") {
				continue
			}
			return fmt.Errorf("execute migration statement: %w", err)
		}
	}

	return nil
}
