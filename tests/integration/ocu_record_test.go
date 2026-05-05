//go:build integration
// +build integration

// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// BLUFF-VIOLATION: R-12 — This integration test uses intMockSource (mock capture source).
// Mocks are permitted ONLY in Unit tests per Constitution §6 / R-12.
// Remediation: Replace with real capture source or containerized test fixture.
// Tracked in: docs/research/chapters/MVP/05_Response/anti_bluff_audit_2026-05-02.md

package integration_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
	"digital.vasic.helixqa/pkg/nexus/record"
	"digital.vasic.helixqa/pkg/nexus/record/encoder"
	// blank imports so each encoder's init() registers its factory kind
	_ "digital.vasic.helixqa/pkg/nexus/record/encoder/nvenc"
	_ "digital.vasic.helixqa/pkg/nexus/record/encoder/vaapi"
	_ "digital.vasic.helixqa/pkg/nexus/record/encoder/x264"
)

// --- in-memory mock source for integration tests ---

type intMockSource struct {
	frames chan contracts.Frame
}

func newIntMockSource(n int) *intMockSource {
	ch := make(chan contracts.Frame, n)
	base := time.Now()
	for i := 0; i < n; i++ {
		ch <- contracts.Frame{
			Seq:       uint64(i),
			Timestamp: base.Add(time.Duration(i) * time.Millisecond * 33), // ~30 fps
			Width:     1920,
			Height:    1080,
		}
	}
	close(ch)
	return &intMockSource{frames: ch}
}

func (m *intMockSource) Name() string                                              { return "integration-mock" }
func (m *intMockSource) Start(_ context.Context, _ contracts.CaptureConfig) error { return nil }
func (m *intMockSource) Stop() error                                               { return nil }
func (m *intMockSource) Frames() <-chan contracts.Frame                            { return m.frames }
func (m *intMockSource) Stats() contracts.CaptureStats                            { return contracts.CaptureStats{} }
func (m *intMockSource) Close() error                                              { return nil }

// --- mock encoder for integration tests ---

type intMockEncoder struct{ count int }

func (e *intMockEncoder) Encode(_ contracts.Frame) error { e.count++; return nil }
func (e *intMockEncoder) Close() error                   { return nil }

// TestOCU_Record_InMemorySource constructs a Recorder with an in-memory mock
// source + mock encoder, records 10 frames, verifies Clip produces non-empty
// output, and confirms Stop is clean.
func TestOCU_Record_InMemorySource(t *testing.T) {
	const frameCount = 10
	src := newIntMockSource(frameCount)
	enc := &intMockEncoder{}
	rec := record.NewRecorder(64, enc)

	require.NoError(t, rec.AttachSource(src))
	require.NoError(t, rec.Start(context.Background(), record.RecordConfig{}))
	// Source channel is pre-closed; drain goroutine exits immediately.
	require.NoError(t, rec.Stop())

	// All frames must have reached the encoder.
	assert.Equal(t, frameCount, enc.count, "all frames must be forwarded to encoder")

	// Clip with zero window returns all frames.
	var buf bytes.Buffer
	err := rec.Clip(time.Now(), 0, &buf, contracts.ClipOptions{Annotation: "integration-test"})
	require.NoError(t, err)
	assert.NotEmpty(t, buf.String(), "Clip must write non-empty JSON output")
	assert.Contains(t, buf.String(), "integration-test", "annotation must appear in Clip output")
}

// TestOCU_Record_EncoderKindsRegistered asserts all three P5 encoder kinds
// are discoverable through the factory after blank-importing their packages.
func TestOCU_Record_EncoderKindsRegistered(t *testing.T) {
	kinds := encoder.Kinds()
	for _, want := range []string{"x264", "nvenc", "vaapi"} {
		require.Contains(t, kinds, want, "encoder kind %q must be registered via init()", want)
	}
}
