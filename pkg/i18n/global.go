// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package i18n

import "sync"

// HelixQA's CLI entry points (cmd/helixqa/main.go and siblings)
// are procedural — there is no central struct receiver that
// every user-facing print routes through. To migrate hardcoded
// English literals to messageID-based lookups without rewriting
// the entire CLI command surface around an injected receiver,
// this file exposes a package-level translator seam:
//
//   - SetTranslator(t) installs a backend (LLM, YAML, fake).
//   - ActiveTranslator() returns the currently-installed backend;
//     callers translate via `i18n.ActiveTranslator().T(...)`.
//
// The default is NoopTranslator{}, so migrated call sites print
// the messageID verbatim until a real backend is installed. This
// preserves pre-migration English output for binaries that
// haven't wired the real backend yet, while routing every
// translated literal through the seam tests can intercept.
//
// Pattern mirrors panoptic/pkg/i18n/global.go (round 99a), kept
// deliberately small so the seam itself does not become a
// hardcoded-content surface.

var (
	activeMu sync.RWMutex
	active   Translator = NoopTranslator{}
)

// SetTranslator installs t as the active translator backend. If t
// is nil, the call is a no-op (the existing backend is retained)
// so accidental nil injections cannot silently break user output.
//
// Typical wiring points:
//   - Production binary main(): install an LLM-backed or
//     YAML-bundle-backed backend before parsing args.
//   - Test setup: install a fakeTranslator that returns a sentinel
//     for assertion (see translator_test.go for the canonical
//     pattern).
func SetTranslator(t Translator) {
	if t == nil {
		return
	}
	activeMu.Lock()
	defer activeMu.Unlock()
	active = t
}

// ActiveTranslator returns the currently-installed translator.
// Default is NoopTranslator{}, returned until SetTranslator has
// been called with a non-nil backend. Safe for concurrent reads.
func ActiveTranslator() Translator {
	activeMu.RLock()
	defer activeMu.RUnlock()
	return active
}

// ResetForTest restores the package-level translator to the Noop
// default. Tests that install a fakeTranslator MUST call this in
// their teardown (or via t.Cleanup) so the global state does not
// leak across tests. Exported for use by external test packages
// that import helix_qa/pkg/i18n.
func ResetForTest() {
	activeMu.Lock()
	defer activeMu.Unlock()
	active = NoopTranslator{}
}
