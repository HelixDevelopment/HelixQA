// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package testbank

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadFile loads a YAML test bank file and returns the parsed
// BankFile. Supports both .yaml and .yml extensions.
func LoadFile(path string) (*BankFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read bank file %s: %w", path, err)
	}

	var bf BankFile
	if err := yaml.Unmarshal(data, &bf); err != nil {
		return nil, fmt.Errorf("parse bank file %s: %w", path, err)
	}

	// Validate all test cases.
	for i := range bf.TestCases {
		if msg := bf.TestCases[i].IsValid(); msg != "" {
			return nil, fmt.Errorf(
				"bank file %s: test case %d: %s",
				path, i, msg,
			)
		}
	}

	return &bf, nil
}

// LoadDir loads all YAML test bank files from a directory.
// It scans for .yaml and .yml files (non-recursive).
func LoadDir(dir string) ([]*BankFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read bank dir %s: %w", dir, err)
	}

	var banks []*BankFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		bf, err := LoadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
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
