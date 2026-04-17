// Package nexus is the Helix Nexus automation layer: a unified Go-native
// adapter surface for browser, mobile, desktop, and AI-driven navigation.
// It absorbs patterns from OpenClaw (role-based element references,
// AI-friendly error translation, isolated browser profiles) while staying
// entirely inside the existing HelixQA stack.
//
// The package is subdivided so each platform has its own sub-package and a
// top-level adapter.go defines the shared interface. Phase 0 delivers the
// scaffolding and the interface contract; later phases fill in the driver
// implementations described in
// docs/plans/2026-04-17-helix-nexus-open-clawed-integration-plan.md.
package nexus
