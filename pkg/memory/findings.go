// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package memory

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Finding represents a single QA issue discovered during a session.
type Finding struct {
	ID                 string
	SessionID          string
	Severity           string
	Category           string
	Title              string
	Description        string
	ReproSteps         string
	EvidencePaths      string
	Platform           string
	Screen             string
	Status             string
	FoundDate          string
	FixedDate          string
	VerifiedDate       string
	AcceptanceCriteria string
}

// CreateFinding inserts a new finding row. The ID must be unique.
func (s *Store) CreateFinding(f Finding) error {
	const q = `
		INSERT INTO findings
			(id, session_id, severity, category, title, description,
			 repro_steps, evidence_paths, platform, screen, status,
			 found_date, fixed_date, verified_date, acceptance_criteria,
			 created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(q,
		f.ID, f.SessionID, f.Severity, f.Category, f.Title,
		f.Description, f.ReproSteps, f.EvidencePaths,
		f.Platform, f.Screen, f.Status,
		nullableString(f.FoundDate),
		nullableString(f.FixedDate),
		nullableString(f.VerifiedDate),
		nullableString(f.AcceptanceCriteria),
		now, now,
	)
	if err != nil {
		return fmt.Errorf("memory: create finding %q: %w", f.ID, err)
	}
	return nil
}

// GetFinding retrieves a finding by ID. Returns (nil, nil) when not found.
func (s *Store) GetFinding(id string) (*Finding, error) {
	const q = `
		SELECT id, session_id, severity, category, title, description,
		       repro_steps, evidence_paths, platform, screen, status,
		       found_date, fixed_date, verified_date, acceptance_criteria
		FROM findings WHERE id = ?`

	row := s.db.QueryRow(q, id)
	f, err := scanFinding(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("memory: get finding %q: %w", id, err)
	}
	return f, nil
}

// UpdateFindingStatus sets the status field (and updated_at) for the given ID.
func (s *Store) UpdateFindingStatus(id, status string) error {
	const q = `UPDATE findings SET status = ?, updated_at = ? WHERE id = ?`
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(q, status, now, id)
	if err != nil {
		return fmt.Errorf("memory: update finding status %q: %w", id, err)
	}
	return nil
}

// ListFindingsByStatus returns all findings with the given status value.
func (s *Store) ListFindingsByStatus(status string) ([]Finding, error) {
	const q = `
		SELECT id, session_id, severity, category, title, description,
		       repro_steps, evidence_paths, platform, screen, status,
		       found_date, fixed_date, verified_date, acceptance_criteria
		FROM findings
		WHERE status = ?
		ORDER BY id ASC`

	rows, err := s.db.Query(q, status)
	if err != nil {
		return nil, fmt.Errorf("memory: list findings by status %q: %w", status, err)
	}
	defer rows.Close()

	var findings []Finding
	for rows.Next() {
		f, err := scanFinding(rows)
		if err != nil {
			return nil, fmt.Errorf("memory: scan finding row: %w", err)
		}
		findings = append(findings, *f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("memory: list findings rows: %w", err)
	}
	return findings, nil
}

// FindDuplicateByTitle returns the first open finding whose
// title matches exactly. Returns (nil, nil) when no duplicate
// exists. Used by FindingsBridge to prevent creating tickets
// for the same issue multiple times.
func (s *Store) FindDuplicateByTitle(
	title string,
) (*Finding, error) {
	const q = `
		SELECT id, session_id, severity, category, title,
		       description, repro_steps, evidence_paths,
		       platform, screen, status,
		       found_date, fixed_date, verified_date, acceptance_criteria
		FROM findings
		WHERE title = ? AND status != 'fixed'
		LIMIT 1`

	row := s.db.QueryRow(q, title)
	f, err := scanFinding(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf(
			"memory: find duplicate by title: %w", err,
		)
	}
	return f, nil
}

// FindRelatedByCategory returns all open findings in the same
// category and platform, for grouping related issues.
func (s *Store) FindRelatedByCategory(
	category, platform string,
) ([]Finding, error) {
	const q = `
		SELECT id, session_id, severity, category, title,
		       description, repro_steps, evidence_paths,
		       platform, screen, status,
		       found_date, fixed_date, verified_date, acceptance_criteria
		FROM findings
		WHERE category = ? AND platform = ?
		  AND status != 'fixed'
		ORDER BY id ASC`

	rows, err := s.db.Query(q, category, platform)
	if err != nil {
		return nil, fmt.Errorf(
			"memory: find related: %w", err,
		)
	}
	defer rows.Close()

	var out []Finding
	for rows.Next() {
		f, err := scanFinding(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *f)
	}
	return out, rows.Err()
}

// NextFindingID returns the next available HELIX-NNN identifier by inspecting
// the highest numeric suffix already stored. Returns "HELIX-001" on an empty
// store.
func (s *Store) NextFindingID() (string, error) {
	const q = `SELECT COALESCE(MAX(CAST(SUBSTR(id, 7) AS INTEGER)), 0) FROM findings`
	var max int
	if err := s.db.QueryRow(q).Scan(&max); err != nil {
		return "", fmt.Errorf("memory: next finding id: %w", err)
	}
	return fmt.Sprintf("HELIX-%03d", max+1), nil
}

// ── Markdown generation ───────────────────────────────────────────────────────

// ToMarkdown renders the finding as a Markdown document with YAML frontmatter
// suitable for storage in a docs/ directory or issue-tracker import.
//
// Format:
//
//	---
//	id: HELIX-NNN
//	severity: ...
//	...
//	---
//	# Title
//
//	Description...
//
//	## Reproduction Steps
//
//	Steps...
//
//	## Evidence
//
//	Paths...
func (f *Finding) ToMarkdown() string {
	var b strings.Builder

	// YAML frontmatter.
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("id: %s\n", f.ID))
	b.WriteString(fmt.Sprintf("severity: %s\n", f.Severity))
	b.WriteString(fmt.Sprintf("category: %s\n", f.Category))
	b.WriteString(fmt.Sprintf("platform: %s\n", f.Platform))
	b.WriteString(fmt.Sprintf("screen: %s\n", f.Screen))
	b.WriteString(fmt.Sprintf("status: %s\n", f.Status))
	if f.FoundDate != "" {
		b.WriteString(fmt.Sprintf("found_date: %s\n", f.FoundDate))
	}
	if f.FixedDate != "" {
		b.WriteString(fmt.Sprintf("fixed_date: %s\n", f.FixedDate))
	}
	if f.VerifiedDate != "" {
		b.WriteString(fmt.Sprintf("verified_date: %s\n", f.VerifiedDate))
	}
	b.WriteString("---\n\n")

	// Title.
	b.WriteString(fmt.Sprintf("# %s\n\n", f.Title))

	// Description.
	if f.Description != "" {
		b.WriteString(f.Description)
		b.WriteString("\n\n")
	}

	// Acceptance criteria.
	if f.AcceptanceCriteria != "" {
		b.WriteString("## Acceptance Criteria\n\n")
		b.WriteString(f.AcceptanceCriteria)
		b.WriteString("\n\n")
	}

	// Reproduction steps.
	if f.ReproSteps != "" {
		b.WriteString("## Reproduction Steps\n\n")
		b.WriteString(f.ReproSteps)
		b.WriteString("\n\n")
	}

	// Evidence.
	if f.EvidencePaths != "" {
		b.WriteString("## Evidence\n\n")
		b.WriteString(f.EvidencePaths)
		b.WriteString("\n")
	}

	return b.String()
}

// WriteToDir writes the finding's Markdown representation to dir as
// `{ID}-{slug}.md` and returns the full path. The directory must exist.
func (f *Finding) WriteToDir(dir string) (string, error) {
	slug := toSlug(f.Title)
	var name string
	if slug != "" {
		name = fmt.Sprintf("%s-%s.md", f.ID, slug)
	} else {
		name = fmt.Sprintf("%s.md", f.ID)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("memory: create issues dir %q: %w", dir, err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(f.ToMarkdown()), 0o644); err != nil {
		return "", fmt.Errorf("memory: write finding %q to %q: %w", f.ID, dir, err)
	}
	return path, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// toSlug converts a title string to a URL/filename-safe lowercase slug.
func toSlug(title string) string {
	s := strings.ToLower(title)
	s = nonAlnum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

// nullableString wraps a string as sql.NullString; empty strings become NULL.
func nullableString(v string) sql.NullString {
	return sql.NullString{String: v, Valid: v != ""}
}

func scanFinding(r rowScanner) (*Finding, error) {
	var (
		f                  Finding
		description        sql.NullString
		reproSteps         sql.NullString
		evidencePaths      sql.NullString
		platform           sql.NullString
		screen             sql.NullString
		foundDate          sql.NullString
		fixedDate          sql.NullString
		verifiedDate       sql.NullString
		acceptanceCriteria sql.NullString
	)

	err := r.Scan(
		&f.ID, &f.SessionID, &f.Severity, &f.Category, &f.Title,
		&description, &reproSteps, &evidencePaths,
		&platform, &screen, &f.Status,
		&foundDate, &fixedDate, &verifiedDate, &acceptanceCriteria,
	)
	if err != nil {
		return nil, err
	}

	f.Description = description.String
	f.ReproSteps = reproSteps.String
	f.EvidencePaths = evidencePaths.String
	f.Platform = platform.String
	f.Screen = screen.String
	f.FoundDate = foundDate.String
	f.FixedDate = fixedDate.String
	f.VerifiedDate = verifiedDate.String
	f.AcceptanceCriteria = acceptanceCriteria.String

	return &f, nil
}
