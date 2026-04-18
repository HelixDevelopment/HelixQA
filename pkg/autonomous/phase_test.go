// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package autonomous

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPhaseManager(t *testing.T) {
	pm := NewPhaseManager()
	assert.NotNil(t, pm)

	phases := pm.All()
	assert.Len(t, phases, 4)
	assert.Equal(t, "setup", phases[0].Name)
	assert.Equal(t, "doc-driven", phases[1].Name)
	assert.Equal(t, "curiosity", phases[2].Name)
	assert.Equal(t, "report", phases[3].Name)

	for _, p := range phases {
		assert.Equal(t, PhasePending, p.Status)
	}
}

func TestPhaseManager_Start(t *testing.T) {
	pm := NewPhaseManager()
	err := pm.Start("setup")
	require.NoError(t, err)

	current := pm.Current()
	assert.Equal(t, "setup", current.Name)
	assert.Equal(t, PhaseRunning, current.Status)
	assert.False(t, current.StartAt.IsZero())
}

func TestPhaseManager_Start_NotFound(t *testing.T) {
	pm := NewPhaseManager()
	err := pm.Start("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestPhaseManager_Start_AlreadyRunning(t *testing.T) {
	pm := NewPhaseManager()
	require.NoError(t, pm.Start("setup"))
	err := pm.Start("setup")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "running")
}

func TestPhaseManager_Complete(t *testing.T) {
	pm := NewPhaseManager()
	require.NoError(t, pm.Start("setup"))
	err := pm.Complete("setup")
	require.NoError(t, err)

	phases := pm.All()
	assert.Equal(t, PhaseCompleted, phases[0].Status)
	assert.False(t, phases[0].EndAt.IsZero())
	assert.Equal(t, 1.0, phases[0].Progress)
}

func TestPhaseManager_Complete_NotRunning(t *testing.T) {
	pm := NewPhaseManager()
	err := pm.Complete("setup")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pending")
}

func TestPhaseManager_Fail(t *testing.T) {
	pm := NewPhaseManager()
	require.NoError(t, pm.Start("setup"))
	err := pm.Fail("setup", fmt.Errorf("init error"))
	require.NoError(t, err)

	phases := pm.All()
	assert.Equal(t, PhaseFailed, phases[0].Status)
	assert.NotNil(t, phases[0].Error)
}

func TestPhaseManager_Fail_NotRunning(t *testing.T) {
	pm := NewPhaseManager()
	err := pm.Fail("setup", fmt.Errorf("oops"))
	assert.Error(t, err)
}

func TestPhaseManager_Skip(t *testing.T) {
	pm := NewPhaseManager()
	err := pm.Skip("curiosity")
	require.NoError(t, err)

	phases := pm.All()
	assert.Equal(t, PhaseSkipped, phases[2].Status)
}

func TestPhaseManager_Skip_NotPending(t *testing.T) {
	pm := NewPhaseManager()
	require.NoError(t, pm.Start("setup"))
	err := pm.Skip("setup")
	assert.Error(t, err)
}

func TestPhaseManager_Current_NoPhaseRunning(t *testing.T) {
	pm := NewPhaseManager()
	current := pm.Current()
	assert.Equal(t, "", current.Name)
}

func TestPhaseManager_All_ReturnsCopy(t *testing.T) {
	pm := NewPhaseManager()
	phases := pm.All()
	phases[0].Name = "modified"

	original := pm.All()
	assert.Equal(t, "setup", original[0].Name)
}

func TestPhaseManager_UpdateProgress(t *testing.T) {
	pm := NewPhaseManager()
	require.NoError(t, pm.Start("setup"))
	err := pm.UpdateProgress("setup", 0.5)
	require.NoError(t, err)

	current := pm.Current()
	assert.Equal(t, 0.5, current.Progress)
}

