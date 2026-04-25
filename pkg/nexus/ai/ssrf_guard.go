// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package ai SSRF guard — thin adapter over the canonical
// digital.vasic.security/pkg/ssrf. All callers keep their existing
// public API (SSRFGuardConfig, Resolver, ValidateURL, ErrSSRFBlocked).
// The duplicated algorithm (~150 LOC) is removed; the guard logic lives
// exclusively in digital.vasic.security/pkg/ssrf.
package ai

import (
	"digital.vasic.security/pkg/ssrf"
)

// SSRFGuardConfig tunes the guard. Zero value is safe: all private
// ranges rejected. Type alias so callers can pass ssrf.Config literals.
type SSRFGuardConfig = ssrf.Config

// Resolver is the narrow DNS contract the guard needs.
// Type alias preserving the interface for callers that inject stubs.
type Resolver = ssrf.Resolver

// ErrSSRFBlocked is returned when the guard refuses a URL. Assigned
// from the canonical ssrf.ErrBlocked so errors.Is() chains work across
// the package boundary.
var ErrSSRFBlocked = ssrf.ErrBlocked

// ValidateURL parses target and runs every guard check, delegating to
// digital.vasic.security/pkg/ssrf.Validate. Returns ErrSSRFBlocked
// (wrapped with a reason) on rejection, nil on pass.
func ValidateURL(target string, cfg SSRFGuardConfig) error {
	return ssrf.Validate(target, cfg)
}
