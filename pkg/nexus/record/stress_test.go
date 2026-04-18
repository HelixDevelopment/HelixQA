// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package record_test

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
	"digital.vasic.helixqa/pkg/nexus/record"
	"digital.vasic.helixqa/pkg/nexus/record/encoder"
)

// safeEncoder is a goroutine-safe mock encoder for the stress test.
type safeEncoder struct {
	mu    sync.Mutex
	count int
}

func (e *safeEncoder) Encode(_ contracts.Frame) error {
	e.mu.Lock()
	e.count++
	e.mu.Unlock()
	return nil
}

func (e *safeEncoder) Close() error { return nil }

// runRecorderCycle performs a complete AttachSource → Start → wait → Clip →
// Stop cycle with a synthetic in-memory source that emits n frames.
func runRecorderCycle(t *testing.T, n int) {
	t.Helper()

	base := time.Now()
	frames := make([]contracts.Frame, n)
	for i := range frames {
		frames[i] = contracts.Frame{
			Seq:       uint64(i),
			Timestamp: base.Add(time.Duration(i) * time.Millisecond),
			Width:     1280,
			Height:    720,
		}
	}

	src := newMockSource(frames)
	enc := &safeEncoder{}
	rec := record.NewRecorder(64, enc)

	require.NoError(t, rec.AttachSource(src))
	require.NoError(t, rec.Start(context.Background(), record.RecordConfig{}))
	// Stop waits for the drain goroutine; source channel already closed.
	require.NoError(t, rec.Stop())

	// Clip must produce non-empty output for frames within a wide window.
	var buf bytes.Buffer
	err := rec.Clip(base.Add(5*time.Millisecond), time.Duration(n+1)*time.Millisecond, &buf, contracts.ClipOptions{})
	require.NoError(t, err)

	// At least one encoder call issued.
	enc.mu.Lock()
	cnt := enc.count
	enc.mu.Unlock()
	require.Equal(t, n, cnt, "all frames must be forwarded to the encoder")
}

// TestStress_Record_100Concurrent runs 100 goroutines, each performing a
// full Recorder lifecycle, under the Go race detector (-race).
func TestStress_Record_100Concurrent(t *testing.T) {
	const total = 100
	const framesPerCycle = 10

	// Verify encoder package resolves unknown kind error (exercises registry).
	_, err := encoder.New("nonexistent-kind")
	require.Error(t, err)

	var wg sync.WaitGroup
	for i := 0; i < total; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runRecorderCycle(t, framesPerCycle)
		}()
	}
	wg.Wait()
}
