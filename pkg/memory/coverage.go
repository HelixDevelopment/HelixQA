// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package memory

import (
	"database/sql"
	"fmt"
	"time"
)

// CoverageEntry records how many times a named screen on a given platform
// has been exercised by HelixQA and what the most recent outcome was.
type CoverageEntry struct {
	ScreenName  string
	Platform    string
	LastTested  time.Time
	TimesTested int
	LastStatus  string
}

// RecordCoverage upserts a coverage row for (screen, platform).
// On conflict it increments times_tested and updates last_tested and
// last_status.
func (s *Store) RecordCoverage(screen, platform, status string) error {
	const q = `
		INSERT INTO coverage (screen_name, platform, last_tested, times_tested, last_status)
		VALUES (?, ?, ?, 1, ?)
		ON CONFLICT(screen_name, platform) DO UPDATE SET
			times_tested = times_tested + 1,
			last_tested  = excluded.last_tested,
			last_status  = excluded.last_status`

	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(q, screen, platform, now, status)
	if err != nil {
		return fmt.Errorf("memory: record coverage %q/%q: %w", screen, platform, err)
	}
	return nil
}

// GetCoverage retrieves the coverage entry for (screen, platform).
// Returns an error wrapping sql.ErrNoRows when the entry does not exist.
func (s *Store) GetCoverage(screen, platform string) (*CoverageEntry, error) {
	const q = `
		SELECT screen_name, platform, last_tested, times_tested, last_status
		FROM coverage
		WHERE screen_name = ? AND platform = ?`

	row := s.db.QueryRow(q, screen, platform)
	entry, err := scanCoverageEntry(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("memory: coverage entry %q/%q: %w", screen, platform, sql.ErrNoRows)
	}
	if err != nil {
		return nil, fmt.Errorf("memory: get coverage %q/%q: %w", screen, platform, err)
	}
	return entry, nil
}

// ListUncoveredScreens returns the elements of allScreens that have no
// coverage row for the given platform.
func (s *Store) ListUncoveredScreens(allScreens []string, platform string) []string {
	const q = `SELECT screen_name FROM coverage WHERE platform = ?`

	rows, err := s.db.Query(q, platform)
	if err != nil {
		// On any query error treat everything as uncovered.
		return allScreens
	}
	defer rows.Close()

	covered := make(map[string]struct{})
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			covered[name] = struct{}{}
		}
	}

	var uncovered []string
	for _, screen := range allScreens {
		if _, ok := covered[screen]; !ok {
			uncovered = append(uncovered, screen)
		}
	}
	return uncovered
}

// ── helpers ───────────────────────────────────────────────────────────────────

func scanCoverageEntry(r rowScanner) (*CoverageEntry, error) {
	var (
		entry         CoverageEntry
		lastTestedStr sql.NullString
		lastStatus    sql.NullString
	)

	err := r.Scan(
		&entry.ScreenName,
		&entry.Platform,
		&lastTestedStr,
		&entry.TimesTested,
		&lastStatus,
	)
	if err != nil {
		return nil, err
	}

	if lastTestedStr.Valid && lastTestedStr.String != "" {
		t, err := time.Parse(time.RFC3339, lastTestedStr.String)
		if err != nil {
			return nil, fmt.Errorf("parse last_tested %q: %w", lastTestedStr.String, err)
		}
		entry.LastTested = t
	}

	entry.LastStatus = lastStatus.String
	return &entry, nil
}
