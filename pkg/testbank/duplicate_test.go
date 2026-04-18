// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package testbank

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadFile_P9_RejectsDuplicateIDInsideOneBank locks in P9 from
// docs/nexus/remaining-work.md: a YAML bank with two test cases
// sharing the same id used to silently collapse downstream;
// LoadFile now refuses the file up front.
func TestLoadFile_P9_RejectsDuplicateIDInsideOneBank(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dup.yaml")
	content := []byte(`name: duplicate-ids
test_cases:
  - id: NX-GEN-demo
    name: First
    platforms: [api]
    steps:
      - name: step 1
        action: "a"
        expected: "ok"
  - id: NX-GEN-demo
    name: Second
    platforms: [api]
    steps:
      - name: step 1
        action: "b"
        expected: "ok"
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadFile(path)
	if err == nil {
		t.Fatal("expected duplicate id error")
	}
	if !strings.Contains(err.Error(), "duplicate test case id") {
		t.Errorf("error must mention duplicate id: %v", err)
	}
}

// TestLoadDir_P9_RejectsDuplicateIDAcrossBanks covers the
// cross-bank case: two distinct files in the same directory that
// both declare the same id.
func TestLoadDir_P9_RejectsDuplicateIDAcrossBanks(t *testing.T) {
	dir := t.TempDir()
	writeBank(t, filepath.Join(dir, "a.yaml"), "NX-GEN-x", "first")
	writeBank(t, filepath.Join(dir, "b.yaml"), "NX-GEN-x", "second")
	_, err := LoadDir(dir)
	if err == nil {
		t.Fatal("expected cross-bank duplicate error")
	}
	if !strings.Contains(err.Error(), "duplicate test case id") {
		t.Errorf("error must mention duplicate id: %v", err)
	}
}

// TestLoadDir_P9_JSONTwinIgnoredWhenYAMLSiblingPresent guards the
// common case where the same bank is serialised twice (YAML + JSON)
// side by side — that is not a duplicate, it is a twin, and the
// loader must skip the JSON form.
func TestLoadDir_P9_JSONTwinIgnoredWhenYAMLSiblingPresent(t *testing.T) {
	dir := t.TempDir()
	writeBank(t, filepath.Join(dir, "a.yaml"), "NX-GEN-x", "first")
	// Write a sibling JSON carrying the same id — the YAML presence
	// should make the loader skip it entirely.
	if err := os.WriteFile(
		filepath.Join(dir, "a.json"),
		[]byte(`{"name":"twin","test_cases":[{"id":"NX-GEN-x","name":"first","platforms":["api"],"steps":[{"name":"s","action":"a","expected":"ok"}]}]}`),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	banks, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("loader must skip JSON twin: %v", err)
	}
	if len(banks) != 1 {
		t.Errorf("loaded %d banks, want 1 (twin must be skipped)", len(banks))
	}
}

func writeBank(t *testing.T, path, id, name string) {
	t.Helper()
	content := []byte("name: test\ntest_cases:\n  - id: " + id + `
    name: ` + name + `
    platforms: [api]
    steps:
      - name: step
        action: "a"
        expected: "ok"
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}
