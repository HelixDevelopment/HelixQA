// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"

	"digital.vasic.helixqa/pkg/i18n"
)

// helixqaT is the package-level call-site helper for the
// CONST-046 i18n seam migration of the helixqa CLI binary. It
// routes every user-facing literal through the active translator
// (round 112 seam) and falls back to the operator's English
// fallback only when the translator returns an empty string or an
// error.
//
// Contract:
//
//   - The default NoopTranslator returns the messageID verbatim
//     (non-empty), so call sites print the messageID rather than
//     the fallback until a real backend is installed via
//     i18n.SetTranslator. This is intentional — it surfaces the
//     untranslated state loudly during the rollout instead of
//     silently displaying English to non-English users.
//
//   - A real backend (LLM-driven generator or YAML-bundle loader)
//     installed via i18n.SetTranslator returns the localized
//     string for messageID. Call sites display that.
//
//   - An empty-string / error backend response falls back to the
//     operator's English fallback. This is the safety net for
//     misconfigured backends — the operator still sees something
//     intelligible even if the backend is broken.
//
// Per the round-205 anti-bluff contract (round-112 §11.4 tests
// already encode this), if a future change replaces a call to
// helixqaT(...) with the raw English literal directly, the
// migration-test sentinel assertions in i18n_migration_test.go
// FAIL — locking the seam at every migrated call site.
func helixqaT(ctx context.Context, messageID, fallback string) string {
	tr := i18n.ActiveTranslator()
	got, err := tr.T(ctx, messageID, nil)
	// A translator that returns the messageID verbatim has NOT
	// actually translated anything — that is the NoopTranslator
	// default (and the common "key-miss echo" behaviour of real
	// backends). Treat it identically to an empty/error response and
	// fall back to the operator's English fallback. Returning the raw
	// messageID to the call site is a reporting bluff: the *_fmt keys
	// (e.g. "helixqa_run_summary_failed_fmt") carry no `%d`/`%s`/`%v`
	// verbs, so a downstream fmt.Printf(helixqaT(...), args...) emits
	// the literal key followed by `%!(EXTRA …)` instead of the real
	// counts. The fallback string is the operator-authored format
	// string that DOES carry the verbs, so it must win whenever no
	// genuine translation is available.
	if err != nil || got == "" || got == messageID {
		return fallback
	}
	return got
}
