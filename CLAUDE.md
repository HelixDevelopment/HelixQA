# CLAUDE.md — HelixQA

> **Constitution v2.1.0**: [Read the Constitution](https://github.com/HelixDevelopment/HelixPlay/blob/main/docs/research/chapters/MVP/05_Response/01_Constitution.md)
> All rules in Constitution §1-§18 are MANDATORY. No exception.
>
> **Amendments (2026-05-01):**
> - Anti-bluff: forbidden patterns include `assert.True(t, true)`,
>   `assert.NotNil(t, nil)`, constructor-only tests, mock-only
>   integration/E2E tests, and permanently skipped tests without
>   containerization plans.
> - Usability evidence mandatory per §6.7 (HelixQA visual assertion,
>   manual recording, or Challenge scenario).
> - Automatic negative-leg fault injection per §1.3 / §6.3 / §11.5.7 —
>   CI breaks each feature and verifies non-Unit tests fail.
> - `ValidateAntiBluff` unconditional; all challenges call `RecordAction()`.

## Project Context
This submodule is part of the HelixPlay system.
See the [feature spec](https://github.com/HelixDevelopment/HelixPlay/blob/001-helixplay-system/specs/001-helixplay-system/spec.md).

## Submodule-Specific Notes
<!-- Add submodule-specific AI agent guidance here -->
