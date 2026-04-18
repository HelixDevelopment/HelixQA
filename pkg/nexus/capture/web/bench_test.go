// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package web

import (
	"context"
	"testing"
	"time"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

func BenchmarkSource_FrameChannelThroughput(b *testing.B) {
	original := newFrameProducer
	defer func() { newFrameProducer = original }()
	newFrameProducer = func(ctx context.Context, cfg contracts.CaptureConfig, out chan<- contracts.Frame, stopCh <-chan struct{}) error {
		for {
			select {
			case <-stopCh:
				return nil
			case out <- contracts.Frame{Timestamp: time.Now(), Width: 800, Height: 600}:
			}
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		src, err := Open(context.Background(), contracts.CaptureConfig{Width: 800, Height: 600})
		if err != nil {
			b.Fatal(err)
		}
		if err := src.Start(context.Background(), contracts.CaptureConfig{Width: 800, Height: 600}); err != nil {
			src.Close()
			b.Fatal(err)
		}
		<-src.Frames()
		src.Stop()
		src.Close()
	}
}
