package ai

import (
	"errors"
	"sync"
	"sync/atomic"
)

// ErrBudgetExceeded is returned when a call would push the cumulative
// session cost above the configured budget.
var ErrBudgetExceeded = errors.New("nexus ai: session budget exceeded")

// CostTracker records every LLM call and enforces a session-wide budget
// in USD. Implementations are safe for concurrent use; the tracker uses
// atomics so it stays cheap on the hot path.
type CostTracker struct {
	budgetCents atomic.Int64
	spentCents  atomic.Int64
	disabled    atomic.Bool

	mu      sync.Mutex
	entries []Entry
}

// Entry is a single tracked LLM call.
type Entry struct {
	Provider   string
	Model      string
	TokensIn   int
	TokensOut  int
	CostCents  int
	Outcome    string // pass | fail | aborted
}

// NewCostTracker creates a tracker whose starting budget equals budget
// USD (0 or negative means unlimited).
func NewCostTracker(budgetUSD float64) *CostTracker {
	t := &CostTracker{}
	t.budgetCents.Store(int64(budgetUSD * 100))
	return t
}

// Disable turns the tracker into a no-op. Useful when the operator
// explicitly removed the budget gate.
func (t *CostTracker) Disable() { t.disabled.Store(true) }

// Reserve checks whether a call of costUSD can be made. If so, it adds
// the cost to spent and returns nil. Otherwise ErrBudgetExceeded.
func (t *CostTracker) Reserve(costUSD float64) error {
	if t.disabled.Load() {
		return nil
	}
	cost := int64(costUSD * 100)
	budget := t.budgetCents.Load()
	if budget <= 0 {
		return nil
	}
	newSpend := t.spentCents.Add(cost)
	if newSpend > budget {
		// Roll back on refusal so repeated calls do not lock the session.
		t.spentCents.Add(-cost)
		return ErrBudgetExceeded
	}
	return nil
}

// Record stores an Entry for audit purposes without touching the budget.
func (t *CostTracker) Record(e Entry) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.entries = append(t.entries, e)
}

// SpentUSD returns the current total spend.
func (t *CostTracker) SpentUSD() float64 {
	return float64(t.spentCents.Load()) / 100
}

// BudgetUSD returns the configured budget.
func (t *CostTracker) BudgetUSD() float64 {
	return float64(t.budgetCents.Load()) / 100
}

// Entries returns a copy of all recorded entries.
func (t *CostTracker) Entries() []Entry {
	t.mu.Lock()
	defer t.mu.Unlock()
	cp := make([]Entry, len(t.entries))
	copy(cp, t.entries)
	return cp
}
