// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package learning provides types for capturing and querying structured
// knowledge about a project: its screens, API endpoints, documentation,
// recent git changes, component inventory, constraints, and known issues.
package learning

import (
	"fmt"
	"path/filepath"

	"digital.vasic.helixqa/pkg/memory"
)

// Screen describes a single navigable screen or view across any platform.
type Screen struct {
	Name       string
	Platform   string
	Route      string
	Component  string
	SourceFile string
}

// APIEndpoint describes a single HTTP endpoint exposed by the project.
type APIEndpoint struct {
	Method     string
	Path       string
	Handler    string
	SourceFile string
}

// DocEntry represents a documentation file tracked in the knowledge base.
type DocEntry struct {
	Path    string
	Title   string
	Content string
}

// ChangeEntry represents a single git commit recorded in the knowledge base.
type ChangeEntry struct {
	Hash    string
	Message string
	Date    string
	Files   []string
}

// KnowledgeBase holds a structured snapshot of everything HelixQA has learned
// about a project.  All slice fields are always non-nil; use the Add* helpers
// to insert entries with automatic deduplication.
type KnowledgeBase struct {
	ProjectName   string
	ProjectRoot   string
	Screens       []Screen
	APIEndpoints  []APIEndpoint
	Docs          []DocEntry
	RecentChanges []ChangeEntry
	Components    []string
	Constraints   []string
	KnownIssues   []string
	// Credentials holds discovered login credentials from
	// .env files and documentation (e.g. ADMIN_USERNAME,
	// ADMIN_PASSWORD). Used by navigation prompts so the
	// LLM knows how to log in without hardcoding.
	Credentials map[string]string
}

// NewKnowledgeBase returns a KnowledgeBase with all slice fields initialised
// to empty (not nil) slices.
func NewKnowledgeBase() *KnowledgeBase {
	return &KnowledgeBase{
		Credentials:   map[string]string{},
		Screens:       []Screen{},
		APIEndpoints:  []APIEndpoint{},
		Docs:          []DocEntry{},
		RecentChanges: []ChangeEntry{},
		Components:    []string{},
		Constraints:   []string{},
		KnownIssues:   []string{},
	}
}

// AddScreen appends s to the knowledge base.  If a screen with the same
// Name and Platform already exists the call is a no-op (deduplication).
func (kb *KnowledgeBase) AddScreen(s Screen) {
	for _, existing := range kb.Screens {
		if existing.Name == s.Name && existing.Platform == s.Platform {
			return
		}
	}
	kb.Screens = append(kb.Screens, s)
}

// AddEndpoint appends ep to the knowledge base.  If an endpoint with the
// same Method and Path already exists the call is a no-op (deduplication).
func (kb *KnowledgeBase) AddEndpoint(ep APIEndpoint) {
	for _, existing := range kb.APIEndpoints {
		if existing.Method == ep.Method && existing.Path == ep.Path {
			return
		}
	}
	kb.APIEndpoints = append(kb.APIEndpoints, ep)
}

// Summary returns a human-readable overview of the knowledge base, listing
// the count of each category of collected information.
func (kb *KnowledgeBase) Summary() string {
	name := kb.ProjectName
	if name == "" {
		name = "(unnamed)"
	}
	return fmt.Sprintf(
		"KnowledgeBase[%s]: screens=%d endpoints=%d docs=%d changes=%d components=%d constraints=%d known_issues=%d",
		name,
		len(kb.Screens),
		len(kb.APIEndpoints),
		len(kb.Docs),
		len(kb.RecentChanges),
		len(kb.Components),
		len(kb.Constraints),
		len(kb.KnownIssues),
	)
}

// BuildKnowledgeBase constructs a fully-populated KnowledgeBase for the
// project rooted at projectRoot. It composes ProjectReader, CodebaseMapper,
// and GitAnalyzer to gather all available information. If store is non-nil,
// open findings from the memory store are included in KnownIssues.
//
// Non-fatal errors (e.g. git not available, no android dirs) are silently
// swallowed so the caller always receives a usable, partially-populated base.
func BuildKnowledgeBase(projectRoot string, store *memory.Store) (*KnowledgeBase, error) {
	kb := NewKnowledgeBase()
	kb.ProjectName = filepath.Base(projectRoot)
	kb.ProjectRoot = projectRoot

	// ── docs ─────────────────────────────────────────────────────────────────
	reader := NewProjectReader(projectRoot)

	docs, err := reader.ReadDocs()
	if err != nil {
		return nil, fmt.Errorf("BuildKnowledgeBase: read docs: %w", err)
	}
	kb.Docs = append(kb.Docs, docs...)

	// ── CLAUDE.md files → more docs + constraints ─────────────────────────
	claudeDocs, err := reader.ReadClaudeMDs()
	if err != nil {
		return nil, fmt.Errorf("BuildKnowledgeBase: read CLAUDE.md files: %w", err)
	}
	kb.Docs = append(kb.Docs, claudeDocs...)
	kb.Constraints = reader.ExtractConstraints(claudeDocs)
	kb.Credentials = reader.ExtractCredentials(projectRoot)

	// ── API endpoints ────────────────────────────────────────────────────────
	mapper := NewCodebaseMapper(projectRoot)

	endpoints, err := mapper.ExtractAPIEndpoints()
	if err != nil {
		return nil, fmt.Errorf("BuildKnowledgeBase: extract API endpoints: %w", err)
	}
	for _, ep := range endpoints {
		kb.AddEndpoint(ep)
	}

	// ── web screens ──────────────────────────────────────────────────────────
	webScreens, err := mapper.ExtractWebScreens()
	if err != nil {
		return nil, fmt.Errorf("BuildKnowledgeBase: extract web screens: %w", err)
	}
	for _, s := range webScreens {
		kb.AddScreen(s)
	}

	// ── android screens ──────────────────────────────────────────────────────
	androidScreens, err := mapper.ExtractAndroidScreens()
	if err != nil {
		return nil, fmt.Errorf("BuildKnowledgeBase: extract android screens: %w", err)
	}
	for _, s := range androidScreens {
		kb.AddScreen(s)
	}

	// ── components ───────────────────────────────────────────────────────────
	kb.Components = mapper.DiscoverComponents()

	// ── recent git history ───────────────────────────────────────────────────
	git := NewGitAnalyzer(projectRoot)
	changes, err := git.RecentCommits(20)
	if err == nil {
		kb.RecentChanges = changes
	}
	// git errors (non-repo, git not installed) are non-fatal — leave empty.

	// ── open findings from memory store ─────────────────────────────────────
	if store != nil {
		findings, err := store.ListFindingsByStatus("open")
		if err != nil {
			return nil, fmt.Errorf("BuildKnowledgeBase: list open findings: %w", err)
		}
		for _, f := range findings {
			kb.KnownIssues = append(kb.KnownIssues,
				fmt.Sprintf("[%s] %s: %s", f.ID, f.Severity, f.Title))
		}
	}

	return kb, nil
}
