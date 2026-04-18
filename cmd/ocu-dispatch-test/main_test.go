// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	cremote "digital.vasic.containers/pkg/remote"
)

type fakeHMForCLI struct{}

func (f *fakeHMForCLI) ProbeAll(context.Context) (map[string]*cremote.HostResources, error) {
	return map[string]*cremote.HostResources{
		"thinker": {
			Host: "thinker", CPUCores: 8, MemoryTotalMB: 32_000,
			GPU: []cremote.GPUDevice{{Vendor: "nvidia", VRAMFreeMB: 5800, CUDASupported: true}},
		},
	}, nil
}

func TestRun_SelectsRemote(t *testing.T) {
	var buf bytes.Buffer
	err := run(context.Background(), &buf, &fakeHMForCLI{})
	require.NoError(t, err)
	require.True(t, strings.Contains(buf.String(), "thinker"))
}
