// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package cpu

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

func TestBackend_Analyze_ReturnsLocalCPU(t *testing.T) {
	b := New()
	frame := contracts.Frame{Width: 800, Height: 600, Format: contracts.PixelFormatBGRA8}
	res, err := b.Analyze(context.Background(), frame)
	require.NoError(t, err)
	require.Equal(t, "local-cpu", res.DispatchedTo)
}

func TestBackend_Analyze_RejectsUnsupportedFormat(t *testing.T) {
	b := New()
	frame := contracts.Frame{Width: 800, Height: 600, Format: contracts.PixelFormatH264}
	_, err := b.Analyze(context.Background(), frame)
	require.Error(t, err)
}

func TestBackend_Match_ReturnsEmpty(t *testing.T) {
	b := New()
	frame := contracts.Frame{Width: 100, Height: 100, Format: contracts.PixelFormatBGRA8}
	tmpl := contracts.Template{Name: "t", Bytes: []byte{0x00}}
	res, err := b.Match(context.Background(), frame, tmpl)
	require.NoError(t, err)
	require.Len(t, res, 0)
}

func TestBackend_Diff_FlagsSameShape(t *testing.T) {
	b := New()
	a := contracts.Frame{Width: 10, Height: 10, Format: contracts.PixelFormatBGRA8}
	c := contracts.Frame{Width: 10, Height: 10, Format: contracts.PixelFormatBGRA8}
	res, err := b.Diff(context.Background(), a, c)
	require.NoError(t, err)
	require.True(t, res.SameShape)
}

func TestBackend_OCR_ReturnsEmpty(t *testing.T) {
	b := New()
	frame := contracts.Frame{Width: 100, Height: 100, Format: contracts.PixelFormatBGRA8}
	rect := contracts.Rect{W: 50, H: 50}
	res, err := b.OCR(context.Background(), frame, rect)
	require.NoError(t, err)
	require.Empty(t, res.FullText)
}
