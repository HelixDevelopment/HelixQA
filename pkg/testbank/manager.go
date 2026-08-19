// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package testbank

import (
	"fmt"
	"sort"
	"sync"

	"digital.vasic.challenges/pkg/challenge"

	"digital.vasic.helixqa/pkg/config"
)

// Manager loads and manages QA test banks, providing filtered
// access by platform, priority, category, and tags. It bridges
// to the Challenges bank for execution.
type Manager struct {
	mu        sync.RWMutex
	testCases map[string]*TestCase
	banks     []*BankFile
	sources   []string
}

// NewManager creates a new empty Manager.
func NewManager() *Manager {
	return &Manager{
		testCases: make(map[string]*TestCase),
	}
}

// LoadFile loads a YAML test bank file into the manager.
func (m *Manager) LoadFile(path string) error {
	bf, err := LoadFile(path)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range bf.TestCases {
		tc := &bf.TestCases[i]
		if _, exists := m.testCases[tc.ID]; exists {
			return fmt.Errorf(
				"duplicate test case ID: %s (from %s)",
				tc.ID, path,
			)
		}
		m.testCases[tc.ID] = tc
	}
	m.banks = append(m.banks, bf)
	m.sources = append(m.sources, path)
	return nil
}

// LoadDir loads all YAML test banks from a directory. Any file
// LoadDir declines to load (HXC-305: e.g. the legacy-prose twin of a
// YAML/JSON pair) is logged via twinLogger — see the package-level
// LoadDir's doc comment — so this call is never silent about what it
// skipped, even for callers that ignore the declined list. Callers
// that need the declined list structurally (to report it themselves)
// should call LoadDirVerbose instead.
func (m *Manager) LoadDir(dir string) error {
	declined, err := m.LoadDirVerbose(dir)
	for _, d := range declined {
		twinLogger.Printf("testbank: Manager.LoadDir(%s): declined %s: %s",
			dir, d.Path, d.Reason)
	}
	return err
}

// LoadDirVerbose loads all YAML test banks from a directory (as
// LoadDir does) and additionally returns every file LoadDir found on
// disk but declined to load, with a reason (HXC-305), so a caller
// (e.g. a CLI subcommand doing a directory scan) can report it
// explicitly instead of relying on the package logger's default
// destination.
func (m *Manager) LoadDirVerbose(dir string) ([]DeclinedFile, error) {
	res, err := LoadDirVerbose(dir)
	if err != nil {
		// LoadDirVerbose reports declined files ALONGSIDE an error on
		// EVERY abort path that could have declined anything — the id
		// floor, a duplicate id, an unparseable or emptied bank (its
		// `abort` helper is the single exit those all share).
		// Discarding them here defeats that: callers print the
		// declined lines they are handed, so a bare nil makes the CLI
		// silent about which twins were dropped on exactly the runs
		// where that is the diagnostic (HXC-305 round-6 BLOCKING A,
		// secondary). The enumeration above earned each of its entries
		// separately: round 6 carried the list on the id floor alone,
		// round 14 converted seven further aborts, and round 16 found
		// the eighth — the unparseable twin — still discarding it, so
		// "unparseable" only became true of this code in round 16. Each
		// was a path the enumeration already claimed before the code
		// delivered it.
		//
		// res.Banks is deliberately NOT consulted: on an error return
		// it holds only what loaded before the abort.
		if res != nil {
			return res.Declined, err
		}
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, bf := range res.Banks {
		for i := range bf.TestCases {
			tc := &bf.TestCases[i]
			if _, exists := m.testCases[tc.ID]; exists {
				return res.Declined, fmt.Errorf(
					"duplicate test case ID: %s",
					tc.ID,
				)
			}
			m.testCases[tc.ID] = tc
		}
		m.banks = append(m.banks, bf)
	}
	m.sources = append(m.sources, dir)
	return res.Declined, nil
}

// Get retrieves a test case by ID.
func (m *Manager) Get(id string) (*TestCase, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tc, ok := m.testCases[id]
	return tc, ok
}

// All returns all loaded test cases sorted by ID.
func (m *Manager) All() []*TestCase {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*TestCase, 0, len(m.testCases))
	for _, tc := range m.testCases {
		result = append(result, tc)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// ForPlatform returns test cases that target the given
// platform, sorted by priority (critical first).
func (m *Manager) ForPlatform(
	p config.Platform,
) []*TestCase {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*TestCase
	for _, tc := range m.testCases {
		if tc.AppliesToPlatform(p) {
			result = append(result, tc)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return priorityOrder(result[i].Priority) <
			priorityOrder(result[j].Priority)
	})
	return result
}

// ByCategory returns test cases filtered by category.
func (m *Manager) ByCategory(category string) []*TestCase {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*TestCase
	for _, tc := range m.testCases {
		if tc.Category == category {
			result = append(result, tc)
		}
	}
	return result
}

// ByPriority returns test cases filtered by priority.
func (m *Manager) ByPriority(p Priority) []*TestCase {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*TestCase
	for _, tc := range m.testCases {
		if tc.Priority == p {
			result = append(result, tc)
		}
	}
	return result
}

// ByTag returns test cases that have the given tag.
func (m *Manager) ByTag(tag string) []*TestCase {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*TestCase
	for _, tc := range m.testCases {
		for _, t := range tc.Tags {
			if t == tag {
				result = append(result, tc)
				break
			}
		}
	}
	return result
}

// Count returns the total number of loaded test cases.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.testCases)
}

// Sources returns the loaded file/directory paths.
func (m *Manager) Sources() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]string, len(m.sources))
	copy(result, m.sources)
	return result
}

// Banks returns the loaded bank files.
func (m *Manager) Banks() []*BankFile {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*BankFile, len(m.banks))
	copy(result, m.banks)
	return result
}

// ToDefinitions converts all loaded test cases to Challenges
// Definitions for the given platform.
func (m *Manager) ToDefinitions(
	platform config.Platform,
) []*challenge.Definition {
	cases := m.ForPlatform(platform)
	defs := make([]*challenge.Definition, len(cases))
	for i, tc := range cases {
		defs[i] = tc.ToDefinition()
	}
	return defs
}

// priorityOrder returns a numeric sort order for priorities.
func priorityOrder(p Priority) int {
	switch p {
	case PriorityCritical:
		return 0
	case PriorityHigh:
		return 1
	case PriorityMedium:
		return 2
	case PriorityLow:
		return 3
	default:
		return 4
	}
}
