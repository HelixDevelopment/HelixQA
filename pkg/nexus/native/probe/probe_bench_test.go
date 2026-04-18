// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package probe

import (
	"context"
	"testing"
)

func BenchmarkProbeLocal(b *testing.B) {
	ctx := context.Background()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := ProbeLocal(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}
