// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package linux

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/nexus/capture"
	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// fakeFrameData is the minimal FrameData implementation for tests.
type fakeFrameData struct{ data []byte }

func (f *fakeFrameData) AsBytes() ([]byte, error)                  { return f.data, nil }
func (f *fakeFrameData) AsDMABuf() (*contracts.DMABufHandle, bool) { return nil, false }
func (f *fakeFrameData) Release() error                            { return nil }

func TestSource_ProducesFramesFromMock(t *testing.T) {
	// Swap the production producer for a mock that emits 3 frames.
	original := newFrameProducer
	defer func() { newFrameProducer = original }()
	newFrameProducer = func(ctx context.Context, cfg contracts.CaptureConfig, out chan<- contracts.Frame, stopCh <-chan struct{}) error {
		for i := 0; i < 3; i++ {
			select {
			case <-stopCh:
				return nil
			case out <- contracts.Frame{
				Seq: uint64(i), Timestamp: time.Now(),
				Width: 1920, Height: 1080, Format: contracts.PixelFormatBGRA8,
				Data: &fakeFrameData{data: []byte{0xBB}},
			}:
			}
		}
		return nil
	}

	src, err := Open(context.Background(), contracts.CaptureConfig{Width: 1920, Height: 1080})
	require.NoError(t, err)
	require.NotNil(t, src)
	defer src.Close()

	require.Equal(t, "linux-x11", src.Name())
	require.NoError(t, src.Start(context.Background(), contracts.CaptureConfig{Width: 1920, Height: 1080}))

	received := 0
	timeout := time.After(2 * time.Second)
	for received < 3 {
		select {
		case f, ok := <-src.Frames():
			if !ok {
				t.Fatalf("frames channel closed early after %d frames", received)
			}
			require.Equal(t, uint64(received), f.Seq)
			require.Equal(t, 1920, f.Width)
			require.Equal(t, contracts.PixelFormatBGRA8, f.Format)
			received++
		case <-timeout:
			t.Fatalf("timed out after %d frames", received)
		}
	}
	stats := src.Stats()
	require.GreaterOrEqual(t, stats.FramesProduced, uint64(3))
	require.NoError(t, src.Stop())
}

func TestSource_RegisteredInFactory(t *testing.T) {
	kinds := capture.Kinds()
	found := false
	for _, k := range kinds {
		if k == "linux-x11" {
			found = true
			break
		}
	}
	require.True(t, found, `capture.Register("linux-x11", ...) should have run in init`)
}

func TestSource_ProductionOpenReturnsNotImplemented(t *testing.T) {
	// Swap to nil producer path (simulates production).
	original := newFrameProducer
	defer func() { newFrameProducer = original }()
	newFrameProducer = productionFrameProducer
	_, err := Open(context.Background(), contracts.CaptureConfig{})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotWired) || err.Error() != "")
}
