// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package planning

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// existingEntry records a known test case from a bank file.
type existingEntry struct {
	id     string
	name   string
	source string
}

// bankFileYAML is the minimal YAML structure read by
// LoadBankDir. It mirrors the subset of testbank.BankFile
// that BankReconciler needs, avoiding a cross-package import.
type bankFileYAML struct {
	Version   string           `yaml:"version"`
	Name      string           `yaml:"name"`
	TestCases []bankCaseYAML   `yaml:"test_cases"`
}

// bankCaseYAML is the per-test-case subset read from YAML.
type bankCaseYAML struct {
	ID        string   `yaml:"id"`
	Name      string   `yaml:"name"`
	Category  string   `yaml:"category"`
	Priority  int      `yaml:"priority"`
	Platforms []string `yaml:"platforms"`
}

// BankReconciler matches generated PlannedTests against
// known entries from test bank files. Matching is done by
// lowercase test name so minor casing differences don't
// prevent a match.
type BankReconciler struct {
	// existing maps lowercase name → existingEntry.
	existing map[string]existingEntry
}

// NewBankReconciler creates an empty BankReconciler.
func NewBankReconciler() *BankReconciler {
	return &BankReconciler{
		existing: make(map[string]existingEntry),
	}
}

// LoadBankDir walks dir, parses every .yaml/.yml file it
// finds, and registers all test cases contained therein.
// The file name (base name only) is used as BankSource.
func (r *BankReconciler) LoadBankDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read bank dir %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read bank file %s: %w", path, err)
		}

		var bf bankFileYAML
		if err := yaml.Unmarshal(data, &bf); err != nil {
			return fmt.Errorf(
				"parse bank file %s: %w", path, err,
			)
		}

		source := entry.Name()
		for _, tc := range bf.TestCases {
			r.AddExisting(tc.ID, tc.Name, source)
		}
	}

	return nil
}

// AddExisting registers a single test case as a known bank
// entry. Keyed by lowercase name for case-insensitive lookup.
func (r *BankReconciler) AddExisting(id, name, source string) {
	key := strings.ToLower(name)
	r.existing[key] = existingEntry{
		id:     id,
		name:   name,
		source: source,
	}
}

// ExistingCount returns the number of registered bank entries.
func (r *BankReconciler) ExistingCount() int {
	return len(r.existing)
}

// Reconcile returns a copy of tests with IsExisting/IsNew and
// BankSource fields populated. Matching is by lowercase name.
// Matched tests also have their ID replaced with the bank ID.
// The original slice is never modified.
func (r *BankReconciler) Reconcile(tests []PlannedTest) []PlannedTest {
	result := make([]PlannedTest, len(tests))
	copy(result, tests)

	for i := range result {
		key := strings.ToLower(result[i].Name)
		if entry, ok := r.existing[key]; ok {
			result[i].IsExisting = true
			result[i].IsNew = false
			result[i].ID = entry.id
			result[i].BankSource = entry.source
		} else {
			result[i].IsNew = true
			result[i].IsExisting = false
		}
	}

	return result
}
