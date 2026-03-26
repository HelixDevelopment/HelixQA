// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package learning_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/learning"
)

// setupGitRepo creates a temporary directory, initialises a git repo inside
// it, and makes 2 commits each touching a different file.  It returns the
// repo root path.
func setupGitRepo(t *testing.T) string {
	t.Helper()

	root := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git command failed: %s\n%s", args, out)
	}

	run("git", "init", "-b", "main")
	run("git", "config", "user.email", "test@test.com")
	run("git", "config", "user.name", "Test")

	// First commit
	write(t, filepath.Join(root, "alpha.go"), "package main\n")
	run("git", "add", "alpha.go")
	run("git", "commit", "-m", "feat: add alpha")

	// Second commit
	write(t, filepath.Join(root, "beta.go"), "package main\n")
	run("git", "add", "beta.go")
	run("git", "commit", "-m", "feat: add beta")

	return root
}

// TestGitAnalyzer_RecentCommits verifies that at least 2 commits are returned
// when the repo has 2 commits.
func TestGitAnalyzer_RecentCommits(t *testing.T) {
	root := setupGitRepo(t)
	a := learning.NewGitAnalyzer(root)

	commits, err := a.RecentCommits(10)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(commits), 2,
		"should find at least 2 commits")
}

// TestGitAnalyzer_RecentCommits_Fields verifies that each ChangeEntry has a
// non-empty Hash, Message, and Date.
func TestGitAnalyzer_RecentCommits_Fields(t *testing.T) {
	root := setupGitRepo(t)
	a := learning.NewGitAnalyzer(root)

	commits, err := a.RecentCommits(10)
	require.NoError(t, err)
	require.NotEmpty(t, commits)

	for _, c := range commits {
		assert.NotEmpty(t, c.Hash, "commit hash should not be empty")
		assert.NotEmpty(t, c.Message, "commit message should not be empty")
		assert.NotEmpty(t, c.Date, "commit date should not be empty")
	}
}

// TestGitAnalyzer_RecentCommits_ChangedFiles verifies that at least one
// commit reports a non-empty Files slice.
func TestGitAnalyzer_RecentCommits_ChangedFiles(t *testing.T) {
	root := setupGitRepo(t)
	a := learning.NewGitAnalyzer(root)

	commits, err := a.RecentCommits(10)
	require.NoError(t, err)
	require.NotEmpty(t, commits)

	atLeastOne := false
	for _, c := range commits {
		if len(c.Files) >= 1 {
			atLeastOne = true
			break
		}
	}
	assert.True(t, atLeastOne, "at least one commit should have >=1 changed file")
}

// TestGitAnalyzer_HotFiles verifies that at least 1 hot file is returned.
func TestGitAnalyzer_HotFiles(t *testing.T) {
	root := setupGitRepo(t)
	a := learning.NewGitAnalyzer(root)

	hot := a.HotFiles(10)
	assert.GreaterOrEqual(t, len(hot), 1,
		"should return at least 1 hot file")
}

// TestGitAnalyzer_HotFiles_Limit verifies that HotFiles respects the limit.
func TestGitAnalyzer_HotFiles_Limit(t *testing.T) {
	root := setupGitRepo(t)
	a := learning.NewGitAnalyzer(root)

	hot := a.HotFiles(1)
	assert.LessOrEqual(t, len(hot), 1,
		"HotFiles(1) should return at most 1 file")
}

// TestGitAnalyzer_NotARepo verifies that RecentCommits returns an error when
// the directory is not a git repository.
func TestGitAnalyzer_NotARepo(t *testing.T) {
	root := t.TempDir()
	a := learning.NewGitAnalyzer(root)

	_, err := a.RecentCommits(5)
	assert.Error(t, err, "should return an error for a non-git directory")
}
