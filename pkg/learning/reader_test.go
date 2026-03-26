// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package learning_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/learning"
)

// setupTestProject creates a temporary directory tree with:
//   - CLAUDE.md at the root
//   - docs/architecture.md
//   - docs/api.md
//   - submodule/CLAUDE.md  (nested, should also be found)
//   - node_modules/CLAUDE.md (should be skipped)
//   - .git/CLAUDE.md (should be skipped)
func setupTestProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()

	// Root CLAUDE.md with a CRITICAL section
	claudeContent := `# Project Root

## CRITICAL: Do Not Break Production

- Never push untested code
- Always run tests before merging
- Maintain zero-warning policy

## Constraints

- Use Podman, not Docker
- All builds must be containerized
`
	write(t, filepath.Join(root, "CLAUDE.md"), claudeContent)

	// docs/architecture.md
	docsDir := filepath.Join(root, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o755))
	write(t, filepath.Join(docsDir, "architecture.md"),
		"# Architecture Overview\n\nThis document describes the system architecture.\n")
	write(t, filepath.Join(docsDir, "api.md"),
		"# API Reference\n\nEndpoints and usage.\n")

	// submodule/CLAUDE.md — nested, not in skip dirs
	subDir := filepath.Join(root, "submodule")
	require.NoError(t, os.MkdirAll(subDir, 0o755))
	write(t, filepath.Join(subDir, "CLAUDE.md"),
		"# Submodule\n\nSubmodule CLAUDE.md content.\n")

	// node_modules — must be skipped
	nmDir := filepath.Join(root, "node_modules", "some-pkg")
	require.NoError(t, os.MkdirAll(nmDir, 0o755))
	write(t, filepath.Join(nmDir, "CLAUDE.md"),
		"# Should Be Ignored\n")

	// .git — must be skipped
	gitDir := filepath.Join(root, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0o755))
	write(t, filepath.Join(gitDir, "CLAUDE.md"),
		"# Should Be Ignored\n")

	return root
}

func write(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

// TestProjectReader_ReadDocs verifies that at least 2 .md docs are found
// under the docs/ directory and that titles and content are populated.
func TestProjectReader_ReadDocs(t *testing.T) {
	root := setupTestProject(t)
	r := learning.NewProjectReader(root)

	docs, err := r.ReadDocs()
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(docs), 2, "should find at least 2 docs")

	for _, d := range docs {
		assert.NotEmpty(t, d.Path, "doc path should not be empty")
		assert.NotEmpty(t, d.Title, "doc title should be extracted")
		assert.NotEmpty(t, d.Content, "doc content should not be empty")
	}
}

// TestProjectReader_ReadDocs_TitleExtraction verifies that the title is
// taken from the first `# ` heading in the file.
func TestProjectReader_ReadDocs_TitleExtraction(t *testing.T) {
	root := setupTestProject(t)
	r := learning.NewProjectReader(root)

	docs, err := r.ReadDocs()
	require.NoError(t, err)

	titles := make(map[string]bool)
	for _, d := range docs {
		titles[d.Title] = true
	}

	assert.True(t, titles["Architecture Overview"] || titles["API Reference"],
		"expected to find known titles; got: %v", titles)
}

// TestProjectReader_ReadDocs_ContentTruncation verifies that content is
// truncated to at most 2000 characters.
func TestProjectReader_ReadDocs_ContentTruncation(t *testing.T) {
	root := t.TempDir()
	docsDir := filepath.Join(root, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o755))

	// Write a file with content longer than 2000 chars
	longContent := "# Long Doc\n\n" + string(make([]byte, 3000))
	write(t, filepath.Join(docsDir, "long.md"), longContent)

	r := learning.NewProjectReader(root)
	docs, err := r.ReadDocs()
	require.NoError(t, err)
	require.Len(t, docs, 1)

	assert.LessOrEqual(t, len(docs[0].Content), 2000,
		"content should be truncated to 2000 chars")
}

// TestProjectReader_ReadClaudeMDs verifies that CLAUDE.md files are found
// recursively, skipping node_modules, .git, and vendor directories.
func TestProjectReader_ReadClaudeMDs(t *testing.T) {
	root := setupTestProject(t)
	r := learning.NewProjectReader(root)

	entries, err := r.ReadClaudeMDs()
	require.NoError(t, err)

	// Should find root CLAUDE.md + submodule/CLAUDE.md = at least 2
	assert.GreaterOrEqual(t, len(entries), 2,
		"should find at least 2 CLAUDE.md files")

	// None should come from node_modules or .git
	for _, e := range entries {
		assert.NotContains(t, e.Path, "node_modules",
			"node_modules CLAUDE.md should be skipped")
		assert.NotContains(t, e.Path, filepath.Join(root, ".git"),
			".git CLAUDE.md should be skipped")
	}
}

// TestProjectReader_ReadClaudeMDs_ContentTruncation verifies that content
// is truncated to at most 4000 characters.
func TestProjectReader_ReadClaudeMDs_ContentTruncation(t *testing.T) {
	root := t.TempDir()

	longContent := "# Big CLAUDE\n\n" + string(make([]byte, 5000))
	write(t, filepath.Join(root, "CLAUDE.md"), longContent)

	r := learning.NewProjectReader(root)
	entries, err := r.ReadClaudeMDs()
	require.NoError(t, err)
	require.Len(t, entries, 1)

	assert.LessOrEqual(t, len(entries[0].Content), 4000,
		"content should be truncated to 4000 chars")
}

// TestProjectReader_ExtractConstraints verifies that bullet items under
// CRITICAL or Constraints sections are extracted.
func TestProjectReader_ExtractConstraints(t *testing.T) {
	root := setupTestProject(t)
	r := learning.NewProjectReader(root)

	entries, err := r.ReadClaudeMDs()
	require.NoError(t, err)

	constraints := r.ExtractConstraints(entries)
	assert.GreaterOrEqual(t, len(constraints), 1,
		"should extract at least 1 constraint bullet")
}

// TestProjectReader_EmptyDir verifies that ReadDocs returns 0 docs when
// there is no docs/ subdirectory.
func TestProjectReader_EmptyDir(t *testing.T) {
	root := t.TempDir()
	r := learning.NewProjectReader(root)

	docs, err := r.ReadDocs()
	require.NoError(t, err)
	assert.Len(t, docs, 0, "empty project should return 0 docs")
}
