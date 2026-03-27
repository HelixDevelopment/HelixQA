# Issue Tickets

Every finding discovered by HelixQA — whether from LLM vision analysis, crash detection, ANR detection, or memory leak analysis — is written as a structured markdown ticket in `docs/issues/`.

## File Naming

```
docs/issues/HELIX-NNN-short-slug.md
```

- `NNN` is a zero-padded sequence number auto-incremented per project
- The slug is derived from the finding title (lowercase, hyphenated)

Examples:
```
docs/issues/HELIX-001-login-button-not-visible-on-dark-theme.md
docs/issues/HELIX-042-memory-leak-in-media-browser.md
docs/issues/HELIX-103-contrast-ratio-below-wcag-aa.md
```

## Ticket Format

Each ticket has a YAML frontmatter block followed by a markdown body:

```yaml
---
id: HELIX-042
severity: high
category: visual
platform: android
screen: media-detail
status: open
found_date: 2026-03-27
session_id: session-20260327-143022
pass_number: 2
---

# Title describing the issue concisely

Detailed description of what was found and why it matters.

## Steps to Reproduce

1. Launch the app on Android device
2. Navigate to the media detail screen
3. Observe the issue

## Expected Behaviour

What should happen according to design specifications.

## Actual Behaviour

What actually happens, with evidence.

## Evidence

- Screenshot: `qa-results/session-20260327-143022/screenshots/test-007-media-detail-step-3.png`
- Video: `qa-results/session-20260327-143022/videos/test-007-media-detail.mp4`

## Analysis

LLM vision analysis conclusion: _"The title text is clipped at the right edge on screens
narrower than 400dp. The layout uses a fixed pixel width rather than match_parent."_
```

## Frontmatter Fields

| Field | Values | Description |
|-------|--------|-------------|
| `id` | `HELIX-NNN` | Unique ticket identifier |
| `severity` | `critical` `high` `medium` `low` `cosmetic` | Impact level |
| `category` | `visual` `ux` `accessibility` `performance` `functional` `brand` | Finding type |
| `platform` | `android` `androidtv` `web` `desktop` `api` | Where found |
| `screen` | string | Screen or page name |
| `status` | `open` `in_progress` `fixed` `verified` `reopened` `wontfix` | Lifecycle state |
| `found_date` | ISO date | Date the finding was first created |
| `session_id` | string | Session directory that produced this ticket |
| `pass_number` | integer | Which QA pass discovered this issue |

## Severity Levels

| Level | Meaning | Examples |
|-------|---------|---------|
| `critical` | App is broken or data is lost | Crash on launch, data corruption, auth bypass |
| `high` | Core feature is impaired | Login fails, video won't play, navigation broken |
| `medium` | Feature works but degraded | Slow load, minor layout break, misleading label |
| `low` | Minor inconvenience | Suboptimal spacing, non-critical typo |
| `cosmetic` | Purely aesthetic | Pixel-level misalignment, font weight inconsistency |

## Categories

| Category | Description |
|----------|-------------|
| `visual` | Layout, spacing, rendering, clipping, wrong colors |
| `ux` | Confusing interaction, missing feedback, broken flow |
| `accessibility` | Contrast ratio, touch target size, missing content descriptions |
| `performance` | Slow response, memory leak, excessive CPU usage |
| `functional` | Feature not working as specified |
| `brand` | Logo placement, brand color deviation, typography |

## Ticket Lifecycle

```
open → in_progress → fixed → verified
                           ↘ reopened → open
                   → wontfix
```

Tickets start as `open`. When a developer picks up the issue, they update the status to `in_progress`. After a fix is deployed, the status becomes `fixed`. On the next HelixQA pass, if the issue is no longer detected, the ticket is automatically updated to `verified`. If the same issue recurs, the status is set to `reopened` — this is automatic regression detection.

## Updating Ticket Status

Edit the YAML frontmatter directly:

```yaml
---
id: HELIX-042
status: fixed        # changed from "open"
fixed_date: 2026-03-28
fixed_by: developer-name
---
```

HelixQA reads the current status from the file on each session and skips re-reporting issues already marked `wontfix`.

## Integrating with External Systems

HELIX tickets are plain markdown files and can be converted to any issue tracker format.

### GitHub Issues

```bash
# Create a GitHub issue from a HELIX ticket
gh issue create \
  --title "$(grep '^# ' docs/issues/HELIX-042-*.md | sed 's/# //')" \
  --body "$(cat docs/issues/HELIX-042-*.md)" \
  --label "bug,qa"
```

### Jira / Linear

Parse the YAML frontmatter to extract fields and use the respective REST APIs to create issues programmatically. The structured frontmatter maps cleanly to issue tracker fields (severity → priority, category → label, screen → component).

## Related Pages

- [Memory System](/manual/memory) — how findings are tracked across sessions
- [Pipeline Phases](/pipeline) — how findings are created in Phase 4
- [Multi-Pass QA](/manual/multi-pass) — automatic regression detection across passes
