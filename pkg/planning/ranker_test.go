// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package planning

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPriorityRanker_SortByPriority(t *testing.T) {
	ranker := NewPriorityRanker(nil)

	tests := []PlannedTest{
		{ID: "T4", Name: "low priority", Priority: 4},
		{ID: "T2", Name: "medium priority", Priority: 2},
		{ID: "T1", Name: "critical", Priority: 1},
		{ID: "T3", Name: "high priority", Priority: 3},
	}

	ranked := ranker.Rank(tests)

	require.Len(t, ranked, 4)
	assert.Equal(t, "T1", ranked[0].ID)
	assert.Equal(t, "T2", ranked[1].ID)
	assert.Equal(t, "T3", ranked[2].ID)
	assert.Equal(t, "T4", ranked[3].ID)
}

func TestPriorityRanker_ExistingFirst(t *testing.T) {
	ranker := NewPriorityRanker(nil)

	tests := []PlannedTest{
		{ID: "NEW-1", Name: "new test", Priority: 1, IsNew: true},
		{ID: "EX-1", Name: "existing test", Priority: 1, IsExisting: true},
	}

	ranked := ranker.Rank(tests)

	require.Len(t, ranked, 2)
	assert.Equal(t, "EX-1", ranked[0].ID,
		"existing test should come before new test at same priority")
	assert.Equal(t, "NEW-1", ranked[1].ID)
}

func TestPriorityRanker_WithFailHistory(t *testing.T) {
	priorFailures := map[string]bool{
		"FAILED-1": true,
	}
	ranker := NewPriorityRanker(priorFailures)

	tests := []PlannedTest{
		// Both priority 2, both existing — but FAILED-1 has
		// prior failure so it should be boosted ahead of OK-1.
		{ID: "OK-1", Name: "ok test", Priority: 2, IsExisting: true},
		{ID: "FAILED-1", Name: "previously failed", Priority: 2, IsExisting: true},
	}

	ranked := ranker.Rank(tests)

	require.Len(t, ranked, 2)
	assert.Equal(t, "FAILED-1", ranked[0].ID,
		"previously failed test should be ranked first")
	assert.Equal(t, "OK-1", ranked[1].ID)
}

func TestPriorityRanker_EmptyList(t *testing.T) {
	ranker := NewPriorityRanker(nil)
	ranked := ranker.Rank(nil)
	assert.Empty(t, ranked)

	ranked = ranker.Rank([]PlannedTest{})
	assert.Empty(t, ranked)
}
