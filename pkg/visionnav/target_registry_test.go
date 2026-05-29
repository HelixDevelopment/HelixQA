// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package visionnav

import (
	"reflect"
	"testing"
)

func TestTargetRegistry_RegisterGetList(t *testing.T) {
	ResetTargetRegistry()
	t.Cleanup(ResetTargetRegistry)

	// Empty registry by default (decoupling: no built-in targets).
	if got := List(); len(got) != 0 {
		t.Fatalf("fresh registry should be empty, got %v", got)
	}
	if _, ok := Get("nope"); ok {
		t.Fatalf("Get on empty registry should miss")
	}

	tgt := Target{
		Name:         "alpha",
		LaunchAction: "launch alpha",
		ScreenGoals:  []string{"Home", "Library"},
	}
	if err := Register(tgt); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := Register(Target{Name: "beta", LaunchAction: "launch beta", ScreenGoals: []string{"Settings"}}); err != nil {
		t.Fatalf("Register beta: %v", err)
	}

	got, ok := Get("alpha")
	if !ok {
		t.Fatalf("Get(alpha) miss after Register")
	}
	if got.LaunchAction != "launch alpha" || !reflect.DeepEqual(got.ScreenGoals, []string{"Home", "Library"}) {
		t.Fatalf("Get(alpha) returned %+v", got)
	}

	// List is sorted + complete.
	if got := List(); !reflect.DeepEqual(got, []string{"alpha", "beta"}) {
		t.Fatalf("List = %v, want [alpha beta]", got)
	}
}

func TestTargetRegistry_DefensiveCopy(t *testing.T) {
	ResetTargetRegistry()
	t.Cleanup(ResetTargetRegistry)

	goals := []string{"Home"}
	if err := Register(Target{Name: "x", LaunchAction: "launch x", ScreenGoals: goals}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	// Mutate the caller's slice; registry must be unaffected.
	goals[0] = "MUTATED"
	got, _ := Get("x")
	if got.ScreenGoals[0] != "Home" {
		t.Fatalf("registry leaked caller slice: %v", got.ScreenGoals)
	}
	// Mutate the returned slice; a second Get must be unaffected.
	got.ScreenGoals[0] = "TAMPERED"
	got2, _ := Get("x")
	if got2.ScreenGoals[0] != "Home" {
		t.Fatalf("Get returned a live registry slice: %v", got2.ScreenGoals)
	}
}

func TestTargetRegistry_RejectsInvalid(t *testing.T) {
	ResetTargetRegistry()
	t.Cleanup(ResetTargetRegistry)

	cases := []Target{
		{Name: "", LaunchAction: "go", ScreenGoals: []string{"g"}},     // no name
		{Name: "n", LaunchAction: "", ScreenGoals: []string{"g"}},      // no launch action
		{Name: "n", LaunchAction: "go", ScreenGoals: nil},              // no goals
		{Name: "n", LaunchAction: "go", ScreenGoals: []string{"", "g"}}, // empty goal
	}
	for i, c := range cases {
		if err := Register(c); err == nil {
			t.Fatalf("case %d: Register should reject %+v", i, c)
		}
	}
	if got := List(); len(got) != 0 {
		t.Fatalf("invalid registrations leaked into registry: %v", got)
	}
}
