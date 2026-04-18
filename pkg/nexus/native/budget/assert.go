// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package budget

import (
	"errors"
	"fmt"
	"time"
)

// ErrBudgetExceeded is returned when a measured value exceeds its
// budget. Wrapped; callers should use errors.Is.
var ErrBudgetExceeded = errors.New("budget exceeded")

// AssertWithin returns nil if got <= budget, else ErrBudgetExceeded
// wrapped with a descriptive message.
func AssertWithin(name string, got, budget time.Duration) error {
	if got <= budget {
		return nil
	}
	return fmt.Errorf("%w: %s = %s > budget %s",
		ErrBudgetExceeded, name, got, budget)
}

// RecordedMetric pairs a measurement with its budget so reports can
// flag regressions declaratively.
type RecordedMetric struct {
	Name   string
	Value  time.Duration
	Budget time.Duration
}

// Within reports whether the metric is within its budget.
func (m RecordedMetric) Within() bool { return m.Value <= m.Budget }
