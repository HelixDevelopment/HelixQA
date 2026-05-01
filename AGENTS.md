# AGENTS.md — HelixQA

> **NON-NEGOTIABLE PRIME DIRECTIVE:**
> **"We had been in position that all tests do execute with success and
> all Challenges as well, but in reality the most of the features does
> not work and can't be used! This MUST NOT be the case and execution
> of tests and Challenges MUST guarantee the quality, the completion
> and full usability by end users of the product!"**
> This statement is the foundational requirement of this project. Any
> agent dispatch, any CI configuration, any code review that allows
> green tests on broken features is a violation and MUST be rejected.

> **Constitution v2.1.0**: [Read the Constitution](https://github.com/HelixDevelopment/HelixPlay/blob/main/docs/research/chapters/MVP/05_Response/01_Constitution.md)
> All rules in Constitution §1-§18 are MANDATORY. No exception.
>
> **Amendments (2026-05-01):**
> - Anti-bluff enforcement strengthened: no vacuous assertions, no
>   constructor-only tests, no mock-only integration/E2E tests, no
>   untriaged skips.
> - Usability evidence mandatory per §6.7.
> - Automatic negative-leg fault injection per §1.3 / §6.3 / §11.5.7.
> - `ValidateAntiBluff` unconditional; all challenges call `RecordAction()`.
> - Container verifier `execCommand()` executes real commands.

## Repo state
This is a `vasic-digital` / `HelixDevelopment` submodule for HelixPlay.

## Critical constraints
- **Anti-bluff:** No placeholders, dead code, vacuous tests. Details in Constitution §1.
- **Containers only:** Every service, DB, build, test runs inside a container.
- **Decoupling:** Reusable components live in public `vasic-digital` submodules.
- **Tests:** 100% coverage across all ten types. Only Unit may use mocks.
- **R-18 Operational Integrity:** No command may suspend/hibernate/lock/terminate/crash the host.

## Git topology
`origin` fetch=GitHub, push=GitFlic. Four remotes configured.
Force-push requires explicit authorization. `--no-verify` is forbidden.
