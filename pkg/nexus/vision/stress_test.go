// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package vision

import (
	"context"
	"errors"
	"sync"
	"testing"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

func TestStress_Pipeline_100Concurrent(t *testing.T) {
	d := &fakeDispatcher{resolveErr: errors.New("no host")}
	local := &fakeLocal{}
	p := NewPipeline(d, local)
	frame := contracts.Frame{Width: 800, Height: 600, Format: contracts.PixelFormatBGRA8}
	const N = 100
	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := p.Analyze(context.Background(), frame)
			if err != nil {
				t.Errorf("analyze: %v", err)
			}
		}()
	}
	wg.Wait()
}
