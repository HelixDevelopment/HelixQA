// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package budget

import (
	"errors"
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

func TestAssertWithin_OK(t *testing.T) {
	err := AssertWithin("capture", 10*time.Millisecond, CaptureLocal)
	require.NoError(t, err)
}

func TestAssertWithin_Exceeds(t *testing.T) {
	err := AssertWithin("capture", 20*time.Millisecond, CaptureLocal)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrBudgetExceeded))
	require.Contains(t, err.Error(), "capture")
	require.Contains(t, err.Error(), "20ms")
	require.Contains(t, err.Error(), "15ms")
}

func TestRecordedMetric_Within(t *testing.T) {
	m := RecordedMetric{Name: "vision", Value: 5 * time.Millisecond, Budget: VisionLocal}
	require.True(t, m.Within())
}

func TestRecordedMetric_Exceeds(t *testing.T) {
	m := RecordedMetric{Name: "vision", Value: 50 * time.Millisecond, Budget: VisionLocal}
	require.False(t, m.Within())
}
