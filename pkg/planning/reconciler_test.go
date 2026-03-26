// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package planning

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sampleBankYAML is a minimal bank YAML that LoadBankDir
// should be able to parse.
const sampleBankYAML = `version: "1.0"
name: "Core Tests"
test_cases:
  - id: TC-001
    name: "Create new document"
    category: functional
    priority: 1
    platforms: [android, web]
  - id: TC-002
    name: "Save markdown file"
    category: functional
    priority: 2
    platforms: [android]
`

func TestBankReconciler_LoadExisting(t *testing.T) {
	dir := t.TempDir()
	bankPath := filepath.Join(dir, "core.yaml")
	err := os.WriteFile(bankPath, []byte(sampleBankYAML), 0644)
	require.NoError(t, err)

	rec := NewBankReconciler()
	err = rec.LoadBankDir(dir)
	require.NoError(t, err)

	assert.Equal(t, 2, rec.ExistingCount(),
		"should have loaded 2 test cases from the bank file")
}

func TestBankReconciler_Reconcile(t *testing.T) {
	rec := NewBankReconciler()
	// Manually register one existing entry.
	rec.AddExisting("TC-001", "Create new document", "core.yaml")

	generated := []PlannedTest{
		{ID: "GEN-001", Name: "Create new document"},
		{ID: "GEN-002", Name: "Delete a document"},
	}

	reconciled := rec.Reconcile(generated)

	require.Len(t, reconciled, 2)

	// First test matches by name — should be marked existing.
	assert.True(t, reconciled[0].IsExisting,
		"matched test should be IsExisting")
	assert.False(t, reconciled[0].IsNew,
		"matched test should not be IsNew")
	assert.Equal(t, "TC-001", reconciled[0].ID,
		"matched test ID should come from bank")
	assert.Equal(t, "core.yaml", reconciled[0].BankSource,
		"BankSource should be set from bank entry")

	// Second test has no bank match — should be marked new.
	assert.True(t, reconciled[1].IsNew,
		"unmatched test should be IsNew")
	assert.False(t, reconciled[1].IsExisting,
		"unmatched test should not be IsExisting")
}

func TestBankReconciler_NewTests(t *testing.T) {
	// No existing entries loaded — everything should be new.
	rec := NewBankReconciler()

	generated := []PlannedTest{
		{ID: "GEN-001", Name: "Feature A test"},
		{ID: "GEN-002", Name: "Feature B test"},
		{ID: "GEN-003", Name: "Feature C test"},
	}

	reconciled := rec.Reconcile(generated)

	require.Len(t, reconciled, 3)
	for _, pt := range reconciled {
		assert.True(t, pt.IsNew,
			"all tests should be new when bank is empty")
		assert.False(t, pt.IsExisting,
			"no test should be existing when bank is empty")
	}
}
