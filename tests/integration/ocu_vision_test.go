//go:build integration
// +build integration

// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package integration_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
	"digital.vasic.helixqa/pkg/nexus/vision"
	"digital.vasic.helixqa/pkg/nexus/vision/cpu"
)

type noRemoteDispatcher struct{}

func (noRemoteDispatcher) Resolve(context.Context, contracts.Capability) (contracts.Worker, error) {
	return nil, context.Canceled
}

func TestOCU_Vision_CPUFallback_AllFourMethods(t *testing.T) {
	p := vision.NewPipeline(noRemoteDispatcher{}, cpu.New())
	frame := contracts.Frame{Width: 800, Height: 600, Format: contracts.PixelFormatBGRA8}

	a, err := p.Analyze(context.Background(), frame)
	require.NoError(t, err)
	require.Equal(t, "local-cpu", a.DispatchedTo)

	_, err = p.Match(context.Background(), frame, contracts.Template{Name: "t", Bytes: []byte{0x00}})
	require.NoError(t, err)

	d, err := p.Diff(context.Background(), frame, frame)
	require.NoError(t, err)
	require.True(t, d.SameShape)

	_, err = p.OCR(context.Background(), frame, contracts.Rect{W: 100, H: 100})
	require.NoError(t, err)
}
