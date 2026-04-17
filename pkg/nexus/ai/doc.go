// Package ai is the Nexus autonomous-navigation layer. It wraps
// multimodal LLMs via the existing LLMProvider / LLMOrchestrator
// submodules and exposes four capabilities:
//
//   Navigator  — picks the next Action from a screenshot + tree + goal.
//   Healer     — recovers a broken selector by cross-referencing a
//                fresh Snapshot with the last-known description.
//   Generator  — converts a natural-language user story into a ready-
//                to-run bank YAML block.
//   Predictor  — flags known-flaky tests before they run, with a
//                per-session retry budget.
//
// Every LLM call goes through CostTracker so a session never exceeds
// the operator-configured budget (NEXUS_LLM_BUDGET_USD). Budget breach
// is a hard abort, not a warning.
package ai
