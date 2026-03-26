// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package memory

import (
	"database/sql"
	"fmt"
	"time"
)

// SetKnowledge upserts a key-value pair into the knowledge store.
// On conflict the existing row's value, source, and last_verified are updated.
func (s *Store) SetKnowledge(key, value, source string) error {
	const q = `
		INSERT INTO knowledge (key, value, source, last_verified)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			value         = excluded.value,
			source        = excluded.source,
			last_verified = excluded.last_verified`

	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(q, key, value, source, now)
	if err != nil {
		return fmt.Errorf("memory: set knowledge %q: %w", key, err)
	}
	return nil
}

// GetKnowledge retrieves the value stored under key.
// Returns an error when the key does not exist.
func (s *Store) GetKnowledge(key string) (string, error) {
	const q = `SELECT value FROM knowledge WHERE key = ?`

	var value string
	err := s.db.QueryRow(q, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("memory: knowledge key %q not found", key)
	}
	if err != nil {
		return "", fmt.Errorf("memory: get knowledge %q: %w", key, err)
	}
	return value, nil
}

// AllKnowledge returns every key-value pair in the knowledge store as a map.
func (s *Store) AllKnowledge() (map[string]string, error) {
	const q = `SELECT key, value FROM knowledge ORDER BY key ASC`

	rows, err := s.db.Query(q)
	if err != nil {
		return nil, fmt.Errorf("memory: all knowledge: %w", err)
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("memory: scan knowledge row: %w", err)
		}
		result[k] = v
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("memory: all knowledge rows: %w", err)
	}
	return result, nil
}
