// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package cpu

import (
	"context"
	"testing"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

func BenchmarkBackend_Analyze(b *testing.B) {
	backend := New()
	frame := contracts.Frame{Width: 1920, Height: 1080, Format: contracts.PixelFormatBGRA8}
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = backend.Analyze(ctx, frame)
	}
}

func BenchmarkBackend_Diff(b *testing.B) {
	backend := New()
	a := contracts.Frame{Width: 1920, Height: 1080, Format: contracts.PixelFormatBGRA8}
	c := contracts.Frame{Width: 1920, Height: 1080, Format: contracts.PixelFormatBGRA8}
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = backend.Diff(ctx, a, c)
	}
}
