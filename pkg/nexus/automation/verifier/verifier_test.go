// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package verifier

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// ── stubs ──────────────────────────────────────────────────────────────────

type stubFrameData struct{}

func (s *stubFrameData) AsBytes() ([]byte, error)                  { return []byte{0}, nil }
func (s *stubFrameData) AsDMABuf() (*contracts.DMABufHandle, bool) { return nil, false }
func (s *stubFrameData) Release() error                            { return nil }

func makeFrame(seq uint64) contracts.Frame {
	return contracts.Frame{Seq: seq, Timestamp: time.Now(), Width: 1, Height: 1, Data: &stubFrameData{}}
}

type stubVision struct {
	diffResult *contracts.DiffResult
	diffErr    error
}

func (s *stubVision) Analyze(_ context.Context, _ contracts.Frame) (*contracts.Analysis, error) {
	return &contracts.Analysis{}, nil
}
func (s *stubVision) Match(_ context.Context, _ contracts.Frame, _ contracts.Template) ([]contracts.Match, error) {
	return nil, nil
}
func (s *stubVision) Diff(_ context.Context, _, _ contracts.Frame) (*contracts.DiffResult, error) {
	return s.diffResult, s.diffErr
}
func (s *stubVision) OCR(_ context.Context, _ contracts.Frame, _ contracts.Rect) (contracts.OCRResult, error) {
	return contracts.OCRResult{}, nil
}

// alwaysVerifier is a trivial Verifier that returns a fixed result.
type alwaysVerifier struct {
	result bool
	err    error
}

func (a *alwaysVerifier) Verify(_ context.Context, _, _ contracts.Frame, _ string) (bool, error) {
	return a.result, a.err
}

// ── PixelVerifier tests ─────────────────────────────────────────────────────

func TestPixelVerifier_AboveThreshold_ReturnsTrue(t *testing.T) {
	vis := &stubVision{diffResult: &contracts.DiffResult{TotalDelta: 10.0, SameShape: true}}
	pv := &PixelVerifier{Vision: vis, Threshold: 5.0}
	ok, err := pv.Verify(context.Background(), makeFrame(0), makeFrame(1), "")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestPixelVerifier_EqualThreshold_ReturnsTrue(t *testing.T) {
	vis := &stubVision{diffResult: &contracts.DiffResult{TotalDelta: 5.0}}
	pv := &PixelVerifier{Vision: vis, Threshold: 5.0}
	ok, err := pv.Verify(context.Background(), makeFrame(0), makeFrame(1), "")
	require.NoError(t, err)
	assert.True(t, ok, "delta == threshold should pass")
}

func TestPixelVerifier_BelowThreshold_ReturnsFalse(t *testing.T) {
	vis := &stubVision{diffResult: &contracts.DiffResult{TotalDelta: 1.0}}
	pv := &PixelVerifier{Vision: vis, Threshold: 5.0}
	ok, err := pv.Verify(context.Background(), makeFrame(0), makeFrame(1), "")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestPixelVerifier_ZeroThreshold_AlwaysTrue(t *testing.T) {
	vis := &stubVision{diffResult: &contracts.DiffResult{TotalDelta: 0.0}}
	pv := &PixelVerifier{Vision: vis, Threshold: 0}
	ok, err := pv.Verify(context.Background(), makeFrame(0), makeFrame(1), "")
	require.NoError(t, err)
	assert.True(t, ok, "zero threshold: delta 0.0 >= 0.0 should pass")
}

func TestPixelVerifier_DiffError_Propagates(t *testing.T) {
	vis := &stubVision{diffErr: errors.New("diff backend unavailable")}
	pv := &PixelVerifier{Vision: vis, Threshold: 1.0}
	ok, err := pv.Verify(context.Background(), makeFrame(0), makeFrame(1), "")
	require.Error(t, err)
	assert.False(t, ok)
	assert.Contains(t, err.Error(), "diff backend unavailable")
}

func TestPixelVerifier_NilVision_ReturnsError(t *testing.T) {
	pv := &PixelVerifier{Vision: nil, Threshold: 1.0}
	ok, err := pv.Verify(context.Background(), makeFrame(0), makeFrame(1), "")
	require.Error(t, err)
	assert.False(t, ok)
	assert.Contains(t, err.Error(), "nil VisionPipeline")
}

func TestPixelVerifier_NilDiffResult_ReturnsError(t *testing.T) {
	vis := &stubVision{diffResult: nil, diffErr: nil}
	pv := &PixelVerifier{Vision: vis, Threshold: 1.0}
	ok, err := pv.Verify(context.Background(), makeFrame(0), makeFrame(1), "")
	require.Error(t, err)
	assert.False(t, ok)
	assert.Contains(t, err.Error(), "nil result")
}

// ── MultiVerifier tests ─────────────────────────────────────────────────────

func TestMultiVerifier_AllPass_ReturnsTrue(t *testing.T) {
	mv := &MultiVerifier{Inner: []Verifier{
		&alwaysVerifier{result: true},
		&alwaysVerifier{result: true},
		&alwaysVerifier{result: true},
	}}
	ok, err := mv.Verify(context.Background(), makeFrame(0), makeFrame(1), "")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestMultiVerifier_FirstFails_ReturnsFalse(t *testing.T) {
	called := 0
	counter := &alwaysVerifier{result: true}
	_ = counter // ensure second is not called
	mv := &MultiVerifier{Inner: []Verifier{
		&alwaysVerifier{result: false},
		// second should not be reached
		&alwaysVerifier{result: true},
	}}
	_ = called
	ok, err := mv.Verify(context.Background(), makeFrame(0), makeFrame(1), "")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestMultiVerifier_MiddleFails_ReturnsFalse(t *testing.T) {
	mv := &MultiVerifier{Inner: []Verifier{
		&alwaysVerifier{result: true},
		&alwaysVerifier{result: false},
		&alwaysVerifier{result: true},
	}}
	ok, err := mv.Verify(context.Background(), makeFrame(0), makeFrame(1), "")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestMultiVerifier_ErrorPropagates(t *testing.T) {
	wantErr := errors.New("inner verifier exploded")
	mv := &MultiVerifier{Inner: []Verifier{
		&alwaysVerifier{result: true},
		&alwaysVerifier{err: wantErr},
		&alwaysVerifier{result: true},
	}}
	ok, err := mv.Verify(context.Background(), makeFrame(0), makeFrame(1), "")
	require.Error(t, err)
	assert.False(t, ok)
	assert.ErrorIs(t, err, wantErr)
}

func TestMultiVerifier_Empty_ReturnsTrue(t *testing.T) {
	mv := &MultiVerifier{Inner: nil}
	ok, err := mv.Verify(context.Background(), makeFrame(0), makeFrame(1), "")
	require.NoError(t, err)
	assert.True(t, ok, "empty MultiVerifier: vacuous truth")
}

func TestMultiVerifier_InterfaceSatisfied(t *testing.T) {
	var _ Verifier = (*MultiVerifier)(nil)
	var _ Verifier = (*PixelVerifier)(nil)
}
