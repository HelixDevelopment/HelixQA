// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package ticket

// OCU evidence kind constants produced by the automation pipeline.
// These are additive to any existing EvidenceKind constants and are
// intentionally plain strings so callers can store them in JSON
// without a code dependency on this package.
//
// Consumers reference exactly one of these constants in an Evidence.Kind
// field to declare what artefact type they are attaching to a ticket.
const (
	// EvidenceKindClip is a time-windowed video clip extracted from the
	// Recorder's ring buffer by ActionRecordClip.
	EvidenceKindClip = "clip"

	// EvidenceKindDiffOverlay is a side-by-side or blended image
	// produced by VisionPipeline.Diff that highlights pixel deltas
	// between two frames.
	EvidenceKindDiffOverlay = "diff_overlay"

	// EvidenceKindOCRDump is the full OCR text extraction output from
	// VisionPipeline.OCR for a given frame.
	EvidenceKindOCRDump = "ocr_dump"

	// EvidenceKindElementTree is a snapshot of the UI element tree
	// (AX tree or DOM structure) captured by an Observer backend.
	EvidenceKindElementTree = "element_tree"

	// EvidenceKindHookTrace is the raw hook-trace log produced by
	// ld_preload or plthook Observer backends recording function
	// calls made by the application under test.
	EvidenceKindHookTrace = "hook_trace"

	// EvidenceKindReplayScript is a `.ocu-replay` DSL script generated
	// by BuildReplayScript that reproduces the sequence of Actions
	// leading to a failure.
	EvidenceKindReplayScript = "replay_script"

	// EvidenceKindLLMReasoning is the full LLM planner reasoning
	// transcript (Evaluation + Memory + NextGoal entries) that
	// preceded the failing action.
	EvidenceKindLLMReasoning = "llm_reasoning"

	// EvidenceKindPerfMetrics is a JSON snapshot of performance
	// counters (CPU %, RAM, frame times) sampled at the moment of
	// failure.
	EvidenceKindPerfMetrics = "perf_metrics"

	// EvidenceKindAXTreeDiff is a structured diff of two consecutive
	// AX-tree snapshots showing exactly which accessibility nodes
	// changed (or failed to change) after an action.
	EvidenceKindAXTreeDiff = "ax_tree_diff"

	// EvidenceKindHAR is an HTTP Archive (HAR) recording of all
	// network requests made by the application during the test
	// session, captured via CDP or proxy.
	EvidenceKindHAR = "har"

	// EvidenceKindWebRTCStream is a reference to a WebRTC / WHIP
	// stream artefact (SDP, ICE candidates, or saved RTP dump)
	// produced by the Recorder's optional publisher.
	EvidenceKindWebRTCStream = "webrtc_stream"

	// EvidenceKindRawDMA is a raw DMA buffer dump captured directly
	// from the display controller, used when screenshot fallbacks are
	// unavailable (e.g. protected content, kernel-level capture).
	EvidenceKindRawDMA = "raw_dma"
)

// Evidence is a typed artefact reference attached to a Ticket or
// returned from FromAutomationResult. Kind is one of the
// EvidenceKind* constants; Ref is an opaque identifier (file path,
// frame sequence number, clip annotation, etc.) resolved by the
// evidence store.
//
// Evidence is intentionally a flat struct: callers that need
// additional metadata (size, MIME type, timestamp) wrap it or store
// metadata alongside the Ref in the evidence store.
type Evidence struct {
	// Kind classifies the artefact type. Use an EvidenceKind* constant.
	Kind string `json:"kind"`

	// Ref is an opaque key the evidence store uses to locate the actual
	// bytes (e.g. a relative file path or a frame sequence number).
	// Consumers must not compare Ref values across different Kind values.
	Ref string `json:"ref"`
}
