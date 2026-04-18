// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"context"
	"testing"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// BenchmarkWrap_Click measures the overhead of Wrap() relative to a
// direct fakeInteractor.Click() call. P3 target: < 1 µs per op.
func BenchmarkWrap_Click(b *testing.B) {
	inner := &fakeInteractor{}
	w := Wrap(inner, NoOp{})
	ctx := context.Background()
	pt := contracts.Point{X: 100, Y: 200}
	opts := contracts.ClickOptions{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = w.Click(ctx, pt, opts)
	}
}

// BenchmarkNoOp_After measures the cost of the no-op verifier alone.
func BenchmarkNoOp_After(b *testing.B) {
	var v NoOp
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = v.After(ctx, "click")
	}
}

// BenchmarkWrap_AllMethods measures a full five-method sequence through Wrap.
func BenchmarkWrap_AllMethods(b *testing.B) {
	inner := &fakeInteractor{}
	w := Wrap(inner, NoOp{})
	ctx := context.Background()
	pt := contracts.Point{X: 50, Y: 50}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = w.Click(ctx, pt, contracts.ClickOptions{})
		_ = w.Type(ctx, "bench", contracts.TypeOptions{})
		_ = w.Scroll(ctx, pt, 0, 1)
		_ = w.Key(ctx, contracts.KeyEnter, contracts.KeyOptions{})
		_ = w.Drag(ctx, pt, contracts.Point{X: 200, Y: 200}, contracts.DragOptions{})
	}
}
