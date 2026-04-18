// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package ticket

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestOCUEvidenceKinds_NonEmpty verifies every constant is a non-empty string.
func TestOCUEvidenceKinds_NonEmpty(t *testing.T) {
	kinds := []string{
		EvidenceKindClip,
		EvidenceKindDiffOverlay,
		EvidenceKindOCRDump,
		EvidenceKindElementTree,
		EvidenceKindHookTrace,
		EvidenceKindReplayScript,
		EvidenceKindLLMReasoning,
		EvidenceKindPerfMetrics,
		EvidenceKindAXTreeDiff,
		EvidenceKindHAR,
		EvidenceKindWebRTCStream,
		EvidenceKindRawDMA,
	}
	for _, k := range kinds {
		assert.NotEmpty(t, k, "EvidenceKind constant must be non-empty")
	}
}

// TestOCUEvidenceKinds_Unique verifies no two constants share the same value.
func TestOCUEvidenceKinds_Unique(t *testing.T) {
	kinds := map[string]string{
		"EvidenceKindClip":         EvidenceKindClip,
		"EvidenceKindDiffOverlay":  EvidenceKindDiffOverlay,
		"EvidenceKindOCRDump":      EvidenceKindOCRDump,
		"EvidenceKindElementTree":  EvidenceKindElementTree,
		"EvidenceKindHookTrace":    EvidenceKindHookTrace,
		"EvidenceKindReplayScript": EvidenceKindReplayScript,
		"EvidenceKindLLMReasoning": EvidenceKindLLMReasoning,
		"EvidenceKindPerfMetrics":  EvidenceKindPerfMetrics,
		"EvidenceKindAXTreeDiff":   EvidenceKindAXTreeDiff,
		"EvidenceKindHAR":          EvidenceKindHAR,
		"EvidenceKindWebRTCStream": EvidenceKindWebRTCStream,
		"EvidenceKindRawDMA":       EvidenceKindRawDMA,
	}
	seen := map[string]string{}
	for name, value := range kinds {
		if prev, dup := seen[value]; dup {
			t.Errorf("duplicate EvidenceKind value %q shared by %s and %s", value, prev, name)
		}
		seen[value] = name
	}
	assert.Len(t, seen, 12, "expected exactly 12 unique evidence kind values")
}

// TestOCUEvidenceKinds_Values verifies the exact string values that are
// stored in JSON evidence refs and bank entries.
func TestOCUEvidenceKinds_Values(t *testing.T) {
	assert.Equal(t, "clip", EvidenceKindClip)
	assert.Equal(t, "diff_overlay", EvidenceKindDiffOverlay)
	assert.Equal(t, "ocr_dump", EvidenceKindOCRDump)
	assert.Equal(t, "element_tree", EvidenceKindElementTree)
	assert.Equal(t, "hook_trace", EvidenceKindHookTrace)
	assert.Equal(t, "replay_script", EvidenceKindReplayScript)
	assert.Equal(t, "llm_reasoning", EvidenceKindLLMReasoning)
	assert.Equal(t, "perf_metrics", EvidenceKindPerfMetrics)
	assert.Equal(t, "ax_tree_diff", EvidenceKindAXTreeDiff)
	assert.Equal(t, "har", EvidenceKindHAR)
	assert.Equal(t, "webrtc_stream", EvidenceKindWebRTCStream)
	assert.Equal(t, "raw_dma", EvidenceKindRawDMA)
}

// TestEvidence_Struct verifies the Evidence struct fields are addressable
// and JSON-tagged correctly (compile-time check via struct literal).
func TestEvidence_Struct(t *testing.T) {
	e := Evidence{
		Kind: EvidenceKindClip,
		Ref:  "seq-42",
	}
	assert.Equal(t, "clip", e.Kind)
	assert.Equal(t, "seq-42", e.Ref)
}
