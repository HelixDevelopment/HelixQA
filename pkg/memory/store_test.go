// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package memory_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/memory"
)

// TestNewStore_CreatesDatabase verifies that NewStore creates
// the SQLite database file at the specified path.
func TestNewStore_CreatesDatabase(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "helixqa.db")

	store, err := memory.NewStore(dbPath)
	require.NoError(t, err)
	defer store.Close()

	_, statErr := os.Stat(dbPath)
	assert.NoError(t, statErr, "database file should exist after NewStore")
}

// TestNewStore_RunsMigrations verifies that all 7 expected tables
// are created by the migration during NewStore.
func TestNewStore_RunsMigrations(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "helixqa.db")

	store, err := memory.NewStore(dbPath)
	require.NoError(t, err)
	defer store.Close()

	tables := []string{
		"sessions",
		"test_results",
		"findings",
		"screenshots",
		"metrics",
		"knowledge",
		"coverage",
	}

	db := store.DB()
	for _, table := range tables {
		row := db.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?",
			table,
		)
		var count int
		err := row.Scan(&count)
		require.NoError(t, err, "query for table %q should not error", table)
		assert.Equal(t, 1, count, "table %q should exist", table)
	}
}

// TestNewStore_IdempotentMigrations verifies that opening a store
// against an existing database (running migrations again) causes no
// error — migrations are idempotent.
func TestNewStore_IdempotentMigrations(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "helixqa.db")

	store1, err := memory.NewStore(dbPath)
	require.NoError(t, err)
	require.NoError(t, store1.Close())

	store2, err := memory.NewStore(dbPath)
	require.NoError(t, err, "second open should not error")
	require.NoError(t, store2.Close())
}

// TestStore_Close verifies that Close succeeds and that a double
// close does not panic or return unexpected errors.
func TestStore_Close(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "helixqa.db")

	store, err := memory.NewStore(dbPath)
	require.NoError(t, err)

	err = store.Close()
	assert.NoError(t, err, "first close should succeed")

	// Double close must not panic.
	assert.NotPanics(t, func() {
		_ = store.Close()
	})
}

// TestNewStore_InvalidPath verifies that NewStore returns a
// descriptive error when given a path where the parent cannot
// be created (e.g., a file used as a directory component).
func TestNewStore_InvalidPath(t *testing.T) {
	dir := t.TempDir()

	// Create a regular file, then try to use it as a directory.
	blocker := filepath.Join(dir, "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o644))

	badPath := filepath.Join(blocker, "sub", "helixqa.db")
	store, err := memory.NewStore(badPath)
	assert.Error(t, err, "NewStore should error on invalid path")
	assert.Nil(t, store, "store should be nil on error")
}
