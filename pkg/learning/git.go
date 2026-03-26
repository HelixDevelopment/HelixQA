// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package learning

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// GitAnalyzer queries the git history of a project to surface recent commits
// and frequently-changed files.
type GitAnalyzer struct {
	root string
}

// NewGitAnalyzer returns a GitAnalyzer anchored at root.
func NewGitAnalyzer(root string) *GitAnalyzer {
	return &GitAnalyzer{root: root}
}

// RecentCommits returns up to limit ChangeEntry values representing the most
// recent commits in the repository.  Each entry includes the commit hash,
// subject line, ISO-8601 author date, and the list of files touched.
//
// The underlying git command is:
//
//	git log --max-count=N --pretty=format:%H|%s|%aI --name-only
//
// Commit blocks are separated by a blank line in the output.
func (g *GitAnalyzer) RecentCommits(limit int) ([]ChangeEntry, error) {
	if limit <= 0 {
		limit = 10
	}
	out, err := g.git(
		"log",
		fmt.Sprintf("--max-count=%d", limit),
		"--pretty=format:%H|%s|%aI",
		"--name-only",
	)
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}

	return parseCommitBlocks(out), nil
}

// HotFiles returns up to limit file paths sorted by the number of commits
// that touched them (most-changed first).  It inspects the last 100 commits
// across all branches.
func (g *GitAnalyzer) HotFiles(limit int) []string {
	if limit <= 0 {
		limit = 10
	}
	out, err := g.git(
		"log",
		"--all",
		"--name-only",
		"--pretty=format:",
		"--max-count=100",
	)
	if err != nil {
		return nil
	}

	freq := map[string]int{}
	for _, line := range strings.Split(out, "\n") {
		f := strings.TrimSpace(line)
		if f != "" {
			freq[f]++
		}
	}

	type kv struct {
		file  string
		count int
	}
	var pairs []kv
	for f, c := range freq {
		pairs = append(pairs, kv{f, c})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].count != pairs[j].count {
			return pairs[i].count > pairs[j].count
		}
		return pairs[i].file < pairs[j].file
	})

	result := make([]string, 0, limit)
	for i := 0; i < len(pairs) && i < limit; i++ {
		result = append(result, pairs[i].file)
	}
	return result
}

// git runs a git command in the analyzer's root directory and returns the
// combined stdout output.  stderr is captured and included in any error.
func (g *GitAnalyzer) git(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.root
	out, err := cmd.Output()
	if err != nil {
		// Include stderr when available via *exec.ExitError.
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(ee.Stderr)))
		}
		return "", err
	}
	return string(out), nil
}

// parseCommitBlocks splits the raw `git log` output into ChangeEntry values.
//
// Each block looks like:
//
//	<hash>|<subject>|<date>
//	file1.go
//	file2.go
//
// Blocks are separated by one or more blank lines.
func parseCommitBlocks(raw string) []ChangeEntry {
	var entries []ChangeEntry

	// Normalise line endings, then split on double-newline boundaries.
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	blocks := strings.Split(raw, "\n\n")

	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		lines := strings.Split(block, "\n")
		if len(lines) == 0 {
			continue
		}

		// First line: hash|subject|date
		headerParts := strings.SplitN(lines[0], "|", 3)
		if len(headerParts) < 3 {
			continue
		}
		entry := ChangeEntry{
			Hash:    strings.TrimSpace(headerParts[0]),
			Message: strings.TrimSpace(headerParts[1]),
			Date:    strings.TrimSpace(headerParts[2]),
		}

		// Remaining non-empty lines are file paths.
		for _, line := range lines[1:] {
			f := strings.TrimSpace(line)
			if f != "" {
				entry.Files = append(entry.Files, f)
			}
		}

		entries = append(entries, entry)
	}

	return entries
}
