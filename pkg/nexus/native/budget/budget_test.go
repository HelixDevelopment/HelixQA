// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package budget

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBudgets_NonZero(t *testing.T) {
	require.Equal(t, 15*time.Millisecond, CaptureLocal)
	require.Equal(t, 8*time.Millisecond, CaptureRemote)
	require.Equal(t, 25*time.Millisecond, VisionLocal)
	require.Equal(t, 8*time.Millisecond, VisionRemoteCompute)
	require.Equal(t, 3*time.Millisecond, VisionRemoteRTT)
	require.Equal(t, 20*time.Millisecond, InteractVerified)
	require.Equal(t, 200*time.Millisecond, ClipExtract)
	require.Equal(t, 100*time.Millisecond, ActionCycleP50)
	require.Equal(t, 200*time.Millisecond, ActionCycleP95)
}

func TestBudgets_MemoryCeilings(t *testing.T) {
	require.Equal(t, uint64(1_500), MaxHostRSSMB)
	require.Equal(t, uint64(4_096), MaxSidecarRSSMB)
	require.Equal(t, uint64(4_096), MaxSidecarVRAMMB)
}
