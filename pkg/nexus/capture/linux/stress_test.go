// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package linux

import (
	"context"
	"sync"
	"testing"
	"time"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

func TestStress_Source_100Clients(t *testing.T) {
	original := newFrameProducer
	defer func() { newFrameProducer = original }()
	newFrameProducer = func(ctx context.Context, cfg contracts.CaptureConfig, out chan<- contracts.Frame, stopCh <-chan struct{}) error {
		for i := 0; i < 3; i++ {
			select {
			case <-stopCh:
				return nil
			case out <- contracts.Frame{Seq: uint64(i), Timestamp: time.Now()}:
			}
		}
		return nil
	}

	const N = 100
	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			src, err := Open(context.Background(), contracts.CaptureConfig{})
			if err != nil {
				t.Errorf("open: %v", err)
				return
			}
			defer src.Close()
			if err := src.Start(context.Background(), contracts.CaptureConfig{}); err != nil {
				t.Errorf("start: %v", err)
				return
			}
			for j := 0; j < 3; j++ {
				select {
				case <-src.Frames():
				case <-time.After(2 * time.Second):
					t.Errorf("timeout frame %d", j)
					return
				}
			}
			_ = src.Stop()
		}()
	}
	wg.Wait()
}
