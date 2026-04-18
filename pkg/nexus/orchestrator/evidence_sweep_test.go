// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestFileEvidenceStore_Sweep_P4_MaxAge locks in P4 from
// docs/nexus/remaining-work.md: items older than RetentionPolicy.MaxAge
// are evicted; newer items are kept.
func TestFileEvidenceStore_Sweep_P4_MaxAge(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileEvidenceStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	oldPath, _ := store.Put("old/frame.png", []byte("stale"))
	newPath, _ := store.Put("new/frame.png", []byte("fresh"))

	// Backdate the old file beyond the policy window.
	past := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(oldPath, past, past); err != nil {
		t.Fatal(err)
	}

	result, err := store.Sweep(RetentionPolicy{MaxAge: 1 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Deleted) != 1 {
		t.Errorf("expected 1 deleted, got %d: %v", len(result.Deleted), result.Deleted)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("old file must be removed")
	}
	if _, err := os.Stat(newPath); err != nil {
		t.Error("new file must be kept")
	}
}

// TestFileEvidenceStore_Sweep_P4_MaxItems keeps only the N newest.
func TestFileEvidenceStore_Sweep_P4_MaxItems(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewFileEvidenceStore(dir)

	paths := []string{}
	for i := 0; i < 10; i++ {
		p, _ := store.Put(filepath.Join("item", "file"+itoa(i)+".png"), []byte("x"))
		paths = append(paths, p)
		// Spread mtimes so the sweep can order them.
		stamp := time.Now().Add(time.Duration(-i) * time.Minute)
		_ = os.Chtimes(p, stamp, stamp)
	}

	result, err := store.Sweep(RetentionPolicy{MaxItems: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Deleted) != 7 {
		t.Errorf("expected 7 deletions, got %d", len(result.Deleted))
	}
	remaining, _ := store.List()
	if len(remaining) != 3 {
		t.Errorf("kept %d items, want 3", len(remaining))
	}
}

// TestFileEvidenceStore_Sweep_P4_MaxBytes caps total footprint.
func TestFileEvidenceStore_Sweep_P4_MaxBytes(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewFileEvidenceStore(dir)
	for i := 0; i < 5; i++ {
		p, _ := store.Put(filepath.Join("item", "file"+itoa(i)+".bin"), make([]byte, 1024))
		stamp := time.Now().Add(time.Duration(-i) * time.Minute)
		_ = os.Chtimes(p, stamp, stamp)
	}
	_, err := store.Sweep(RetentionPolicy{MaxBytes: 2048})
	if err != nil {
		t.Fatal(err)
	}
	remaining, _ := store.List()
	var total int64
	for _, r := range remaining {
		total += r.Size
	}
	if total > 2048 {
		t.Errorf("footprint %d exceeds MaxBytes=2048", total)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	digits := []byte{}
	for i > 0 {
		digits = append([]byte{byte('0' + i%10)}, digits...)
		i /= 10
	}
	return string(digits)
}
