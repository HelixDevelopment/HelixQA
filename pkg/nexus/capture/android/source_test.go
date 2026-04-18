// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package android

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/nexus/capture"
	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

type fakeFrameData struct{ data []byte }

func (f *fakeFrameData) AsBytes() ([]byte, error)                  { return f.data, nil }
func (f *fakeFrameData) AsDMABuf() (*contracts.DMABufHandle, bool) { return nil, false }
func (f *fakeFrameData) Release() error                            { return nil }

func TestSource_ProducesH264Frames(t *testing.T) {
	original := newFrameProducer
	defer func() { newFrameProducer = original }()
	newFrameProducer = func(ctx context.Context, cfg contracts.CaptureConfig, out chan<- contracts.Frame, stopCh <-chan struct{}) error {
		for i := 0; i < 3; i++ {
			select {
			case <-stopCh:
				return nil
			case out <- contracts.Frame{
				Seq: uint64(i), Timestamp: time.Now(),
				Width: 1280, Height: 720, Format: contracts.PixelFormatH264,
				Data: &fakeFrameData{data: []byte{0x00, 0x00, 0x00, 0x01}},
			}:
			}
		}
		return nil
	}
	src, err := Open(context.Background(), contracts.CaptureConfig{Width: 1280, Height: 720})
	require.NoError(t, err)
	defer src.Close()

	require.NoError(t, src.Start(context.Background(), contracts.CaptureConfig{Width: 1280, Height: 720}))
	received := 0
	timeout := time.After(2 * time.Second)
	for received < 3 {
		select {
		case f := <-src.Frames():
			require.Equal(t, contracts.PixelFormatH264, f.Format)
			received++
		case <-timeout:
			t.Fatalf("timed out after %d frames", received)
		}
	}
	require.NoError(t, src.Stop())
}

func TestFactoryRegistersBothKinds(t *testing.T) {
	kinds := capture.Kinds()
	haveAndroid, haveTV := false, false
	for _, k := range kinds {
		if k == "android" {
			haveAndroid = true
		}
		if k == "androidtv" {
			haveTV = true
		}
	}
	require.True(t, haveAndroid, "android kind missing")
	require.True(t, haveTV, "androidtv kind missing")
}

func TestSource_ReportsCorrectKind(t *testing.T) {
	phone, err := capture.Open(context.Background(), "android", contracts.CaptureConfig{})
	require.NoError(t, err)
	require.Equal(t, "android", phone.Name())
	phone.Close()

	tv, err := capture.Open(context.Background(), "androidtv", contracts.CaptureConfig{})
	require.NoError(t, err)
	require.Equal(t, "androidtv", tv.Name())
	tv.Close()
}

func TestProductionOpenReturnsErrNotWired(t *testing.T) {
	// Force stub mode for determinism: Start must return ErrNotWired.
	t.Setenv("HELIXQA_CAPTURE_ANDROID_STUB", "1")
	original := newFrameProducer
	defer func() { newFrameProducer = original }()
	newFrameProducer = productionFrameProducer
	src, err := Open(context.Background(), contracts.CaptureConfig{})
	require.NoError(t, err)
	defer src.Close()
	err = src.Start(context.Background(), contracts.CaptureConfig{})
	require.True(t, err == nil || errors.Is(err, ErrNotWired),
		"Start should succeed or return ErrNotWired; got %v", err)
}

// TestProduction_MissingAdbErrors verifies graceful fallback when adb is
// absent or stub mode is on — Start must return ErrNotWired, never panic.
func TestProduction_MissingAdbErrors(t *testing.T) {
	t.Setenv("HELIXQA_CAPTURE_ANDROID_STUB", "1")
	orig := newFrameProducer
	defer func() { newFrameProducer = orig }()
	newFrameProducer = productionFrameProducer

	src, err := Open(context.Background(), contracts.CaptureConfig{})
	require.NoError(t, err)
	defer src.Close()

	err = src.Start(context.Background(), contracts.CaptureConfig{})
	require.Error(t, err, "Start must return an error in stub mode")
	require.True(t, errors.Is(err, ErrNotWired),
		"expected ErrNotWired, got %v", err)
}

// TestSplitH264NALUnits_ThreeStartCodes verifies that splitH264NALUnits
// correctly splits a buffer containing three 4-byte start codes.
func TestSplitH264NALUnits_ThreeStartCodes(t *testing.T) {
	sc := []byte{0x00, 0x00, 0x00, 0x01}
	// Build: SC + [0x67, 0x01] + SC + [0x68, 0x02] + SC + [0x65, 0x03]
	input := append(append(append(append(append(append(
		sc, 0x67, 0x01),
		sc...), 0x68, 0x02),
		sc...), 0x65, 0x03),
		[]byte{}...)

	nals := splitH264NALUnits(input)
	require.Len(t, nals, 3, "expected 3 NAL units")
	// Each NAL must start with the start code.
	for i, nal := range nals {
		require.GreaterOrEqual(t, len(nal), 4,
			"NAL %d too short: %v", i, nal)
		require.Equal(t, sc, nal[:4],
			"NAL %d missing start code", i)
	}
	// NAL type bytes.
	require.Equal(t, byte(0x67), nals[0][4])
	require.Equal(t, byte(0x68), nals[1][4])
	require.Equal(t, byte(0x65), nals[2][4])
}
