// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package record_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
	"digital.vasic.helixqa/pkg/nexus/record"
	"digital.vasic.helixqa/pkg/nexus/record/encoder"
)

// --- mock CaptureSource ---

type mockSource struct {
	frames chan contracts.Frame
}

func newMockSource(frames []contracts.Frame) *mockSource {
	ch := make(chan contracts.Frame, len(frames))
	for _, f := range frames {
		ch <- f
	}
	close(ch)
	return &mockSource{frames: ch}
}

func (m *mockSource) Name() string                                             { return "mock" }
func (m *mockSource) Start(_ context.Context, _ contracts.CaptureConfig) error { return nil }
func (m *mockSource) Stop() error                                              { return nil }
func (m *mockSource) Frames() <-chan contracts.Frame                           { return m.frames }
func (m *mockSource) Stats() contracts.CaptureStats                            { return contracts.CaptureStats{} }
func (m *mockSource) Close() error                                             { return nil }

// --- mock Encoder ---

type mockEncoder struct {
	encoded []contracts.Frame
	closed  bool
}

func (e *mockEncoder) Encode(f contracts.Frame) error {
	e.encoded = append(e.encoded, f)
	return nil
}

func (e *mockEncoder) Close() error {
	e.closed = true
	return nil
}

// --- mock WebRTCPublisher ---

type mockPublisher struct {
	url string
	err error
}

func (p *mockPublisher) Publish(_ context.Context) (string, error) {
	return p.url, p.err
}

// --- tests ---

// TestRecorder_AttachSource_Start_Stop exercises the happy path:
// attach source → Start → frames encoded → Stop closes encoder.
func TestRecorder_AttachSource_Start_Stop(t *testing.T) {
	base := time.Now()
	frames := []contracts.Frame{
		{Seq: 0, Timestamp: base, Width: 1920, Height: 1080},
		{Seq: 1, Timestamp: base.Add(time.Second), Width: 1920, Height: 1080},
		{Seq: 2, Timestamp: base.Add(2 * time.Second), Width: 1920, Height: 1080},
	}
	src := newMockSource(frames)
	enc := &mockEncoder{}
	rec := record.NewRecorder(64, enc)

	require.NoError(t, rec.AttachSource(src))
	require.NoError(t, rec.Start(context.Background(), record.RecordConfig{}))

	// Wait for drain goroutine to finish (source channel closed immediately).
	require.NoError(t, rec.Stop())

	assert.Len(t, enc.encoded, 3, "all 3 frames should be encoded")
	assert.True(t, enc.closed, "encoder must be closed on Stop")
}

// TestRecorder_Start_WithoutSource returns ErrNoSource.
func TestRecorder_Start_WithoutSource(t *testing.T) {
	rec := record.NewRecorder(64, nil)
	err := rec.Start(context.Background(), record.RecordConfig{})
	require.ErrorIs(t, err, record.ErrNoSource)
}

// TestRecorder_Start_AlreadyStarted returns ErrAlreadyStarted on second call.
func TestRecorder_Start_AlreadyStarted(t *testing.T) {
	src := newMockSource(nil)
	// Use a source whose channel stays open so drain doesn't exit.
	openSrc := &mockSource{frames: make(chan contracts.Frame)}
	rec := record.NewRecorder(64, nil)
	require.NoError(t, rec.AttachSource(openSrc))
	require.NoError(t, rec.Start(context.Background(), record.RecordConfig{}))
	err := rec.Start(context.Background(), record.RecordConfig{})
	require.ErrorIs(t, err, record.ErrAlreadyStarted)
	_ = rec.Stop()
	_ = src.Close()
}

