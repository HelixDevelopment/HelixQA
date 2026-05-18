// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package i18n provides the translator seam for HelixQA's
// CONST-046 anti-hardcoded-content migration. Per CONST-046, no
// user-facing text may be hardcoded as a static literal; every
// such string is routed through a Translator implementation
// (LLM-backed at runtime, i18n-bundle-backed at test time, or
// the NoopTranslator pass-through default that preserves the
// pre-migration behaviour for downstream consumers that have not
// yet wired a real backend).
//
// HelixQA's CLI / library entry points are procedural — there is
// no central struct that owns all user-facing output — so this
// package exposes a package-level seam (SetTranslator + ActiveTranslator,
// see global.go) that mirrors the pattern already in production
// in panoptic (round 99a).
//
// The NoopTranslator returns the messageID verbatim, which means
// migrated call sites continue to display their pre-migration
// English text until a real Translator is installed via
// SetTranslator. This preserves backward compatibility while
// closing the CONST-046 violation at the source layer.
package i18n

import "context"

// Translator is the seam between HelixQA code and the runtime
// localization backend. It MUST be implemented by every backend
// (LLM-driven generator, YAML-bundle loader, fakeTranslator in
// tests) that HelixQA migrations consume.
//
// T returns the localized form of messageID, with templateData
// supplying any named placeholders the message references. The
// returned string MUST NOT be the empty string on success — an
// empty result is a backend bug per the CONST-046 contract.
//
// TPlural is the count-aware variant; backends MAY select among
// `zero`, `one`, `two`, `few`, `many`, `other` keyed variants
// per CLDR plural rules. NoopTranslator returns the messageID
// verbatim from both methods to preserve pre-migration text.
type Translator interface {
	T(ctx context.Context, messageID string, templateData map[string]any) (string, error)
	TPlural(ctx context.Context, messageID string, count int, templateData map[string]any) (string, error)
}

// NoopTranslator is the zero-value backend that returns the
// messageID verbatim. It exists so that downstream consumers
// (HelixQA's own CLI, importers of helix_qa packages, tests that
// don't care about translation) can rely on the Translator
// interface without wiring a real backend. Per CONST-046, the
// real backend (LLM-driven or i18n-bundle-loader) MUST be wired
// before user-visible release, but the Noop default keeps the
// pre-migration behaviour stable during the rollout.
type NoopTranslator struct{}

// T returns messageID verbatim. The error return is always nil;
// it exists to match the Translator interface contract.
func (NoopTranslator) T(_ context.Context, messageID string, _ map[string]any) (string, error) {
	return messageID, nil
}

// TPlural returns messageID verbatim, ignoring count. The error
// return is always nil; it exists to match the Translator
// interface contract.
func (NoopTranslator) TPlural(_ context.Context, messageID string, _ int, _ map[string]any) (string, error) {
	return messageID, nil
}
