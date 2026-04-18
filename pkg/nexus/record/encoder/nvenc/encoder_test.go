// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package nvenc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
	"digital.vasic.helixqa/pkg/nexus/record/encoder"
	nvencpkg "digital.vasic.helixqa/pkg/nexus/record/encoder/nvenc"
)

// ---------------------------------------------------------------------------
// Fake Dispatcher and Worker for testing
// ---------------------------------------------------------------------------

type fakeWorker struct {
	calls    int
	callErr  error
	closed   bool
	closeErr error
}

func (w *fakeWorker) Call(_ context.Context, _ proto.Message, _ proto.Message) error {
	w.calls++
	return w.callErr
}

func (w *fakeWorker) Close() error {
	w.closed = true
	return w.closeErr
}

// fakeDispatcher resolves to a pre-configured Worker or returns an error.
type fakeDispatcher struct {
	worker contracts.Worker
	err    error
}

func (d *fakeDispatcher) Resolve(_ context.Context, _ contracts.Capability) (contracts.Worker, error) {
	if d.err != nil {
		return nil, d.err
	}
	return d.worker, nil
}

// ---------------------------------------------------------------------------
// Existing tests (must stay green)
// ---------------------------------------------------------------------------

// TestNVENC_FactoryRegistered verifies the init() registers "nvenc" in the
// parent encoder factory.
func TestNVENC_FactoryRegistered(t *testing.T) {
	kinds := encoder.Kinds()
	assert.Contains(t, kinds, "nvenc", "nvenc must be registered via init()")
}

// TestNVENC_ProductionReturnsErrNotWired verifies the production stub returned
// by the factory (productionEncoder) still returns ErrNotWired from Encode().
func TestNVENC_ProductionReturnsErrNotWired(t *testing.T) {
	enc, err := encoder.New("nvenc")
	require.NoError(t, err)
	require.NotNil(t, enc)

	err = enc.Encode(contracts.Frame{Seq: 0})
	require.ErrorIs(t, err, encoder.ErrNotWired)
}

// TestNVENC_Close_AlwaysSucceeds verifies Close() never errors on the factory stub.
func TestNVENC_Close_AlwaysSucceeds(t *testing.T) {
	enc, err := encoder.New("nvenc")
	require.NoError(t, err)
	require.NoError(t, enc.Close())
}

// ---------------------------------------------------------------------------
// P5.5 new tests
// ---------------------------------------------------------------------------

// TestStubEnv_ForcesErrNotWired — HELIXQA_RECORD_NVENC_STUB=1 must return
// ErrNotWired from NewProductionEncoder without consulting the Dispatcher.
func TestStubEnv_ForcesErrNotWired(t *testing.T) {
	t.Setenv("HELIXQA_RECORD_NVENC_STUB", "1")

	disp := &fakeDispatcher{worker: &fakeWorker{}}
	_, err := nvencpkg.NewProductionEncoder(nvencpkg.RecordConfig{}, disp)
	require.ErrorIs(t, err, encoder.ErrNotWired)
}

// TestNilDispatcher_ReturnsErrNotWired — a nil Dispatcher must be rejected
// at construction time.
func TestNilDispatcher_ReturnsErrNotWired(t *testing.T) {
	t.Setenv("HELIXQA_RECORD_NVENC_STUB", "")

	_, err := nvencpkg.NewProductionEncoder(nvencpkg.RecordConfig{}, nil)
	require.ErrorIs(t, err, encoder.ErrNotWired)
}

// TestHappyPath_FakeWorker_AcceptsNEncodes — a fake Dispatcher resolving to a
// fake Worker must accept N consecutive Encode calls and then Close cleanly.
func TestHappyPath_FakeWorker_AcceptsNEncodes(t *testing.T) {
	t.Setenv("HELIXQA_RECORD_NVENC_STUB", "")

	worker := &fakeWorker{}
	disp := &fakeDispatcher{worker: worker}

	enc, err := nvencpkg.NewProductionEncoder(nvencpkg.RecordConfig{Width: 1920, Height: 1080, FrameRate: 30}, disp)
	require.NoError(t, err)
	require.NotNil(t, enc)

	const n = 5
	for i := range n {
		require.NoError(t, enc.Encode(contracts.Frame{Seq: uint64(i)}))
	}
	require.Equal(t, n, worker.calls, "worker must be called once per Encode")

	require.NoError(t, enc.Close())
	require.True(t, worker.closed, "worker must be closed")
}

// TestResolveError_ReturnsErrNotWired — when Dispatcher.Resolve returns an
// error (no GPU host satisfies KindNVENC) Encode must return ErrNotWired.
func TestResolveError_ReturnsErrNotWired(t *testing.T) {
	t.Setenv("HELIXQA_RECORD_NVENC_STUB", "")

	disp := &fakeDispatcher{err: errors.New("no GPU host")}

	enc, err := nvencpkg.NewProductionEncoder(nvencpkg.RecordConfig{}, disp)
	require.NoError(t, err, "construction must succeed even when sidecar is absent")

	err = enc.Encode(contracts.Frame{Seq: 0})
	require.ErrorIs(t, err, encoder.ErrNotWired)
}

// TestWorkerCallError_SurfacedToCaller — when Worker.Call returns an error it
// must be propagated (wrapped) to the Encode caller.
func TestWorkerCallError_SurfacedToCaller(t *testing.T) {
	t.Setenv("HELIXQA_RECORD_NVENC_STUB", "")

	sentinel := errors.New("grpc: transport closed")
	worker := &fakeWorker{callErr: sentinel}
	disp := &fakeDispatcher{worker: worker}

	enc, err := nvencpkg.NewProductionEncoder(nvencpkg.RecordConfig{}, disp)
	require.NoError(t, err)

	err = enc.Encode(contracts.Frame{Seq: 0})
	require.Error(t, err)
	require.ErrorContains(t, err, "nvenc: worker call:")
	require.ErrorIs(t, err, sentinel)
}

// TestClose_IsIdempotent — Close must not panic or error on second call.
func TestClose_IsIdempotent(t *testing.T) {
	t.Setenv("HELIXQA_RECORD_NVENC_STUB", "")

	worker := &fakeWorker{}
	disp := &fakeDispatcher{worker: worker}

	enc, err := nvencpkg.NewProductionEncoder(nvencpkg.RecordConfig{}, disp)
	require.NoError(t, err)

	// Trigger lazy resolve.
	require.NoError(t, enc.Encode(contracts.Frame{Seq: 0}))

	require.NoError(t, enc.Close())
	require.NoError(t, enc.Close(), "second Close must be idempotent")
}
