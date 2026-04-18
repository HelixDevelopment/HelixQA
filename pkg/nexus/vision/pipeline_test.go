// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package vision

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

type fakeLocal struct {
	analyze func(context.Context, contracts.Frame) (*contracts.Analysis, error)
	match   func(context.Context, contracts.Frame, contracts.Template) ([]contracts.Match, error)
	diff    func(context.Context, contracts.Frame, contracts.Frame) (*contracts.DiffResult, error)
	ocr     func(context.Context, contracts.Frame, contracts.Rect) (contracts.OCRResult, error)
}

func (f *fakeLocal) Analyze(ctx context.Context, fr contracts.Frame) (*contracts.Analysis, error) {
	if f.analyze != nil {
		return f.analyze(ctx, fr)
	}
	return &contracts.Analysis{DispatchedTo: "local-cpu"}, nil
}
func (f *fakeLocal) Match(ctx context.Context, fr contracts.Frame, t contracts.Template) ([]contracts.Match, error) {
	if f.match != nil {
		return f.match(ctx, fr, t)
	}
	return nil, nil
}
func (f *fakeLocal) Diff(ctx context.Context, a, b contracts.Frame) (*contracts.DiffResult, error) {
	if f.diff != nil {
		return f.diff(ctx, a, b)
	}
	return &contracts.DiffResult{}, nil
}
func (f *fakeLocal) OCR(ctx context.Context, fr contracts.Frame, r contracts.Rect) (contracts.OCRResult, error) {
	if f.ocr != nil {
		return f.ocr(ctx, fr, r)
	}
	return contracts.OCRResult{}, nil
}

type fakeDispatcher struct {
	resolveErr error
	worker     contracts.Worker
}

func (d *fakeDispatcher) Resolve(ctx context.Context, need contracts.Capability) (contracts.Worker, error) {
	if d.resolveErr != nil {
		return nil, d.resolveErr
	}
	if d.worker != nil {
		return d.worker, nil
	}
	return nil, errors.New("fakeDispatcher: no worker")
}

type fakeWorker struct{ host string }

func (w *fakeWorker) Call(context.Context, proto.Message, proto.Message) error { return nil }
func (w *fakeWorker) Close() error                                             { return nil }

func TestPipeline_Analyze_LocalFallback(t *testing.T) {
	d := &fakeDispatcher{resolveErr: errors.New("no host")}
	local := &fakeLocal{}
	p := NewPipeline(d, local)
	frame := contracts.Frame{Width: 800, Height: 600, Format: contracts.PixelFormatBGRA8}
	res, err := p.Analyze(context.Background(), frame)
	require.NoError(t, err)
	require.Equal(t, "local-cpu", res.DispatchedTo)
}

func TestPipeline_Match_LocalFallback(t *testing.T) {
	d := &fakeDispatcher{resolveErr: errors.New("no host")}
	local := &fakeLocal{match: func(ctx context.Context, fr contracts.Frame, tmpl contracts.Template) ([]contracts.Match, error) {
		return []contracts.Match{{Confidence: 0.9}}, nil
	}}
	p := NewPipeline(d, local)
	frame := contracts.Frame{Width: 100, Height: 100, Format: contracts.PixelFormatBGRA8}
	tmpl := contracts.Template{Name: "btn", Bytes: []byte{0x00}}
	matches, err := p.Match(context.Background(), frame, tmpl)
	require.NoError(t, err)
	require.Len(t, matches, 1)
	require.Equal(t, 0.9, matches[0].Confidence)
}

func TestPipeline_Diff_LocalFallback(t *testing.T) {
	d := &fakeDispatcher{resolveErr: errors.New("no host")}
	local := &fakeLocal{}
	p := NewPipeline(d, local)
	a := contracts.Frame{Format: contracts.PixelFormatBGRA8}
	b := contracts.Frame{Format: contracts.PixelFormatBGRA8}
	res, err := p.Diff(context.Background(), a, b)
	require.NoError(t, err)
	require.NotNil(t, res)
}

func TestPipeline_OCR_LocalFallback(t *testing.T) {
	d := &fakeDispatcher{resolveErr: errors.New("no host")}
	local := &fakeLocal{ocr: func(ctx context.Context, fr contracts.Frame, r contracts.Rect) (contracts.OCRResult, error) {
		return contracts.OCRResult{FullText: "hi"}, nil
	}}
	p := NewPipeline(d, local)
	frame := contracts.Frame{}
	rect := contracts.Rect{W: 10, H: 10}
	res, err := p.OCR(context.Background(), frame, rect)
	require.NoError(t, err)
	require.Equal(t, "hi", res.FullText)
}

func TestPipeline_NilLocalBackend_Errors(t *testing.T) {
	d := &fakeDispatcher{resolveErr: errors.New("no host")}
	p := NewPipeline(d, nil)
	_, err := p.Analyze(context.Background(), contracts.Frame{})
	require.Error(t, err)
}

func TestPipeline_Analyze_RemoteDispatch(t *testing.T) {
	d := &fakeDispatcher{worker: &fakeWorker{host: "thinker"}}
	local := &fakeLocal{}
	p := NewPipeline(d, local)
	frame := contracts.Frame{Width: 800, Height: 600, Format: contracts.PixelFormatBGRA8}
	res, err := p.Analyze(context.Background(), frame)
	require.NoError(t, err)
	require.Equal(t, "thinker-cuda", res.DispatchedTo)
}

func TestPipeline_OCR_RemoteDispatch(t *testing.T) {
	d := &fakeDispatcher{worker: &fakeWorker{host: "thinker"}}
	local := &fakeLocal{}
	p := NewPipeline(d, local)
	frame := contracts.Frame{Format: contracts.PixelFormatBGRA8}
	res, err := p.OCR(context.Background(), frame, contracts.Rect{W: 10, H: 10})
	require.NoError(t, err)
	require.Empty(t, res.FullText)
}
