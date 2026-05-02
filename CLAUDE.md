# CLAUDE.md — HelixQA

> **NON-NEGOTIABLE PRIME DIRECTIVE:**
> **"We had been in position that all tests do execute with success and
> all Challenges as well, but in reality the most of the features does
> not work and can't be used! This MUST NOT be the case and execution
> of tests and Challenges MUST guarantee the quality, the completion
> and full usability by end users of the product!"**
> This statement is the foundational requirement of this project. Any
> agent dispatch, any CI configuration, any code review that allows
> green tests on broken features is a violation and MUST be rejected.

> **Constitution v2.2.0**: [Read the Constitution](https://github.com/HelixDevelopment/HelixPlay/blob/main/docs/research/chapters/MVP/05_Response/01_Constitution.md)
> All rules in Constitution §1-§21 are MANDATORY. No exception.
>
> **Amendments (2026-05-02):**
> - Anti-bluff: forbidden patterns include `assert.True(t, true)`,
>   `assert.NotNil(t, nil)`, constructor-only tests, mock-only
>   integration/E2E tests, and permanently skipped tests without
>   containerization plans.
> - Usability evidence mandatory per §6.7 (HelixQA visual assertion,
>   manual recording, or Challenge scenario).
> - Automatic negative-leg fault injection per §1.3 / §6.3 / §11.5.7 —
>   CI breaks each feature and verifies non-Unit tests fail.
> - `ValidateAntiBluff` unconditional; all challenges call `RecordAction()`.
> - Observable behaviour assertion ratio: at least 60% of assertions must verify
>   observable behaviour per §1.2.
> - Mutation score >= 85% enforced by `mutation_ratchet_challenge.sh` per §6.4.
> - The 18 Contract Clauses (R-01..R-18) codified in §17.
> - Eight Architectural Pillars codified in §18 — binding architectural decisions.
> - Performance SLAs codified in §19 — <=30ms LAN, <=50ms WAN at p999.
> - Technology Stack codified in §20 — mandatory technology choices.
> - Implementation Roadmap codified in §21 — 14 phases (P00–P13).

## Project Context
This submodule is part of the HelixPlay system.
See the [feature spec](https://github.com/HelixDevelopment/HelixPlay/blob/001-helixplay-system/specs/001-helixplay-system/spec.md).

## Submodule-Specific Notes
<!-- Add submodule-specific AI agent guidance here -->
