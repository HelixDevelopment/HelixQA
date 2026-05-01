# AGENTS.md — HelixQA

> **Constitution v2.0.0**: [Read the Constitution](https://github.com/HelixDevelopment/HelixPlay/blob/main/docs/research/chapters/MVP/05_Response/01_Constitution.md)
> All rules in Constitution §1-§18 are MANDATORY. No exception.

## Repo state
This is a `vasic-digital` / `HelixDevelopment` submodule for HelixPlay.
Specs live in `docs/research/chapters/MVP/` — treat as source of truth.

## Git topology
Four remotes; `origin` is **split**: fetch from GitHub, push to GitFlic.

```bash
github      git@github.com:HelixDevelopment/HelixPlay.git
gitlab      git@gitlab.com:helixdevelopment1/HelixPlay.git
gitverse    git@gitverse.ru:helixdevelopment/HelixPlay.git
gitflic     git@gitflic.ru:helixdevelopment/helixplay.git
origin      fetch=github, push=gitflic
```

When operator says "push", confirm which mirror — `origin` only updates GitFlic. Force-push requires explicit authorization. `--no-verify` is forbidden.

## Critical constraints

These are mandatory project-wide rules, not suggestions:

- **Anti-bluff:** No `TODO`, `FIXME`, `XXX`, `placeholder`, empty function bodies, dead code, or tests that pass without exercising real behaviour. Details in Constitution §1. Explicitly forbidden: `assert.True(t, true)`, `assert.NotNil(t, nil)`, and any tautologically true assertion. Tests MUST confirm that all tested codebase really works as expected and can be used by end users.
- **Containers only:** Every service, DB, build step, test runner, and scanner runs inside a container. Definitions live in `vasic-digital/Containers` — never vendor a `Dockerfile` outside that submodule. No faking a local toolchain.
- **Decoupling:** Reusable components live in **public** `vasic-digital` Git/Go submodules. Reuse before recreating.

## Agent instructions
<!-- Add submodule-specific agent instructions here -->
