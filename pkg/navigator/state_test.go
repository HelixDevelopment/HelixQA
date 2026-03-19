// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package navigator

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewStateTracker(t *testing.T) {
	st := NewStateTracker()
	assert.NotNil(t, st)
	assert.Equal(t, "", st.CurrentScreen())
	assert.Equal(t, 0, st.ActionCount())
	assert.Equal(t, 0, st.ErrorCount())
	assert.Empty(t, st.History())
}

func TestStateTracker_SetCurrentScreen(t *testing.T) {
	st := NewStateTracker()
	st.SetCurrentScreen("screen-main")
	assert.Equal(t, "screen-main", st.CurrentScreen())
}

func TestStateTracker_RecordAction_Success(t *testing.T) {
	st := NewStateTracker()
	st.RecordAction("screen-a", "screen-b", "click", true)

	assert.Equal(t, 1, st.ActionCount())
	assert.Equal(t, 0, st.ErrorCount())
	assert.Equal(t, "screen-b", st.CurrentScreen())

	history := st.History()
	assert.Len(t, history, 1)
	assert.Equal(t, "screen-a", history[0].FromScreen)
	assert.Equal(t, "screen-b", history[0].ToScreen)
	assert.Equal(t, "click", history[0].Action)
	assert.True(t, history[0].Success)
}

func TestStateTracker_RecordAction_Failure(t *testing.T) {
	st := NewStateTracker()
	st.SetCurrentScreen("screen-a")
	st.RecordAction("screen-a", "", "scroll", false)

	assert.Equal(t, 1, st.ActionCount())
	assert.Equal(t, 1, st.ErrorCount())
	// Current screen should not change on failure.
	assert.Equal(t, "screen-a", st.CurrentScreen())
}

func TestStateTracker_RecordAction_Multiple(t *testing.T) {
	st := NewStateTracker()
	st.RecordAction("a", "b", "click", true)
	st.RecordAction("b", "c", "scroll", true)
	st.RecordAction("c", "", "type", false)

	assert.Equal(t, 3, st.ActionCount())
	assert.Equal(t, 1, st.ErrorCount())
	assert.Equal(t, "c", st.CurrentScreen())
	assert.Len(t, st.History(), 3)
}

func TestStateTracker_History_ReturnsCopy(t *testing.T) {
	st := NewStateTracker()
	st.RecordAction("a", "b", "click", true)

	history := st.History()
	history[0].Action = "modified"

	original := st.History()
	assert.Equal(t, "click", original[0].Action)
}

func TestStateTracker_Elapsed(t *testing.T) {
	st := NewStateTracker()
	elapsed := st.Elapsed()
	assert.GreaterOrEqual(t, elapsed.Nanoseconds(), int64(0))
}

func TestStateTracker_Reset(t *testing.T) {
	st := NewStateTracker()
	st.SetCurrentScreen("screen-a")
	st.RecordAction("a", "b", "click", true)
	st.RecordAction("b", "", "type", false)

	st.Reset()
	assert.Equal(t, "", st.CurrentScreen())
	assert.Equal(t, 0, st.ActionCount())
	assert.Equal(t, 0, st.ErrorCount())
	assert.Empty(t, st.History())
}

// Stress test: concurrent state operations.
func TestStateTracker_Stress_ConcurrentOperations(t *testing.T) {
	st := NewStateTracker()
	const goroutines = 20
	const ops = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(gID int) {
			defer wg.Done()
			for i := 0; i < ops; i++ {
				from := fmt.Sprintf("screen-%d-%d", gID, i)
				to := fmt.Sprintf("screen-%d-%d", gID, i+1)
				st.RecordAction(from, to, "click", i%3 != 0)
				_ = st.CurrentScreen()
				_ = st.ActionCount()
				_ = st.ErrorCount()
			}
		}(g)
	}
	wg.Wait()

	assert.Equal(t, goroutines*ops, st.ActionCount())
	assert.Len(t, st.History(), goroutines*ops)
}
