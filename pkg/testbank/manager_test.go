// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package testbank

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/config"
)

// --- BankFile YAML ---

const sampleBank = `version: "1.0"
name: "Yole Core Tests"
description: "Core functionality tests for Yole editor"
test_cases:
  - id: TC-001
    name: "Create new document"
    description: "Verify creating a new document works on all platforms"
    category: functional
    priority: critical
    platforms: [android, web, desktop]
    steps:
      - name: "Open app"
        action: "Launch application"
        expected: "Main editor screen visible"
      - name: "Create document"
        action: "Tap new document button"
        expected: "Empty document opens"
    tags: [core, smoke]
    estimated_duration: "30s"
    expected_result: "New document created successfully"
    documentation_refs:
      - type: user_guide
        section: "3.1"
        path: "docs/USER_MANUAL_FORMATS.md"

  - id: TC-002
    name: "Save markdown file"
    description: "Verify markdown files save correctly"
    category: functional
    priority: high
    platforms: [android, desktop]
    steps:
      - name: "Create markdown"
        action: "Type markdown content"
        expected: "Content appears in editor"
      - name: "Save file"
        action: "Save the document"
        expected: "File saved to disk"
    tags: [markdown, save]
    estimated_duration: "15s"

  - id: TC-003
    name: "Web-only export"
    description: "Verify web export functionality"
    category: integration
    priority: medium
    platforms: [web]
    steps:
      - name: "Export to HTML"
        action: "Click export button"
        expected: "HTML file downloaded"
    tags: [web, export]
    estimated_duration: "10s"

  - id: TC-004
    name: "Edge case empty file"
    description: "Open an empty file without crash"
    category: edge_case
    priority: low
    tags: [edge, stability]
    estimated_duration: "5s"
metadata:
  author: "test-suite"
  version: "1.0.0"
`

func writeSampleBank(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(sampleBank), 0644))
	return path
}

// --- Manager Tests ---

func TestManager_LoadFile(t *testing.T) {
	dir := t.TempDir()
	path := writeSampleBank(t, dir, "core.yaml")

	mgr := NewManager()
	require.NoError(t, mgr.LoadFile(path))

	assert.Equal(t, 4, mgr.Count())
	assert.Equal(t, []string{path}, mgr.Sources())
}

func TestManager_LoadFile_DuplicateID(t *testing.T) {
	dir := t.TempDir()
	path := writeSampleBank(t, dir, "core.yaml")

	mgr := NewManager()
	require.NoError(t, mgr.LoadFile(path))

	// Loading same file again should fail on duplicate IDs.
	err := mgr.LoadFile(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate test case ID")
}

func TestManager_LoadFile_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	require.NoError(t, os.WriteFile(path, []byte("{{invalid"), 0644))

	mgr := NewManager()
	err := mgr.LoadFile(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse bank file")
}

func TestManager_LoadFile_MissingID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "noid.yaml")
	content := `version: "1.0"
name: "Bad Bank"
test_cases:
  - name: "No ID test"
    description: "Missing required ID"
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	mgr := NewManager()
	err := mgr.LoadFile(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing ID")
}

func TestManager_LoadFile_MissingName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "noname.yaml")
	content := `version: "1.0"
name: "Bad Bank"
test_cases:
  - id: TC-X
    description: "Missing name"
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	mgr := NewManager()
	err := mgr.LoadFile(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing name")
}

func TestManager_LoadDir(t *testing.T) {
	dir := t.TempDir()
	writeSampleBank(t, dir, "bank1.yaml")

	// Second bank with different IDs.
	bank2 := `version: "1.0"
name: "Extra"
test_cases:
  - id: TC-100
    name: "Extra test"
    category: extra
    priority: low
`
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "bank2.yml"),
		[]byte(bank2), 0644,
	))

	// Non-YAML file should be ignored.
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "readme.txt"),
		[]byte("not a bank"), 0644,
	))

	mgr := NewManager()
	require.NoError(t, mgr.LoadDir(dir))

	assert.Equal(t, 5, mgr.Count()) // 4 + 1
}

func TestManager_LoadDir_Empty(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager()
	require.NoError(t, mgr.LoadDir(dir))
	assert.Equal(t, 0, mgr.Count())
}

