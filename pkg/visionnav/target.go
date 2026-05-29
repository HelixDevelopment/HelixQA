// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// target.go — runtime-registered exploration targets.
//
// A Target names a thing the autonomous Session drives: how to launch
// it (LaunchAction, in the Provider/executor action grammar) and which
// screen states count as "goal reached" (ScreenGoals, matched against
// captured OCR snapshots).
//
// This file is INTENTIONALLY project-agnostic. It ships with an EMPTY
// registry. Callers (any project — a TV vendor, a generic Android farm,
// a streaming-box maker) register the targets they care about at runtime
// via Register, mirroring the pkg/autonomous geo_probe RegisterEndpoint
// pattern. There are NO target literals in this package: no package
// names, no device serials, no launch intents. Coupling a target into
// this source would be a decoupling regression.
//
// Thread-safety: Register / Get / List / ResetTargetRegistry are guarded
// by targetRegistryMu (RWMutex), exactly as geo_probe guards its maps.

package visionnav

import (
	"fmt"
	"sort"
	"sync"
)

// Target describes one thing the Session explores.
type Target struct {
	// Name identifies the target in logs + Get lookups (e.g. a caller-
	// supplied package id or a friendly label). Required, unique.
	Name string
	// LaunchAction is the first action the Session dispatches to bring
	// the target on-screen, expressed in the executor's action grammar
	// (e.g. the caller's "launch <pkg>" string). Required.
	LaunchAction string
	// ScreenGoals are OCR-snapshot substrings; the Session treats the
	// target as reached when a captured Evidence.OCRSnapshot contains
	// any one of them (case-sensitive substring match). At least one
	// is required — a target with no goal can never PASS.
	ScreenGoals []string
}

// Validate returns an error if the Target is missing the fields the
// Session needs to drive it to a verdict.
func (t *Target) Validate() error {
	if t == nil {
		return fmt.Errorf("visionnav: nil Target")
	}
	if t.Name == "" {
		return fmt.Errorf("visionnav: Target.Name is empty")
	}
	if t.LaunchAction == "" {
		return fmt.Errorf("visionnav: Target %q has empty LaunchAction", t.Name)
	}
	if len(t.ScreenGoals) == 0 {
		return fmt.Errorf("visionnav: Target %q has no ScreenGoals "+
			"(no goal means the Session can never reach PASS)", t.Name)
	}
	for i, g := range t.ScreenGoals {
		if g == "" {
			return fmt.Errorf("visionnav: Target %q ScreenGoals[%d] is empty", t.Name, i)
		}
	}
	return nil
}

var (
	targetRegistryMu sync.RWMutex
	targets          = map[string]Target{}
)

// Register teaches the package about a Target. The Target is Validate()d
// before it is stored so a malformed target can't be smuggled in.
// Registering the same Name twice replaces the prior entry.
func Register(t Target) error {
	if err := t.Validate(); err != nil {
		return fmt.Errorf("visionnav: Register: %w", err)
	}
	// Defensive copy of the slice so a caller mutating its own slice
	// after Register can't reach into the stored Target.
	goals := make([]string, len(t.ScreenGoals))
	copy(goals, t.ScreenGoals)
	stored := Target{Name: t.Name, LaunchAction: t.LaunchAction, ScreenGoals: goals}

	targetRegistryMu.Lock()
	defer targetRegistryMu.Unlock()
	targets[t.Name] = stored
	return nil
}

// Get returns the registered Target for name and whether it was found.
// The returned Target carries a copy of ScreenGoals so the caller cannot
// mutate the registry's stored slice.
func Get(name string) (Target, bool) {
	targetRegistryMu.RLock()
	defer targetRegistryMu.RUnlock()
	t, ok := targets[name]
	if !ok {
		return Target{}, false
	}
	goals := make([]string, len(t.ScreenGoals))
	copy(goals, t.ScreenGoals)
	return Target{Name: t.Name, LaunchAction: t.LaunchAction, ScreenGoals: goals}, true
}

// List returns all registered target names, sorted for deterministic
// iteration (callers often log or table-drive over this).
func List() []string {
	targetRegistryMu.RLock()
	defer targetRegistryMu.RUnlock()
	names := make([]string, 0, len(targets))
	for n := range targets {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// ResetTargetRegistry clears all registered targets. Tests only —
// mirrors geo_probe.ResetGeoRegistry.
func ResetTargetRegistry() {
	targetRegistryMu.Lock()
	defer targetRegistryMu.Unlock()
	targets = map[string]Target{}
}
