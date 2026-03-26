// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package planning

import (
	"sort"
)

// PriorityRanker sorts a slice of PlannedTest by a
// deterministic priority order:
//
//  1. Priority ascending (1 = critical, runs first)
//  2. Tests with prior failures before passing tests
//  3. Existing tests before new tests
//  4. Alphabetical by ID (tiebreaker)
type PriorityRanker struct {
	priorFailures map[string]bool
}

// NewPriorityRanker creates a PriorityRanker. priorFailures
// maps test IDs that previously failed to true; passing nil
// is safe and treated as an empty map.
func NewPriorityRanker(priorFailures map[string]bool) *PriorityRanker {
	if priorFailures == nil {
		priorFailures = make(map[string]bool)
	}
	return &PriorityRanker{priorFailures: priorFailures}
}

// Rank returns a new sorted slice of PlannedTest. The
// original slice is never modified.
func (r *PriorityRanker) Rank(tests []PlannedTest) []PlannedTest {
	if len(tests) == 0 {
		return []PlannedTest{}
	}

	result := make([]PlannedTest, len(tests))
	copy(result, tests)

	sort.SliceStable(result, func(i, j int) bool {
		a, b := &result[i], &result[j]

		// 1. Priority ascending (lower number = higher
		//    urgency).
		if a.Priority != b.Priority {
			return a.Priority < b.Priority
		}

		// 2. Prior failures run before clean tests.
		aFailed := r.priorFailures[a.ID]
		bFailed := r.priorFailures[b.ID]
		if aFailed != bFailed {
			return aFailed
		}

		// 3. Existing tests run before new tests.
		if a.IsExisting != b.IsExisting {
			return a.IsExisting
		}

		// 4. Alphabetical by ID as a stable tiebreaker.
		return a.ID < b.ID
	})

	return result
}
