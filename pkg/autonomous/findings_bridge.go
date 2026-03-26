// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package autonomous

import (
	"fmt"
	"time"

	"digital.vasic.helixqa/pkg/analysis"
	"digital.vasic.helixqa/pkg/memory"
)

// FindingsBridge connects the analysis pipeline to the memory store and
// optionally the issues directory on disk. It translates AnalysisFinding
// values produced by vision analysis into persisted memory.Finding records
// and Markdown issue files.
type FindingsBridge struct {
	store     *memory.Store
	issuesDir string
	sessionID string
}

// NewFindingsBridge creates a FindingsBridge backed by the given store.
// issuesDir is optional: when non-empty, each finding is also written to
// that directory as a Markdown file. sessionID is attached to every
// finding that the bridge persists.
func NewFindingsBridge(
	store *memory.Store,
	issuesDir string,
	sessionID string,
) *FindingsBridge {
	return &FindingsBridge{
		store:     store,
		issuesDir: issuesDir,
		sessionID: sessionID,
	}
}

// Process persists each AnalysisFinding to the store and, when an
// issuesDir is configured, writes a Markdown file per finding. It
// returns the slice of HELIX-NNN identifiers that were created.
//
// Process is a no-op (returns nil, nil) when the store is nil or the
// findings slice is empty.
func (b *FindingsBridge) Process(
	findings []analysis.AnalysisFinding,
) ([]string, error) {
	if b.store == nil || len(findings) == 0 {
		return nil, nil
	}

	today := time.Now().UTC().Format("2006-01-02")
	ids := make([]string, 0, len(findings))

	for _, af := range findings {
		id, err := b.store.NextFindingID()
		if err != nil {
			return ids, fmt.Errorf(
				"findings_bridge: next finding id: %w", err,
			)
		}

		mf := memory.Finding{
			ID:          id,
			SessionID:   b.sessionID,
			Severity:    string(af.Severity),
			Category:    string(af.Category),
			Title:       af.Title,
			Description: af.Description,
			ReproSteps:  af.ReproSteps,
			EvidencePaths: af.Evidence,
			Platform:    af.Platform,
			Screen:      af.Screen,
			Status:      "open",
			FoundDate:   today,
		}

		if err := b.store.CreateFinding(mf); err != nil {
			return ids, fmt.Errorf(
				"findings_bridge: create finding %q: %w", id, err,
			)
		}

		if b.issuesDir != "" {
			if _, err := mf.WriteToDir(b.issuesDir); err != nil {
				return ids, fmt.Errorf(
					"findings_bridge: write finding %q to dir: %w",
					id, err,
				)
			}
		}

		ids = append(ids, id)
	}

	return ids, nil
}
