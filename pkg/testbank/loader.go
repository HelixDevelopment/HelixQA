// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package testbank

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// jsonBankFile mirrors BankFile but accepts "challenges" as an
// alternate key for test_cases (used by comprehensive JSON banks).
type jsonBankFile struct {
	Version    string            `json:"version"`
	Name       string            `json:"name"`
	Description string           `json:"description"`
	TestCases  []TestCase        `json:"test_cases"`
	Challenges []TestCase        `json:"challenges"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// LoadFile loads a test bank file (YAML or JSON) and returns the
// parsed BankFile.
func LoadFile(path string) (*BankFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read bank file %s: %w", path, err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	var bf BankFile

	if ext == ".json" {
		var jbf jsonBankFile
		if err := json.Unmarshal(data, &jbf); err != nil {
			return nil, fmt.Errorf("parse bank file %s: %w", path, err)
		}
		bf.Version = jbf.Version
		bf.Name = jbf.Name
		bf.Description = jbf.Description
		bf.Metadata = jbf.Metadata
		bf.TestCases = jbf.TestCases
		if len(bf.TestCases) == 0 && len(jbf.Challenges) > 0 {
			bf.TestCases = jbf.Challenges
		}
	} else {
		if err := yaml.Unmarshal(data, &bf); err != nil {
			return nil, fmt.Errorf("parse bank file %s: %w", path, err)
		}
	}

	// Validate all test cases + guard against intra-bank duplicate
	// ids. P9 fix (docs/nexus/remaining-work.md): a duplicate id
	// inside a single bank used to silently overwrite the prior
	// entry when loaded into maps downstream; refuse at load time.
	seen := map[string]int{}
	for i := range bf.TestCases {
		if msg := bf.TestCases[i].IsValid(); msg != "" {
			return nil, fmt.Errorf(
				"bank file %s: test case %d: %s",
				path, i, msg,
			)
		}
		id := bf.TestCases[i].ID
		if id == "" {
			continue
		}
		if prev, dup := seen[id]; dup {
			return nil, fmt.Errorf(
				"bank file %s: duplicate test case id %q at indices %d and %d",
				path, id, prev, i,
			)
		}
		seen[id] = i
	}

	return &bf, nil
}

// LoadDir loads all test bank files from a directory.
// It scans for .yaml, .yml, and .json files (non-recursive).
func LoadDir(dir string) ([]*BankFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read bank dir %s: %w", dir, err)
	}

	var banks []*BankFile
	// P9: track every id across the directory so cross-bank
	// collisions are caught at load time. The bank-registry layer
	// downstream also derives from these ids and silently drops
	// collisions; blocking here is the permanent fix.
	xref := map[string]string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yaml" && ext != ".yml" && ext != ".json" {
			continue
		}
		// Skip JSON twins when a YAML sibling exists to avoid a
		// spurious duplicate-id error — the YAML and JSON forms are
		// two serialisations of the same bank.
		if ext == ".json" {
			base := strings.TrimSuffix(entry.Name(), ".json")
			if _, err := os.Stat(filepath.Join(dir, base+".yaml")); err == nil {
				continue
			}
			if _, err := os.Stat(filepath.Join(dir, base+".yml")); err == nil {
				continue
			}
		}
		path := filepath.Join(dir, entry.Name())
		bf, err := LoadFile(path)
		if err != nil {
			return nil, err
		}
		for _, tc := range bf.TestCases {
			if tc.ID == "" {
				continue
			}
			if prev, dup := xref[tc.ID]; dup {
				return nil, fmt.Errorf(
					"bank dir %s: duplicate test case id %q across banks (also in %s)",
					dir, tc.ID, prev,
				)
			}
			xref[tc.ID] = path
		}
		banks = append(banks, bf)
	}
	return banks, nil
}

// SaveFile writes a BankFile to a YAML file.
func SaveFile(path string, bf *BankFile) error {
	data, err := yaml.Marshal(bf)
	if err != nil {
		return fmt.Errorf("marshal bank file: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create bank dir: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}