func TestManager_LoadDir_NonExistent(t *testing.T) {
	mgr := NewManager()
	err := mgr.LoadDir("/nonexistent/path")
	assert.Error(t, err)
}

func TestManager_Get(t *testing.T) {
	dir := t.TempDir()
	writeSampleBank(t, dir, "core.yaml")

	mgr := NewManager()
	require.NoError(t, mgr.LoadFile(
		filepath.Join(dir, "core.yaml"),
	))

	tc, ok := mgr.Get("TC-001")
	assert.True(t, ok)
	assert.Equal(t, "Create new document", tc.Name)
	assert.Equal(t, PriorityCritical, tc.Priority)

	_, ok = mgr.Get("NONEXISTENT")
	assert.False(t, ok)
}

func TestManager_All_SortedByID(t *testing.T) {
	dir := t.TempDir()
	writeSampleBank(t, dir, "core.yaml")

	mgr := NewManager()
	require.NoError(t, mgr.LoadFile(
		filepath.Join(dir, "core.yaml"),
	))

	all := mgr.All()
	assert.Len(t, all, 4)
	// Should be sorted by ID.
	for i := 1; i < len(all); i++ {
		assert.True(t, all[i-1].ID < all[i].ID,
			"expected sorted: %s < %s",
			all[i-1].ID, all[i].ID,
		)
	}
}

func TestManager_ForPlatform_Android(t *testing.T) {
	dir := t.TempDir()
	writeSampleBank(t, dir, "core.yaml")

	mgr := NewManager()
	require.NoError(t, mgr.LoadFile(
		filepath.Join(dir, "core.yaml"),
	))

	android := mgr.ForPlatform(config.PlatformAndroid)
	// TC-001 (all), TC-002 (android+desktop), TC-004 (no platforms = all)
	assert.Len(t, android, 3)
	// Should be sorted by priority: critical first.
	assert.Equal(t, PriorityCritical, android[0].Priority)
}

func TestManager_ForPlatform_Web(t *testing.T) {
	dir := t.TempDir()
	writeSampleBank(t, dir, "core.yaml")

	mgr := NewManager()
	require.NoError(t, mgr.LoadFile(
		filepath.Join(dir, "core.yaml"),
	))

	web := mgr.ForPlatform(config.PlatformWeb)
	// TC-001 (all), TC-003 (web-only), TC-004 (no platforms = all)
	assert.Len(t, web, 3)
}

func TestManager_ForPlatform_Desktop(t *testing.T) {
	dir := t.TempDir()
	writeSampleBank(t, dir, "core.yaml")

	mgr := NewManager()
	require.NoError(t, mgr.LoadFile(
		filepath.Join(dir, "core.yaml"),
	))

	desktop := mgr.ForPlatform(config.PlatformDesktop)
	// TC-001 (all), TC-002 (android+desktop), TC-004 (no platforms = all)
	assert.Len(t, desktop, 3)
}

func TestManager_ByCategory(t *testing.T) {
	dir := t.TempDir()
	writeSampleBank(t, dir, "core.yaml")

	mgr := NewManager()
	require.NoError(t, mgr.LoadFile(
		filepath.Join(dir, "core.yaml"),
	))

	functional := mgr.ByCategory("functional")
	assert.Len(t, functional, 2) // TC-001, TC-002

	integration := mgr.ByCategory("integration")
	assert.Len(t, integration, 1) // TC-003

	edgeCases := mgr.ByCategory("edge_case")
	assert.Len(t, edgeCases, 1) // TC-004

	empty := mgr.ByCategory("nonexistent")
	assert.Empty(t, empty)
}

func TestManager_ByPriority(t *testing.T) {
	dir := t.TempDir()
	writeSampleBank(t, dir, "core.yaml")

	mgr := NewManager()
	require.NoError(t, mgr.LoadFile(
		filepath.Join(dir, "core.yaml"),
	))

	critical := mgr.ByPriority(PriorityCritical)
	assert.Len(t, critical, 1) // TC-001

	high := mgr.ByPriority(PriorityHigh)
	assert.Len(t, high, 1) // TC-002

	low := mgr.ByPriority(PriorityLow)
	assert.Len(t, low, 1) // TC-004
}

