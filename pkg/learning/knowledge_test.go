// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package learning_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/learning"
	"digital.vasic.helixqa/pkg/memory"
)

// TestKnowledgeBase_Empty verifies that a newly created KnowledgeBase has
// all slices initialised to empty (not nil) and an empty ProjectName.
func TestKnowledgeBase_Empty(t *testing.T) {
	kb := learning.NewKnowledgeBase()
	require.NotNil(t, kb)

	assert.Empty(t, kb.ProjectName)
	assert.Empty(t, kb.ProjectRoot)

	assert.NotNil(t, kb.Screens)
	assert.Len(t, kb.Screens, 0)

	assert.NotNil(t, kb.APIEndpoints)
	assert.Len(t, kb.APIEndpoints, 0)

	assert.NotNil(t, kb.Docs)
	assert.Len(t, kb.Docs, 0)

	assert.NotNil(t, kb.RecentChanges)
	assert.Len(t, kb.RecentChanges, 0)

	assert.NotNil(t, kb.Components)
	assert.Len(t, kb.Components, 0)

	assert.NotNil(t, kb.Constraints)
	assert.Len(t, kb.Constraints, 0)

	assert.NotNil(t, kb.KnownIssues)
	assert.Len(t, kb.KnownIssues, 0)
}

// TestKnowledgeBase_Summary verifies that the human-readable summary
// includes correct counts after adding 2 screens and 1 endpoint.
func TestKnowledgeBase_Summary(t *testing.T) {
	kb := learning.NewKnowledgeBase()
	kb.ProjectName = "Catalogizer"

	kb.AddScreen(learning.Screen{
		Name:       "Home",
		Platform:   "android",
		Route:      "/home",
		Component:  "HomeScreen",
		SourceFile: "HomeScreen.kt",
	})
	kb.AddScreen(learning.Screen{
		Name:       "Settings",
		Platform:   "web",
		Route:      "/settings",
		Component:  "SettingsPage",
		SourceFile: "SettingsPage.tsx",
	})
	kb.AddEndpoint(learning.APIEndpoint{
		Method:     "GET",
		Path:       "/api/v1/health",
		Handler:    "HealthHandler",
		SourceFile: "health.go",
	})

	summary := kb.Summary()

	assert.True(t, strings.Contains(summary, "2"), "summary should mention 2 screens")
	assert.True(t, strings.Contains(summary, "1"), "summary should mention 1 endpoint")
}

// TestKnowledgeBase_AddScreen verifies that adding the same screen twice
// (same Name + Platform) results in exactly one entry (deduplication).
func TestKnowledgeBase_AddScreen(t *testing.T) {
	kb := learning.NewKnowledgeBase()

	s := learning.Screen{
		Name:       "Dashboard",
		Platform:   "web",
		Route:      "/dashboard",
		Component:  "DashboardPage",
		SourceFile: "DashboardPage.tsx",
	}

	kb.AddScreen(s)
	kb.AddScreen(s) // duplicate

	assert.Len(t, kb.Screens, 1, "duplicate screen should not be added")
}

// TestKnowledgeBase_AddEndpoint verifies that adding the same endpoint twice
// (same Method + Path) results in exactly one entry (deduplication).
func TestKnowledgeBase_AddEndpoint(t *testing.T) {
	kb := learning.NewKnowledgeBase()

	ep := learning.APIEndpoint{
		Method:     "POST",
		Path:       "/api/v1/login",
		Handler:    "LoginHandler",
		SourceFile: "auth.go",
	}

	kb.AddEndpoint(ep)
	kb.AddEndpoint(ep) // duplicate

	assert.Len(t, kb.APIEndpoints, 1, "duplicate endpoint should not be added")
}

