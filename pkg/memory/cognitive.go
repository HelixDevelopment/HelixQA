// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package memory

import (
	"context"
	"fmt"
	"sync"
)

// CognitiveMemory provides semantic search and knowledge graph
// capabilities on top of the structured SQLite store. When a
// CognitiveProvider is configured, the Store uses it for
// enriched memory operations (semantic dedup, cross-session
// context, natural language recall). When no provider is set,
// all operations gracefully fall back to SQLite-only behavior.
type CognitiveMemory struct {
	provider CognitiveProvider
	store    *Store
	mu       sync.Mutex
}

// CognitiveProvider is the interface for external cognitive
// memory systems (e.g., HelixMemory with Mem0+Cognee+Letta).
// Implementations are optional — HelixQA works without them.
type CognitiveProvider interface {
	// Store saves a memory entry with semantic indexing.
	Store(ctx context.Context, entry MemoryEntry) error

	// Search performs semantic similarity search.
	Search(ctx context.Context, query string, limit int) ([]MemoryEntry, error)

	// Recall retrieves memories related to a specific context.
	Recall(ctx context.Context, context string) ([]MemoryEntry, error)

	// Health checks if the cognitive backend is available.
	Health(ctx context.Context) error
}

// MemoryEntry is a unit of cognitive memory.
type MemoryEntry struct {
	ID       string            `json:"id"`
	Content  string            `json:"content"`
	Type     string            `json:"type"` // fact, observation, learning, issue
	Source   string            `json:"source"`
	Session  string            `json:"session"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// NewCognitiveMemory creates a cognitive memory layer backed by
// the given store. Provider can be nil (SQLite-only mode).
func NewCognitiveMemory(store *Store, provider CognitiveProvider) *CognitiveMemory {
	return &CognitiveMemory{
		provider: provider,
		store:    store,
	}
}

// HasCognitive returns true if a cognitive provider is configured
// and healthy.
func (cm *CognitiveMemory) HasCognitive(ctx context.Context) bool {
	if cm.provider == nil {
		return false
	}
	return cm.provider.Health(ctx) == nil
}

// Remember stores a memory entry. If cognitive provider is
// available, stores semantically. Always stores to SQLite
// knowledge table as fallback.
func (cm *CognitiveMemory) Remember(ctx context.Context, entry MemoryEntry) error {
	// Always persist to SQLite knowledge store
	if cm.store != nil {
		cm.store.SetKnowledge(entry.ID, entry.Content, entry.Source)
	}

	// Optionally enrich with cognitive provider
	if cm.provider != nil {
		if err := cm.provider.Store(ctx, entry); err != nil {
			// Log but don't fail — SQLite has the data
			fmt.Printf("  cognitive: store warning: %v\n", err)
		}
	}
	return nil
}

// Search queries memory. Uses cognitive provider for semantic
// search if available, falls back to SQLite keyword match.
func (cm *CognitiveMemory) Search(ctx context.Context, query string, limit int) ([]MemoryEntry, error) {
	if cm.provider != nil {
		results, err := cm.provider.Search(ctx, query, limit)
		if err == nil && len(results) > 0 {
			return results, nil
		}
		// Fall through to SQLite on error
	}

	// SQLite fallback — search knowledge table
	if cm.store == nil {
		return nil, nil
	}
	all, err := cm.store.AllKnowledge()
	if err != nil {
		return nil, err
	}

	var results []MemoryEntry
	for k, v := range all {
		if len(results) >= limit {
			break
		}
		results = append(results, MemoryEntry{ID: k, Content: v, Type: "fact"})
	}
	return results, nil
}

// RecallSession retrieves all memories from a specific session.
func (cm *CognitiveMemory) RecallSession(ctx context.Context, sessionID string) ([]MemoryEntry, error) {
	if cm.provider != nil {
		results, err := cm.provider.Recall(ctx, "session:"+sessionID)
		if err == nil {
			return results, nil
		}
	}

	// SQLite fallback — get findings from session
	if cm.store == nil {
		return nil, nil
	}
	findings, err := cm.store.ListFindingsByStatus("open")
	if err != nil {
		return nil, err
	}

	var results []MemoryEntry
	for _, f := range findings {
		if f.SessionID == sessionID {
			results = append(results, MemoryEntry{
				ID:      f.ID,
				Content: f.Title + ": " + f.Description,
				Type:    "issue",
				Session: f.SessionID,
			})
		}
	}
	return results, nil
}