func TestPhaseManager_UpdateProgress_Clamped(t *testing.T) {
	pm := NewPhaseManager()
	require.NoError(t, pm.Start("setup"))

	err := pm.UpdateProgress("setup", -0.5)
	require.NoError(t, err)
	assert.Equal(t, 0.0, pm.Current().Progress)

	err = pm.UpdateProgress("setup", 1.5)
	require.NoError(t, err)
	assert.Equal(t, 1.0, pm.Current().Progress)
}

func TestPhaseManager_UpdateProgress_NotFound(t *testing.T) {
	pm := NewPhaseManager()
	err := pm.UpdateProgress("nonexistent", 0.5)
	assert.Error(t, err)
}

func TestPhaseManager_FullLifecycle(t *testing.T) {
	pm := NewPhaseManager()

	// Setup phase.
	require.NoError(t, pm.Start("setup"))
	require.NoError(t, pm.Complete("setup"))

	// Doc-driven phase.
	require.NoError(t, pm.Start("doc-driven"))
	require.NoError(t, pm.Complete("doc-driven"))

	// Skip curiosity.
	require.NoError(t, pm.Skip("curiosity"))

	// Report phase.
	require.NoError(t, pm.Start("report"))
	require.NoError(t, pm.Complete("report"))

	phases := pm.All()
	assert.Equal(t, PhaseCompleted, phases[0].Status)
	assert.Equal(t, PhaseCompleted, phases[1].Status)
	assert.Equal(t, PhaseSkipped, phases[2].Status)
	assert.Equal(t, PhaseCompleted, phases[3].Status)
}

func TestPhase_Duration(t *testing.T) {
	p := Phase{Name: "test"}
	assert.Equal(t, 0, int(p.Duration()))
}

func TestPhaseStatus_Constants(t *testing.T) {
	assert.Equal(t, PhaseStatus("pending"), PhasePending)
	assert.Equal(t, PhaseStatus("running"), PhaseRunning)
	assert.Equal(t, PhaseStatus("completed"), PhaseCompleted)
	assert.Equal(t, PhaseStatus("failed"), PhaseFailed)
	assert.Equal(t, PhaseStatus("skipped"), PhaseSkipped)
}

// mockPhaseListener records phase events.
type mockPhaseListener struct {
	mu        sync.Mutex
	starts    []Phase
	completes []Phase
	errors    []Phase
}

func (m *mockPhaseListener) OnPhaseStart(phase Phase) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.starts = append(m.starts, phase)
}

func (m *mockPhaseListener) OnPhaseComplete(phase Phase) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.completes = append(m.completes, phase)
}

func (m *mockPhaseListener) OnPhaseError(phase Phase, _ error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors = append(m.errors, phase)
}

func TestPhaseManager_Listener_Start(t *testing.T) {
	pm := NewPhaseManager()
	listener := &mockPhaseListener{}
	pm.AddListener(listener)

	require.NoError(t, pm.Start("setup"))

	assert.Len(t, listener.starts, 1)
	assert.Equal(t, "setup", listener.starts[0].Name)
}

func TestPhaseManager_Listener_Complete(t *testing.T) {
	pm := NewPhaseManager()
	listener := &mockPhaseListener{}
	pm.AddListener(listener)

	require.NoError(t, pm.Start("setup"))
	require.NoError(t, pm.Complete("setup"))

	assert.Len(t, listener.completes, 1)
	assert.Equal(t, "setup", listener.completes[0].Name)
}

func TestPhaseManager_Listener_Error(t *testing.T) {
	pm := NewPhaseManager()
	listener := &mockPhaseListener{}
	pm.AddListener(listener)

	require.NoError(t, pm.Start("setup"))
	require.NoError(t, pm.Fail("setup", fmt.Errorf("oops")))

	assert.Len(t, listener.errors, 1)
	assert.Equal(t, "setup", listener.errors[0].Name)
}

// Stress test: concurrent phase reads.
func TestPhaseManager_Stress_ConcurrentReads(t *testing.T) {
	pm := NewPhaseManager()
	require.NoError(t, pm.Start("setup"))

	var wg sync.WaitGroup
	const goroutines = 50

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_ = pm.Current()
			_ = pm.All()
		}()
	}
	wg.Wait()
}