// setupKBProject creates a temporary git-initialised project with:
//   - docs/overview.md
//   - CLAUDE.md with a Constraints section
//   - catalog-api/main.go with one Gin route
//
// It returns the project root.
func setupKBProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()

	// Initialise git so GitAnalyzer.RecentCommits does not error.
	gitRun := func(args ...string) {
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
		require.NoError(t, err, "git %v failed: %s", args, out)
	}
	gitRun("git", "init", "-b", "main")
	gitRun("git", "config", "user.email", "test@test.com")
	gitRun("git", "config", "user.name", "Test")

	// docs/overview.md
	docsDir := filepath.Join(root, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o755))
	write(t, filepath.Join(docsDir, "overview.md"),
		"# Overview\n\nProject overview.\n")

	// CLAUDE.md with constraints
	write(t, filepath.Join(root, "CLAUDE.md"), `# CLAUDE

## Constraints

- Use Podman, not Docker
- All builds must be containerized
`)

	// catalog-api/main.go with one Gin route. The go.mod marker is
	// what HelixQA's auto-discovery uses to classify this directory as
	// a ComponentGoAPI — the library itself holds no hardcoded
	// reference to "catalog-api".
	apiDir := filepath.Join(root, "catalog-api")
	require.NoError(t, os.MkdirAll(apiDir, 0o755))
	write(t, filepath.Join(apiDir, "go.mod"), `module example.com/api

go 1.21

require github.com/gin-gonic/gin v1.9.1
`)
	write(t, filepath.Join(apiDir, "main.go"), `package main

import "github.com/gin-gonic/gin"

func main() {
	router := gin.Default()
	router.GET("/api/v1/health", healthHandler)
}
`)

	// Initial commit
	gitRun("git", "add", ".")
	gitRun("git", "commit", "-m", "feat: initial project")

	return root
}

// TestBuildKnowledgeBase verifies that BuildKnowledgeBase correctly populates
// a KnowledgeBase from a fake project with docs, CLAUDE.md, and a Gin route.
func TestBuildKnowledgeBase(t *testing.T) {
	root := setupKBProject(t)

	kb, err := learning.BuildKnowledgeBase(root, nil)
	require.NoError(t, err)
	require.NotNil(t, kb)

	// ProjectName should equal the base directory name.
	assert.Equal(t, filepath.Base(root), kb.ProjectName,
		"ProjectName should be the base of root")

	// ProjectRoot should be set.
	assert.Equal(t, root, kb.ProjectRoot)

	// Docs: at least the overview.md under docs/.
	assert.GreaterOrEqual(t, len(kb.Docs), 1,
		"should have at least 1 doc from docs/")

	// Constraints extracted from CLAUDE.md.
	assert.GreaterOrEqual(t, len(kb.Constraints), 1,
		"should extract at least 1 constraint from CLAUDE.md")

	// API endpoint extracted from catalog-api/main.go.
	assert.GreaterOrEqual(t, len(kb.APIEndpoints), 1,
		"should extract at least 1 API endpoint")

	// Components: catalog-api should be discovered.
	assert.Contains(t, kb.Components, "catalog-api",
		"catalog-api should be in discovered components")

	// RecentChanges: at least 1 commit.
	assert.GreaterOrEqual(t, len(kb.RecentChanges), 1,
		"should have at least 1 recent commit")
}

// TestBuildKnowledgeBase_WithMemory verifies that when a non-nil Store is
// provided, open findings are included in KnownIssues.
func TestBuildKnowledgeBase_WithMemory(t *testing.T) {
	root := setupKBProject(t)

	// Create an in-memory store with one open finding.
	dbPath := filepath.Join(t.TempDir(), "helix_test.db")
	store, err := memory.NewStore(dbPath)
	require.NoError(t, err)
	defer store.Close()

	// Seed a session (required by findings FK).
	_, execErr := store.DB().Exec(
		`INSERT INTO sessions (id, started_at, total_tests, passed, failed, findings_count, pass_number)
		 VALUES ('s1', '2026-01-01T00:00:00Z', 0, 0, 0, 0, 1)`)
	require.NoError(t, execErr)

	require.NoError(t, store.CreateFinding(memory.Finding{
		ID:        "HELIX-001",
		SessionID: "s1",
		Severity:  "high",
		Category:  "crash",
		Title:     "App crashes on launch",
		Status:    "open",
	}))

	kb, err := learning.BuildKnowledgeBase(root, store)
	require.NoError(t, err)
	require.NotNil(t, kb)

	// KnownIssues should include the title of the open finding.
	require.GreaterOrEqual(t, len(kb.KnownIssues), 1,
		"should have at least 1 known issue from open findings")

	combined := strings.Join(kb.KnownIssues, " ")
	assert.True(t,
		strings.Contains(combined, "App crashes on launch") ||
			strings.Contains(combined, "HELIX-001"),
		"known issues should reference the open finding; got: %v", kb.KnownIssues)
}
