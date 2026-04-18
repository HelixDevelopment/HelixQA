// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package cpu is the OCU P2 minimal CPU fallback for the vision
// Pipeline. It accepts BGRA8 frames and produces empty-but-valid
// results — real CV computation (edges, templates, OCR) will be
// added in P2.5 when OpenCV is wired in.
package cpu

import (
	"context"
	"fmt"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// Backend is the CPU-only vision.LocalBackend.
type Backend struct{}

// New returns a new CPU backend.
func New() *Backend { return &Backend{} }

// Analyze implements vision.LocalBackend.
func (b *Backend) Analyze(_ context.Context, frame contracts.Frame) (*contracts.Analysis, error) {
	if err := requireBGRA(frame); err != nil {
		return nil, err
	}
	return &contracts.Analysis{
		DispatchedTo: "local-cpu",
	}, nil
}

// Match implements vision.LocalBackend.
func (b *Backend) Match(_ context.Context, frame contracts.Frame, _ contracts.Template) ([]contracts.Match, error) {
	if err := requireBGRA(frame); err != nil {
		return nil, err
	}
	return nil, nil
}

// Diff implements vision.LocalBackend.
func (b *Backend) Diff(_ context.Context, before, after contracts.Frame) (*contracts.DiffResult, error) {
	if err := requireBGRA(before); err != nil {
		return nil, err
	}
	if err := requireBGRA(after); err != nil {
		return nil, err
	}
	same := before.Width == after.Width && before.Height == after.Height
	return &contracts.DiffResult{SameShape: same}, nil
}

// OCR implements vision.LocalBackend.
func (b *Backend) OCR(_ context.Context, frame contracts.Frame, _ contracts.Rect) (contracts.OCRResult, error) {
	if err := requireBGRA(frame); err != nil {
		return contracts.OCRResult{}, err
	}
	return contracts.OCRResult{}, nil
}

func requireBGRA(f contracts.Frame) error {
	if f.Format != contracts.PixelFormatBGRA8 {
		return fmt.Errorf("cpu backend: unsupported pixel format %q (want BGRA8)", f.Format)
	}
	return nil
}
