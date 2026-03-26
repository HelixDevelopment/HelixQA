// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package memory_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStore_SetKnowledge verifies that a key-value pair is persisted and
// can be retrieved.
func TestStore_SetKnowledge(t *testing.T) {
	s := newTestStore(t)

	require.NoError(t, s.SetKnowledge("api.base_url", "http://localhost:8080", "config"))

	val, err := s.GetKnowledge("api.base_url")
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:8080", val)
}

// TestStore_SetKnowledge_Upsert verifies that setting the same key twice
// results in the second value winning (upsert semantics).
func TestStore_SetKnowledge_Upsert(t *testing.T) {
	s := newTestStore(t)

	require.NoError(t, s.SetKnowledge("env", "staging", "manual"))
	require.NoError(t, s.SetKnowledge("env", "production", "auto-detect"))

	val, err := s.GetKnowledge("env")
	require.NoError(t, err)
	assert.Equal(t, "production", val)
}

// TestStore_GetKnowledge_NotFound verifies that requesting a missing key
// returns an error.
func TestStore_GetKnowledge_NotFound(t *testing.T) {
	s := newTestStore(t)

	_, err := s.GetKnowledge("nonexistent.key")
	assert.Error(t, err)
}

// TestStore_AllKnowledge verifies that all stored key-value pairs are
// returned as a map.
func TestStore_AllKnowledge(t *testing.T) {
	s := newTestStore(t)

	require.NoError(t, s.SetKnowledge("k1", "v1", "src1"))
	require.NoError(t, s.SetKnowledge("k2", "v2", "src2"))
	require.NoError(t, s.SetKnowledge("k3", "v3", "src3"))

	all, err := s.AllKnowledge()
	require.NoError(t, err)
	assert.Len(t, all, 3)
	assert.Equal(t, "v1", all["k1"])
	assert.Equal(t, "v2", all["k2"])
	assert.Equal(t, "v3", all["k3"])
}