func TestManager_ByTag(t *testing.T) {
	dir := t.TempDir()
	writeSampleBank(t, dir, "core.yaml")

	mgr := NewManager()
	require.NoError(t, mgr.LoadFile(
		filepath.Join(dir, "core.yaml"),
	))

	core := mgr.ByTag("core")
	assert.Len(t, core, 1) // TC-001

	smoke := mgr.ByTag("smoke")
	assert.Len(t, smoke, 1) // TC-001

	markdown := mgr.ByTag("markdown")
	assert.Len(t, markdown, 1) // TC-002

	edge := mgr.ByTag("edge")
	assert.Len(t, edge, 1) // TC-004

	empty := mgr.ByTag("nonexistent")
	assert.Empty(t, empty)
}

func TestManager_Banks(t *testing.T) {
	dir := t.TempDir()
	writeSampleBank(t, dir, "core.yaml")

	mgr := NewManager()
	require.NoError(t, mgr.LoadFile(
		filepath.Join(dir, "core.yaml"),
	))

	banks := mgr.Banks()
	require.Len(t, banks, 1)
	assert.Equal(t, "Yole Core Tests", banks[0].Name)
	assert.Equal(t, "1.0", banks[0].Version)
	assert.Len(t, banks[0].TestCases, 4)
	assert.Equal(t, "test-suite", banks[0].Metadata["author"])
}

func TestManager_ToDefinitions(t *testing.T) {
	dir := t.TempDir()
	writeSampleBank(t, dir, "core.yaml")

	mgr := NewManager()
	require.NoError(t, mgr.LoadFile(
		filepath.Join(dir, "core.yaml"),
	))

	defs := mgr.ToDefinitions(config.PlatformAndroid)
	assert.Len(t, defs, 3)

	// Each definition should have correct fields.
	for _, def := range defs {
		assert.NotEmpty(t, def.ID)
		assert.NotEmpty(t, def.Name)
	}
}

// --- Schema Tests ---

func TestTestCase_ToDefinition(t *testing.T) {
	tc := &TestCase{
		ID:                "TC-TEST",
		Name:              "Test conversion",
		Description:       "Verifies definition conversion",
		Category:          "unit",
		Dependencies:      []string{"TC-001", "TC-002"},
		EstimatedDuration: "30s",
	}

	def := tc.ToDefinition()
	assert.Equal(t, "TC-TEST", string(def.ID))
	assert.Equal(t, "Test conversion", def.Name)
	assert.Equal(t, "unit", def.Category)
	assert.Len(t, def.Dependencies, 2)
	assert.Equal(t, "30s", def.EstimatedDuration)
}

func TestTestCase_AppliesToPlatform(t *testing.T) {
	tests := []struct {
		name      string
		platforms []config.Platform
		target    config.Platform
		expected  bool
	}{
		{
			name:      "empty platforms applies to all",
			platforms: nil,
			target:    config.PlatformAndroid,
			expected:  true,
		},
		{
			name:      "explicit android",
			platforms: []config.Platform{config.PlatformAndroid},
			target:    config.PlatformAndroid,
			expected:  true,
		},
		{
			name:      "android not in web-only",
			platforms: []config.Platform{config.PlatformWeb},
			target:    config.PlatformAndroid,
			expected:  false,
		},
		{
			name: "multi-platform match",
			platforms: []config.Platform{
				config.PlatformAndroid,
				config.PlatformDesktop,
			},
			target:   config.PlatformDesktop,
			expected: true,
		},
		{
			name: "multi-platform no match",
			platforms: []config.Platform{
				config.PlatformAndroid,
				config.PlatformDesktop,
			},
			target:   config.PlatformWeb,
			expected: false,
		},
		{
			name:      "platform all matches everything",
			platforms: []config.Platform{config.PlatformAll},
			target:    config.PlatformWeb,
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := &TestCase{Platforms: tt.platforms}
			assert.Equal(t, tt.expected,
				tc.AppliesToPlatform(tt.target),
			)
		})
	}
}

