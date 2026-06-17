// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package challengegen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"digital.vasic.challenges/pkg/bank"
	"digital.vasic.challenges/pkg/challenge"
)

// ValidateGenerated proves a set of generated Challenges is VALID by
// round-tripping them through the REAL Challenges-module bank
// validator (bank.ValidateFile) — the same gate that guards every
// hand-written test bank. It writes the definitions into a canonical
// BankFile JSON in a temp dir and runs the on-disk validator.
//
// This deliberately reuses the existing validator instead of
// re-checking ID/Name locally, so a generated challenge that would be
// rejected by the real loader is rejected here too. Returns a non-nil
// error listing every validation problem found, or nil when the whole
// set is loadable.
func ValidateGenerated(defs []challenge.Definition) error {
	file := bank.BankFile{
		Version:    "1.0.0",
		Name:       "helixqa-generated-challenges",
		Challenges: defs,
	}

	data, err := json.Marshal(file)
	if err != nil {
		return fmt.Errorf("marshal generated bank: %w", err)
	}

	dir, err := os.MkdirTemp("", "challengegen-validate-*")
	if err != nil {
		return fmt.Errorf("temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "generated_bank.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write generated bank: %w", err)
	}

	verrs := bank.ValidateFile(path)
	if len(verrs) == 0 {
		return nil
	}

	return fmt.Errorf(
		"%d validation error(s) in generated challenges: %v",
		len(verrs), verrs,
	)
}
