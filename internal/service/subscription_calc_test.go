package service

import (
	"testing"
	"time"

	"github.com/helix-seller/helix-seller/internal/model"
)

func TestCalculatePeriodEnd_Day(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	got := calculatePeriodEnd(start, model.SubscriptionIntervalDay, 1)
	want := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("day: got %v, want %v", got, want)
	}
}

func TestCalculatePeriodEnd_Day_Multiple(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	got := calculatePeriodEnd(start, model.SubscriptionIntervalDay, 7)
	want := time.Date(2026, 1, 8, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("day 7: got %v, want %v", got, want)
	}
}

func TestCalculatePeriodEnd_Week(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	got := calculatePeriodEnd(start, model.SubscriptionIntervalWeek, 1)
	want := time.Date(2026, 1, 8, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("week: got %v, want %v", got, want)
	}
}

func TestCalculatePeriodEnd_Week_Multiple(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	got := calculatePeriodEnd(start, model.SubscriptionIntervalWeek, 2)
	want := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("week 2: got %v, want %v", got, want)
	}
}

func TestCalculatePeriodEnd_Month(t *testing.T) {
	start := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	got := calculatePeriodEnd(start, model.SubscriptionIntervalMonth, 1)
	want := time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("month: got %v, want %v", got, want)
	}
}

func TestCalculatePeriodEnd_Month_Multiple(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	got := calculatePeriodEnd(start, model.SubscriptionIntervalMonth, 3)
	want := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("month 3: got %v, want %v", got, want)
	}
}

func TestCalculatePeriodEnd_Year(t *testing.T) {
	start := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	got := calculatePeriodEnd(start, model.SubscriptionIntervalYear, 1)
	want := time.Date(2027, 6, 15, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("year: got %v, want %v", got, want)
	}
}

func TestCalculatePeriodEnd_Year_Multiple(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	got := calculatePeriodEnd(start, model.SubscriptionIntervalYear, 2)
	want := time.Date(2028, 1, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("year 2: got %v, want %v", got, want)
	}
}

func TestCalculatePeriodEnd_Default(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	got := calculatePeriodEnd(start, "unknown", 1)
	want := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("default: got %v, want %v", got, want)
	}
}

func TestCalculatePeriodEnd_ZeroCount(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	got := calculatePeriodEnd(start, model.SubscriptionIntervalMonth, 0)
	if !got.Equal(start) {
		t.Errorf("zero count: got %v, want %v", got, start)
	}
}

func TestCalculatePeriodEnd_AllIntervals(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	intervals := []model.SubscriptionInterval{
		model.SubscriptionIntervalDay,
		model.SubscriptionIntervalWeek,
		model.SubscriptionIntervalMonth,
		model.SubscriptionIntervalYear,
	}
	for _, iv := range intervals {
		got := calculatePeriodEnd(start, iv, 1)
		if got.Before(start) || got.Equal(start) {
			t.Errorf("interval %s: end %v should be after start %v", iv, got, start)
		}
	}
}
