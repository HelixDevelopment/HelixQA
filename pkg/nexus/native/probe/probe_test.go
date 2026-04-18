// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package probe

import (
	"context"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProbeLocal_PopulatesHost(t *testing.T) {
	r, err := ProbeLocal(context.Background())
	require.NoError(t, err)
	require.Equal(t, runtime.GOOS, r.OS)
	require.Equal(t, runtime.GOARCH, r.Arch)
	require.Greater(t, r.CPUCores, 0)
	require.Greater(t, r.MemoryTotalMB, uint64(0))
}