func TestTestCase_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		tc       TestCase
		hasError bool
	}{
		{
			name:     "valid",
			tc:       TestCase{ID: "TC-1", Name: "Test"},
			hasError: false,
		},
		{
			name:     "missing ID",
			tc:       TestCase{Name: "Test"},
			hasError: true,
		},
		{
			name:     "missing name",
			tc:       TestCase{ID: "TC-1"},
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.tc.IsValid()
			if tt.hasError {
				assert.NotEmpty(t, msg)
			} else {
				assert.Empty(t, msg)
			}
		})
	}
}

// --- Loader Tests ---

func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	path := writeSampleBank(t, dir, "test.yaml")

	bf, err := LoadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "Yole Core Tests", bf.Name)
	assert.Len(t, bf.TestCases, 4)
}

func TestLoadFile_NonExistent(t *testing.T) {
	_, err := LoadFile("/nonexistent.yaml")
	assert.Error(t, err)
}

func TestLoadDir(t *testing.T) {
	dir := t.TempDir()
	writeSampleBank(t, dir, "a.yaml")

	bfs, err := LoadDir(dir)
	require.NoError(t, err)
	assert.Len(t, bfs, 1)
}

func TestLoadDir_YMLExtension(t *testing.T) {
	dir := t.TempDir()
	// Write with .yml extension.
	path := filepath.Join(dir, "test.yml")
	require.NoError(t, os.WriteFile(path, []byte(sampleBank), 0644))

	bfs, err := LoadDir(dir)
	require.NoError(t, err)
	assert.Len(t, bfs, 1)
}

func TestSaveFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "output.yaml")

	bf := &BankFile{
		Version: "1.0",
		Name:    "Saved Bank",
		TestCases: []TestCase{
			{
				ID:       "TC-SAVE",
				Name:     "Save test",
				Category: "io",
				Priority: PriorityHigh,
			},
		},
	}

	require.NoError(t, SaveFile(path, bf))

	// Verify by loading back.
	loaded, err := LoadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "Saved Bank", loaded.Name)
	assert.Len(t, loaded.TestCases, 1)
	assert.Equal(t, "TC-SAVE", loaded.TestCases[0].ID)
}

func TestSaveFile_NestedDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "bank.yaml")

	bf := &BankFile{
		Version: "1.0",
		Name:    "Nested",
		TestCases: []TestCase{
			{ID: "TC-N", Name: "Nested test"},
		},
	}

	require.NoError(t, SaveFile(path, bf))

	loaded, err := LoadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "Nested", loaded.Name)
}

// --- Documentation Refs ---

func TestTestCase_DocumentationRefs(t *testing.T) {
	dir := t.TempDir()
	writeSampleBank(t, dir, "core.yaml")

	mgr := NewManager()
	require.NoError(t, mgr.LoadFile(
		filepath.Join(dir, "core.yaml"),
	))

	tc, ok := mgr.Get("TC-001")
	require.True(t, ok)
	require.Len(t, tc.DocumentationRefs, 1)
	assert.Equal(t, "user_guide", tc.DocumentationRefs[0].Type)
	assert.Equal(t, "3.1", tc.DocumentationRefs[0].Section)
	assert.Equal(t,
		"docs/USER_MANUAL_FORMATS.md",
		tc.DocumentationRefs[0].Path,
	)
}

// --- Steps ---

func TestTestCase_Steps(t *testing.T) {
	dir := t.TempDir()
	writeSampleBank(t, dir, "core.yaml")

	mgr := NewManager()
	require.NoError(t, mgr.LoadFile(
		filepath.Join(dir, "core.yaml"),
	))

	tc, ok := mgr.Get("TC-001")
	require.True(t, ok)
	require.Len(t, tc.Steps, 2)
	assert.Equal(t, "Open app", tc.Steps[0].Name)
	assert.Equal(t, "Launch application", tc.Steps[0].Action)
	assert.Equal(t,
		"Main editor screen visible",
		tc.Steps[0].Expected,
	)
}

// --- Priority Sorting ---

func TestPriorityOrder(t *testing.T) {
	assert.Less(t, priorityOrder(PriorityCritical),
		priorityOrder(PriorityHigh))
	assert.Less(t, priorityOrder(PriorityHigh),
		priorityOrder(PriorityMedium))
	assert.Less(t, priorityOrder(PriorityMedium),
		priorityOrder(PriorityLow))
	assert.Less(t, priorityOrder(PriorityLow),
		priorityOrder(Priority("unknown")))
}
