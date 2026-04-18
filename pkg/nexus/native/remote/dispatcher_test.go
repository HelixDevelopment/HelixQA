// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package remote

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	cremote "digital.vasic.containers/pkg/remote"
	"digital.vasic.containers/pkg/scheduler"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

type fakeHostMgr struct{ hosts map[string]*cremote.HostResources }

func (f *fakeHostMgr) ProbeAll(context.Context) (map[string]*cremote.HostResources, error) {
	return f.hosts, nil
}

func TestDispatcher_Resolve_PrefersLocalWhenNoGPUNeeded(t *testing.T) {
	d := NewDispatcher(&fakeHostMgr{}, scheduler.Options{})
	w, err := d.Resolve(context.Background(), contracts.Capability{
		Kind:        contracts.KindCUDAOpenCV,
		PreferLocal: true,
	})
	require.NoError(t, err)
	require.NotNil(t, w)
	defer w.Close()
}

func TestDispatcher_Resolve_PicksGPUHost(t *testing.T) {
	d := NewDispatcher(&fakeHostMgr{hosts: map[string]*cremote.HostResources{
		"thinker": {
			Host:          "thinker",
			CPUCores:      8,
			MemoryTotalMB: 32_000,
			GPU: []cremote.GPUDevice{
				{Vendor: "nvidia", VRAMFreeMB: 5800, CUDASupported: true},
			},
		},
	}}, scheduler.Options{GPUWeight: 1})
	w, err := d.Resolve(context.Background(), contracts.Capability{
		Kind:    contracts.KindCUDAOpenCV,
		MinVRAM: 2048,
	})
	require.NoError(t, err)
	require.NotNil(t, w)
	defer w.Close()
}

func TestDispatcher_Resolve_NoHostAvailable(t *testing.T) {
	d := NewDispatcher(
		&fakeHostMgr{hosts: map[string]*cremote.HostResources{}},
		scheduler.Options{GPUWeight: 1},
	)
	_, err := d.Resolve(context.Background(), contracts.Capability{
		Kind:    contracts.KindCUDAOpenCV,
		MinVRAM: 2048,
	})
	require.Error(t, err)
}

func TestUnwrap_RemoteWorker(t *testing.T) {
	d := NewDispatcher(&fakeHostMgr{hosts: map[string]*cremote.HostResources{
		"thinker": {
			Host: "thinker", CPUCores: 8, MemoryTotalMB: 32_000,
			GPU: []cremote.GPUDevice{{Vendor: "nvidia", VRAMFreeMB: 5800, CUDASupported: true}},
		},
	}}, scheduler.Options{GPUWeight: 1})
	w, err := d.Resolve(context.Background(), contracts.Capability{
		Kind: contracts.KindCUDAOpenCV, MinVRAM: 1024,
	})
	require.NoError(t, err)
	defer w.Close()
	info, ok := Unwrap(w)
	require.True(t, ok)
	require.Equal(t, "thinker", info.Host())
}

func TestUnwrap_LocalWorker(t *testing.T) {
	d := NewDispatcher(&fakeHostMgr{}, scheduler.Options{})
	w, err := d.Resolve(context.Background(), contracts.Capability{
		Kind: contracts.KindCUDAOpenCV, PreferLocal: true,
	})
	require.NoError(t, err)
	defer w.Close()
	_, ok := Unwrap(w)
	require.False(t, ok)
}
