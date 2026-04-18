// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package ticket

import (
	automation "digital.vasic.helixqa/pkg/nexus/automation"
)

// FromAutomationResult translates an automation.Result's EvidenceRef
// slice into ticket Evidence entries. The mapping is 1-to-1: each
// EvidenceRef becomes one Evidence with the same Kind and Ref values.
//
// Callers may attach the returned slice to a Ticket via the Evidence
// field for downstream rendering and evidence-store resolution.
func FromAutomationResult(res automation.Result) []Evidence {
	out := make([]Evidence, 0, len(res.Evidence))
	for _, ref := range res.Evidence {
		out = append(out, Evidence{
			Kind: ref.Kind,
			Ref:  ref.Ref,
		})
	}
	return out
}
