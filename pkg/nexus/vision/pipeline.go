// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package vision hosts the OCU P2 GPU vision pipeline. It
// implements contracts.VisionPipeline by first trying to Resolve a
// remote CUDA worker via ocuremote.Dispatcher and falling back to
// a local CPU backend when no GPU host is available. Real OpenCV
// CUDA bindings + TensorRT OCR arrive in P2.5.
package vision

import (
	"context"
	"errors"
	"time"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// ErrNoBackend is returned when the pipeline has no remote worker
// and no local backend.
var ErrNoBackend = errors.New("vision: no remote worker and no local backend configured")

// LocalBackend is the CPU (or OpenCL, eventually) fallback that
// the pipeline delegates to when no remote GPU worker is available.
type LocalBackend interface {
	Analyze(ctx context.Context, frame contracts.Frame) (*contracts.Analysis, error)
	Match(ctx context.Context, frame contracts.Frame, tmpl contracts.Template) ([]contracts.Match, error)
	Diff(ctx context.Context, before, after contracts.Frame) (*contracts.DiffResult, error)
	OCR(ctx context.Context, frame contracts.Frame, region contracts.Rect) (contracts.OCRResult, error)
}

// Pipeline implements contracts.VisionPipeline. It prefers a
// remote CUDA worker (via the Dispatcher) and falls back to the
// LocalBackend.
type Pipeline struct {
	dispatcher contracts.Dispatcher
	local      LocalBackend
}

// NewPipeline wires a dispatcher and a local fallback.
func NewPipeline(d contracts.Dispatcher, local LocalBackend) *Pipeline {
	return &Pipeline{dispatcher: d, local: local}
}

// Analyze implements contracts.VisionPipeline.
func (p *Pipeline) Analyze(ctx context.Context, frame contracts.Frame) (*contracts.Analysis, error) {
	if worker, err := p.resolve(ctx, contracts.KindCUDAOpenCV); err == nil && worker != nil {
		defer worker.Close()
		return p.analyzeRemote(ctx, frame)
	}
	if p.local == nil {
		return nil, ErrNoBackend
	}
	return p.local.Analyze(ctx, frame)
}

// Match implements contracts.VisionPipeline.
func (p *Pipeline) Match(ctx context.Context, frame contracts.Frame, tmpl contracts.Template) ([]contracts.Match, error) {
	if worker, err := p.resolve(ctx, contracts.KindCUDAOpenCV); err == nil && worker != nil {
		defer worker.Close()
		// P2 stub: remote path returns empty matches. Real gRPC in P2.5.
		return nil, nil
	}
	if p.local == nil {
		return nil, ErrNoBackend
	}
	return p.local.Match(ctx, frame, tmpl)
}

// Diff implements contracts.VisionPipeline.
func (p *Pipeline) Diff(ctx context.Context, before, after contracts.Frame) (*contracts.DiffResult, error) {
	if worker, err := p.resolve(ctx, contracts.KindCUDAOpenCV); err == nil && worker != nil {
		defer worker.Close()
		return &contracts.DiffResult{}, nil
	}
	if p.local == nil {
		return nil, ErrNoBackend
	}
	return p.local.Diff(ctx, before, after)
}

// OCR implements contracts.VisionPipeline.
func (p *Pipeline) OCR(ctx context.Context, frame contracts.Frame, region contracts.Rect) (contracts.OCRResult, error) {
	if worker, err := p.resolve(ctx, contracts.KindTensorRTOCR); err == nil && worker != nil {
		defer worker.Close()
		return contracts.OCRResult{}, nil
	}
	if p.local == nil {
		return contracts.OCRResult{}, ErrNoBackend
	}
	return p.local.OCR(ctx, frame, region)
}

func (p *Pipeline) resolve(ctx context.Context, kind contracts.CapabilityKind) (contracts.Worker, error) {
	if p.dispatcher == nil {
		return nil, errors.New("vision: no dispatcher configured")
	}
	return p.dispatcher.Resolve(ctx, contracts.Capability{Kind: kind, MinVRAM: 2048})
}

func (p *Pipeline) analyzeRemote(ctx context.Context, frame contracts.Frame) (*contracts.Analysis, error) {
	start := time.Now()
	// P2 stub: real gRPC call into the CUDA sidecar arrives in P2.5.
	_ = frame
	return &contracts.Analysis{
		DispatchedTo: "thinker-cuda",
		LatencyMs:    int(time.Since(start).Milliseconds()),
	}, nil
}

// Compile-time interface satisfaction check.
var _ contracts.VisionPipeline = (*Pipeline)(nil)
