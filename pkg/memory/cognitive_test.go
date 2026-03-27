// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package memory

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCognitiveProvider implements CognitiveProvider for testing.
type mockCognitiveProvider struct {
	entries []MemoryEntry
	healthy bool
}

func (m *mockCognitiveProvider) Store(_ context.Context, entry MemoryEntry) error {
	m.entries = append(m.entries, entry)
	return nil
}

func (m *mockCognitiveProvider) Search(_ context.Context, query string, limit int) ([]MemoryEntry, error) {
	if !m.healthy {
		return nil, fmt.Errorf("provider unhealthy")
	}
	var results []MemoryEntry
	for _, e := range m.entries {
		if len(results) >= limit {
			break
		}
		results = append(results, e)
	}
	return results, nil
}

func (m *mockCognitiveProvider) Recall(_ context.Context, ctx string) ([]MemoryEntry, error) {
	return m.entries, nil
}

func (m *mockCognitiveProvider) Health(_ context.Context) error {
	if !m.healthy {
		return fmt.Errorf("unhealthy")
	}
	return nil
}

func TestCognitiveMemory_WithProvider(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer store.Close()

	provider := &mockCognitiveProvider{healthy: true}
	cm := NewCognitiveMemory(store, provider)

	assert.True(t, cm.HasCognitive(context.Background()))

	err = cm.Remember(context.Background(), MemoryEntry{
		ID: "mem-001", Content: "Login screen has 2 fields", Type: "fact", Source: "qa-pass-1",
	})
	require.NoError(t, err)
	assert.Len(t, provider.entries, 1)

	// Also stored in SQLite
	val, err := store.GetKnowledge("mem-001")
	require.NoError(t, err)
	assert.Equal(t, "Login screen has 2 fields", val)
}

func TestCognitiveMemory_WithoutProvider(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer store.Close()

	cm := NewCognitiveMemory(store, nil)

	assert.False(t, cm.HasCognitive(context.Background()))

	// Remember still works (SQLite only)
	err = cm.Remember(context.Background(), MemoryEntry{
		ID: "mem-002", Content: "Dashboard has 5 widgets", Type: "fact",
	})
	require.NoError(t, err)

	val, _ := store.GetKnowledge("mem-002")
	assert.Equal(t, "Dashboard has 5 widgets", val)
}

func TestCognitiveMemory_SearchFallback(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer store.Close()

	store.SetKnowledge("k1", "value1", "test")
	store.SetKnowledge("k2", "value2", "test")

	cm := NewCognitiveMemory(store, nil)
	results, err := cm.Search(context.Background(), "query", 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 2)
}

func TestCognitiveMemory_SearchWithProvider(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer store.Close()

	provider := &mockCognitiveProvider{
		healthy: true,
		entries: []MemoryEntry{
			{ID: "sem-1", Content: "Semantic result", Type: "fact"},
		},
	}

	cm := NewCognitiveMemory(store, provider)
	results, err := cm.Search(context.Background(), "query", 10)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "Semantic result", results[0].Content)
}

func TestCognitiveMemory_RecallSession(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer store.Close()

	// Create a finding in the session
	store.CreateFinding(Finding{
		ID: "HELIX-TEST", SessionID: "session-1", Severity: "low",
		Category: "ux", Title: "Test issue", Description: "Details", Status: "open",
	})

	cm := NewCognitiveMemory(store, nil)
	results, err := cm.RecallSession(context.Background(), "session-1")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 1)
}

func TestCognitiveMemory_NilStore(t *testing.T) {
	cm := NewCognitiveMemory(nil, nil)
	assert.False(t, cm.HasCognitive(context.Background()))

	// Should not panic
	err := cm.Remember(context.Background(), MemoryEntry{ID: "x", Content: "y"})
	assert.NoError(t, err)

	results, err := cm.Search(context.Background(), "q", 5)
	assert.NoError(t, err)
	assert.Nil(t, results)
}
