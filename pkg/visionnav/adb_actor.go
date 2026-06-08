// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// adb_actor.go — Issues.md C6 (§GT.3) wiring: the real device ScreenActor.
//
// session.go defines the ScreenActor interface (Screenshot + Dispatch)
// the autonomous loop drives, and notes that pkg/navigator's ADBExecutor
// "satisfies it structurally" with "a Dispatch shim ... one line in the
// caller". THIS file is that adapter: it turns the small, project-agnostic
// action grammar the LLM produces into concrete executor calls.
//
// Decoupling (project-agnostic): visionnav must NOT import pkg/navigator
// (the session.go decoupling rule + the HelixQA-stays-generic mandate).
// So ADBActor is built against a minimal LOCAL interface, DeviceExecutor,
// that pkg/navigator.ADBExecutor satisfies structurally. There are NO
// project package names, device serials, or region literals in this file;
// the launch action's payload is whatever the caller's Target.LaunchAction
// carries (runtime data).
//
// Action grammar (generic, executor-neutral):
//
//	tap <x> <y>          → Click(x, y)
//	key <KEYCODE_...>    → KeyPress(keycode)
//	back                 → Back()
//	home                 → Home()
//	text <string...>     → Type(string)
//	launch <args...>     → Shell("monkey"/"am" style payload via Shell)
//	shell <args...>      → Shell(args)
//	noop / wait          → no executor call (lets a step observe only)
//
// Unknown verbs return an error rather than silently no-op — a silently
// swallowed action is the §11.4 PASS-bluff pattern (the loop would think
// it acted when it did not).

package visionnav

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// DeviceExecutor is the minimal executor contract ADBActor needs. It is
// the structural subset of pkg/navigator.ADBExecutor that the visionnav
// action grammar maps onto. Defining it here (consumer side, per Go
// idiom) keeps visionnav decoupled from any concrete executor package
// and lets tests supply a recording fake.
type DeviceExecutor interface {
	Screenshot(ctx context.Context) ([]byte, error)
	Click(ctx context.Context, x, y int) error
	Type(ctx context.Context, text string) error
	KeyPress(ctx context.Context, key string) error
	Back(ctx context.Context) error
	Home(ctx context.Context) error
	// Shell runs the command string verbatim as a single adb-shell
	// argument (so callers can chain with && / |). Matches
	// pkg/navigator.ADBExecutor.Shell's single-string signature.
	Shell(ctx context.Context, cmd string) ([]byte, error)
}

// ADBActor adapts a DeviceExecutor to the visionnav.ScreenActor
// interface, interpreting the generic action grammar.
type ADBActor struct {
	exec DeviceExecutor
}

// NewADBActor wires a DeviceExecutor (e.g. *navigator.ADBExecutor) as a
// visionnav.ScreenActor.
func NewADBActor(exec DeviceExecutor) (*ADBActor, error) {
	if exec == nil {
		return nil, fmt.Errorf("visionnav: NewADBActor: exec is nil")
	}
	return &ADBActor{exec: exec}, nil
}

// Screenshot delegates straight through.
func (a *ADBActor) Screenshot(ctx context.Context) ([]byte, error) {
	return a.exec.Screenshot(ctx)
}

// Dispatch parses one action string and performs it. The grammar is
// documented at the top of this file. An empty action, "noop", or
// "wait" is a deliberate observe-only step (no executor call); every
// other verb maps to exactly one executor method, and an unrecognised
// verb is a hard error (never a silent no-op).
func (a *ADBActor) Dispatch(ctx context.Context, action string) error {
	fields := strings.Fields(action)
	if len(fields) == 0 {
		// Empty action == observe-only step. Legitimate: the loop may
		// want a screenshot without acting.
		return nil
	}
	verb := strings.ToLower(fields[0])
	args := fields[1:]

	switch verb {
	case "noop", "wait", "observe":
		return nil

	case "tap", "click":
		if len(args) != 2 {
			return fmt.Errorf("visionnav: %q action needs 2 args (x y), got %d", verb, len(args))
		}
		x, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("visionnav: %q x %q not an int: %w", verb, args[0], err)
		}
		y, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("visionnav: %q y %q not an int: %w", verb, args[1], err)
		}
		return a.exec.Click(ctx, x, y)

	case "key", "keyevent":
		if len(args) != 1 {
			return fmt.Errorf("visionnav: %q action needs 1 arg (keycode), got %d", verb, len(args))
		}
		return a.exec.KeyPress(ctx, args[0])

	case "back":
		return a.exec.Back(ctx)

	case "home":
		return a.exec.Home(ctx)

	case "text", "type":
		if len(args) == 0 {
			return fmt.Errorf("visionnav: %q action needs text", verb)
		}
		return a.exec.Type(ctx, strings.Join(args, " "))

	case "launch", "shell":
		if len(args) == 0 {
			return fmt.Errorf("visionnav: %q action needs a command payload", verb)
		}
		_, err := a.exec.Shell(ctx, strings.Join(args, " "))
		return err

	default:
		return fmt.Errorf("visionnav: unknown action verb %q "+
			"(grammar: tap|key|back|home|text|launch|shell|noop)", verb)
	}
}
