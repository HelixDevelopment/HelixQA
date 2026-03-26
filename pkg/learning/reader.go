// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package learning

import (
	"os"
	"path/filepath"
	"strings"
)

// skipDirs contains directory names that should never be walked when
// searching for CLAUDE.md / AGENTS.md files.
var skipDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	"vendor":       true,
}

// ProjectReader reads documentation and constraint information from a
// project directory tree.
type ProjectReader struct {
	root string
}

// NewProjectReader returns a ProjectReader anchored at root.
func NewProjectReader(root string) *ProjectReader {
	return &ProjectReader{root: root}
}

// ReadDocs walks the docs/ subdirectory under the project root, reads every
// .md and .markdown file it finds, extracts a title from the first `# `
// heading, and truncates the content to 2000 characters.
func (r *ProjectReader) ReadDocs() ([]DocEntry, error) {
	docsDir := filepath.Join(r.root, "docs")

	info, err := os.Stat(docsDir)
	if os.IsNotExist(err) || (err == nil && !info.IsDir()) {
		return []DocEntry{}, nil
	}
	if err != nil {
		return nil, err
	}

	var entries []DocEntry

	err = filepath.WalkDir(docsDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		name := strings.ToLower(d.Name())
		if !strings.HasSuffix(name, ".md") && !strings.HasSuffix(name, ".markdown") {
			return nil
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		content := truncateContent(string(raw), 2000)
		entries = append(entries, DocEntry{
			Path:    path,
			Title:   extractTitle(string(raw)),
			Content: content,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	return entries, nil
}

// ReadClaudeMDs walks the entire project tree searching for CLAUDE.md and
// AGENTS.md files.  It skips node_modules, .git, and vendor directories.
// Content is truncated to 4000 characters per file.
func (r *ProjectReader) ReadClaudeMDs() ([]DocEntry, error) {
	var entries []DocEntry

	err := filepath.WalkDir(r.root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if name != "CLAUDE.md" && name != "AGENTS.md" {
			return nil
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		content := truncateContent(string(raw), 4000)
		entries = append(entries, DocEntry{
			Path:    path,
			Title:   extractTitle(string(raw)),
			Content: content,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	return entries, nil
}

// ExtractConstraints scans the provided DocEntry slice and extracts bullet
// items (lines beginning with `- `) that appear after a heading whose text
// contains "constraint" or "critical" (case-insensitive).
func (r *ProjectReader) ExtractConstraints(docs []DocEntry) []string {
	var constraints []string
	seen := map[string]bool{}

	for _, doc := range docs {
		inSection := false
		for _, line := range strings.Split(doc.Content, "\n") {
			trimmed := strings.TrimSpace(line)

			// Detect headings (## / ### / # …)
			if strings.HasPrefix(trimmed, "#") {
				heading := strings.ToLower(strings.TrimLeft(trimmed, "# "))
				inSection = strings.Contains(heading, "constraint") ||
					strings.Contains(heading, "critical")
				continue
			}

			if inSection && strings.HasPrefix(trimmed, "- ") {
				item := strings.TrimPrefix(trimmed, "- ")
				item = strings.TrimSpace(item)
				if item != "" && !seen[item] {
					seen[item] = true
					constraints = append(constraints, item)
				}
			}
		}
	}

	return constraints
}

// extractTitle returns the text of the first `# ` heading found in content.
// If no heading is found it returns an empty string.
func extractTitle(content string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		}
	}
	return ""
}

// truncateContent returns s truncated to at most maxLen characters.
func truncateContent(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