// TestRecorder_Clip_WritesNonEmptyOutput verifies Clip produces JSON output
// for frames within the window.
func TestRecorder_Clip_WritesNonEmptyOutput(t *testing.T) {
	base := time.Now()
	frames := []contracts.Frame{
		{Seq: 0, Timestamp: base, Width: 1920, Height: 1080},
		{Seq: 1, Timestamp: base.Add(time.Second), Width: 1920, Height: 1080},
		{Seq: 2, Timestamp: base.Add(2 * time.Second), Width: 1920, Height: 1080},
	}
	src := newMockSource(frames)
	enc := &mockEncoder{}
	rec := record.NewRecorder(64, enc)
	require.NoError(t, rec.AttachSource(src))
	require.NoError(t, rec.Start(context.Background(), record.RecordConfig{}))
	require.NoError(t, rec.Stop())

	var buf bytes.Buffer
	err := rec.Clip(base.Add(time.Second), 4*time.Second, &buf, contracts.ClipOptions{})
	require.NoError(t, err)
	assert.NotEmpty(t, buf.String(), "Clip must write non-empty JSON output")
}

// TestRecorder_LiveStream_NoPublisher returns ErrNotWired when no publisher
// is attached.
func TestRecorder_LiveStream_NoPublisher(t *testing.T) {
	rec := record.NewRecorder(64, nil)
	_, err := rec.LiveStream(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not wired")
}

// TestRecorder_LiveStream_MockPublisher delegates to the injected publisher.
func TestRecorder_LiveStream_MockPublisher(t *testing.T) {
	rec := record.NewRecorder(64, nil)
	pub := &mockPublisher{url: "https://whip.local/session/abc", err: nil}
	rec.WithPublisher(pub)
	url, err := rec.LiveStream(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "https://whip.local/session/abc", url)
}

// TestRecorder_LiveStream_MockPublisher_Error propagates publisher errors.
func TestRecorder_LiveStream_MockPublisher_Error(t *testing.T) {
	rec := record.NewRecorder(64, nil)
	wantErr := errors.New("whip: no session slot")
	pub := &mockPublisher{err: wantErr}
	rec.WithPublisher(pub)
	_, err := rec.LiveStream(context.Background())
	require.ErrorIs(t, err, wantErr)
}

// TestRecorder_NilEncoder_NoEncode verifies a nil encoder skips encoding but
// still fills the ring (Clip works).
func TestRecorder_NilEncoder_NoEncode(t *testing.T) {
	base := time.Now()
	frames := []contracts.Frame{
		{Seq: 0, Timestamp: base, Width: 640, Height: 480},
	}
	src := newMockSource(frames)
	rec := record.NewRecorder(64, nil)
	require.NoError(t, rec.AttachSource(src))
	require.NoError(t, rec.Start(context.Background(), record.RecordConfig{}))
	require.NoError(t, rec.Stop())

	var buf bytes.Buffer
	require.NoError(t, rec.Clip(base, 2*time.Second, &buf, contracts.ClipOptions{}))
	assert.NotEmpty(t, buf.String())
}

// TestRecorder_Clip_WithAnnotation verifies ClipOptions.Annotation is appended.
func TestRecorder_Clip_WithAnnotation(t *testing.T) {
	base := time.Now()
	frames := []contracts.Frame{
		{Seq: 0, Timestamp: base, Width: 1920, Height: 1080},
	}
	src := newMockSource(frames)
	rec := record.NewRecorder(64, nil)
	require.NoError(t, rec.AttachSource(src))
	require.NoError(t, rec.Start(context.Background(), record.RecordConfig{}))
	require.NoError(t, rec.Stop())

	var buf bytes.Buffer
	opts := contracts.ClipOptions{Annotation: "login-screen-detected"}
	require.NoError(t, rec.Clip(base, 2*time.Second, &buf, opts))
	assert.Contains(t, buf.String(), "login-screen-detected")
}

// Verify encoder.ErrNotWired surfaces correctly from the production stubs.
func TestEncoder_ErrNotWired_IsDefinedInParentPackage(t *testing.T) {
	assert.NotNil(t, encoder.ErrNotWired)
	assert.Contains(t, encoder.ErrNotWired.Error(), "not wired")
}
