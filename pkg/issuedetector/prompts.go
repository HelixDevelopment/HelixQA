// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package issuedetector

// Versioned prompt templates for LLM-based issue detection.
// Version changes are tracked for reproducibility.

const promptVersion = "v1"

const actionAnalysisPrompt = `You are a QA engineer analyzing a UI action.

Before the action "%s" on screen "%s":
- Screen had %d elements
- Screen description: %s

After the action:
- Screen has %d elements
- Screen description: %s

Analyze if anything went wrong. Look for:
1. Visual bugs (truncation, overlap, misalignment)
2. Functional issues (wrong screen, unexpected state)
3. Performance concerns (if transition was slow)

Return a JSON array of issues found, or empty array if everything looks fine:
[{"category":"visual","severity":"medium","title":"...","description":"...","suggestion":"..."}]`

const uxAnalysisPrompt = `You are a UX expert analyzing a navigation graph.

The app has %d screens with %d transitions.
Coverage: %.0f%% of screens visited.

Unvisited screens: %s

Navigation depth (avg transitions from home): varies.

Analyze the navigation structure for UX issues:
1. Dead ends (screens with no way back)
2. Excessive depth (too many steps to reach common features)
3. Missing back navigation
4. Confusing or inconsistent navigation patterns

Return a JSON array of UX issues found:
[{"category":"ux","severity":"medium","title":"...","description":"...","suggestion":"..."}]`

const accessibilityAnalysisPrompt = `You are an accessibility expert analyzing a screen.

Screen: %s (%s)
Elements: %d total, %d clickable
Text regions: %d

Element details:
%s

Analyze for accessibility issues:
1. Low contrast text (WCAG AA minimum 4.5:1)
2. Missing labels on interactive elements
3. Touch targets too small (< 44x44 dp on mobile)
4. Missing content descriptions
5. Color-only information conveyed

Return a JSON array of accessibility issues:
[{"category":"accessibility","severity":"medium","title":"...","description":"...","suggestion":"..."}]`
