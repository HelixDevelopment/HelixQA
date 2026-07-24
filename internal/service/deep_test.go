package service

import (
	"testing"
	"time"

	"github.com/helix-seller/helix-seller/internal/model"
)

func TestExchangeRateService_Convert_SameCurrency(t *testing.T) {
	svc := NewExchangeRateService(nil, nil)

	tests := []struct {
		name       string
		amount     int64
		currency   string
	}{
		{"USD to USD", 10000, "USD"},
		{"EUR to EUR", 0, "EUR"},
		{"GBP to GBP", 999999, "GBP"},
		{"JPY to JPY", -500, "JPY"},
		{"RSD to RSD", 1, "RSD"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converted, rate, err := svc.Convert(nil, tt.amount, tt.currency, tt.currency)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if converted != tt.amount {
				t.Errorf("converted = %d, want %d (same currency should return same amount)", converted, tt.amount)
			}
			if rate != 1.0 {
				t.Errorf("rate = %f, want 1.0 for same currency", rate)
			}
		})
	}
}

func TestExchangeRateService_Convert_SameCurrency_NilContext(t *testing.T) {
	svc := NewExchangeRateService(nil, nil)
	converted, rate, err := svc.Convert(nil, 5000, "USD", "USD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if converted != 5000 {
		t.Errorf("converted = %d, want 5000", converted)
	}
	if rate != 1.0 {
		t.Errorf("rate = %f, want 1.0", rate)
	}
}

func TestExchangeRateService_Convert_SameCurrency_ZeroAmount(t *testing.T) {
	svc := NewExchangeRateService(nil, nil)
	converted, rate, err := svc.Convert(nil, 0, "EUR", "EUR")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if converted != 0 {
		t.Errorf("converted = %d, want 0", converted)
	}
	if rate != 1.0 {
		t.Errorf("rate = %f, want 1.0", rate)
	}
}

func TestExchangeRateService_Convert_SameCurrency_NegativeAmount(t *testing.T) {
	svc := NewExchangeRateService(nil, nil)
	converted, rate, err := svc.Convert(nil, -5000, "GBP", "GBP")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if converted != -5000 {
		t.Errorf("converted = %d, want -5000", converted)
	}
	if rate != 1.0 {
		t.Errorf("rate = %f, want 1.0", rate)
	}
}

func TestCalculatePeriodEnd_LeapYear(t *testing.T) {
	start := time.Date(2028, 1, 15, 0, 0, 0, 0, time.UTC)
	got := calculatePeriodEnd(start, model.SubscriptionIntervalMonth, 1)
	want := time.Date(2028, 2, 15, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("leap year month: got %v, want %v", got, want)
	}
}

func TestCalculatePeriodEnd_LeapYear_Feb29(t *testing.T) {
	start := time.Date(2028, 2, 29, 0, 0, 0, 0, time.UTC)
	got := calculatePeriodEnd(start, model.SubscriptionIntervalYear, 1)
	want := time.Date(2029, 3, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("leap year to non-leap year: got %v, want %v", got, want)
	}
}

func TestCalculatePeriodEnd_EndOfMonth(t *testing.T) {
	tests := []struct {
		name  string
		start time.Time
		count int16
		want  time.Time
	}{
		{
			"Jan 31 + 1 month overflows to Mar 3",
			time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),
			1,
			time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC),
		},
		{
			"Jan 31 + 2 months = Mar 31",
			time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),
			2,
			time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
		},
		{
			"Jan 31 + 3 months overflows to May 1",
			time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),
			3,
			time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			"Nov 30 + 1 month = Dec 30",
			time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC),
			1,
			time.Date(2026, 12, 30, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculatePeriodEnd(tt.start, model.SubscriptionIntervalMonth, tt.count)
			if !got.Equal(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalculatePeriodEnd_ZeroCount_AllIntervals(t *testing.T) {
	start := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)

	intervals := []model.SubscriptionInterval{
		model.SubscriptionIntervalDay,
		model.SubscriptionIntervalWeek,
		model.SubscriptionIntervalMonth,
		model.SubscriptionIntervalYear,
	}

	for _, iv := range intervals {
		t.Run(string(iv), func(t *testing.T) {
			got := calculatePeriodEnd(start, iv, 0)
			if !got.Equal(start) {
				t.Errorf("zero count with interval %s: got %v, want %v", iv, got, start)
			}
		})
	}
}

func TestCalculatePeriodEnd_HighCount(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	got := calculatePeriodEnd(start, model.SubscriptionIntervalDay, 100)
	want := time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("100 days: got %v, want %v", got, want)
	}

	got = calculatePeriodEnd(start, model.SubscriptionIntervalMonth, 100)
	want = time.Date(2034, 5, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("100 months: got %v, want %v", got, want)
	}

	got = calculatePeriodEnd(start, model.SubscriptionIntervalYear, 100)
	want = time.Date(2126, 1, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("100 years: got %v, want %v", got, want)
	}
}

func TestCalculatePeriodEnd_Week_HighCount(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	got := calculatePeriodEnd(start, model.SubscriptionIntervalWeek, 52)
	want := start.AddDate(0, 0, 7*52)
	if !got.Equal(want) {
		t.Errorf("52 weeks: got %v, want %v", got, want)
	}
}

func TestWebhookService_EventMatches_ComplexPatterns(t *testing.T) {
	svc := &WebhookService{}

	tests := []struct {
		name      string
		events    []string
		eventType string
		want      bool
	}{
		{"wildcard in list", []string{"payment.succeeded", "*"}, "anything", true},
		{"multiple wildcards", []string{"*", "*"}, "test", true},
		{"wildcard at end", []string{"payment.created", "payment.updated", "*"}, "payment.deleted", true},
		{"no match in long list", []string{"a", "b", "c", "d", "e"}, "f", false},
		{"exact match in long list", []string{"a", "b", "c", "payment.succeeded", "e"}, "payment.succeeded", true},
		{"empty event type", []string{"*"}, "", true},
		{"empty event type no wildcard", []string{"payment.succeeded"}, "", false},
		{"dotted events match", []string{"invoice.payment_succeeded"}, "invoice.payment_succeeded", true},
		{"dotted events partial", []string{"invoice.payment_succeeded"}, "invoice", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.eventMatches(tt.events, tt.eventType)
			if got != tt.want {
				t.Errorf("eventMatches(%v, %q) = %v, want %v", tt.events, tt.eventType, got, tt.want)
			}
		})
	}
}

func TestCalculatePeriodEnd_DefaultFallback(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	got := calculatePeriodEnd(start, model.SubscriptionInterval("bogus"), 2)
	want := start.AddDate(0, 2, 0)
	if !got.Equal(want) {
		t.Errorf("default fallback: got %v, want %v", got, want)
	}
}

func TestCalculatePeriodEnd_DayPreservesTime(t *testing.T) {
	start := time.Date(2026, 1, 1, 15, 30, 45, 0, time.UTC)
	got := calculatePeriodEnd(start, model.SubscriptionIntervalDay, 1)
	want := time.Date(2026, 1, 2, 15, 30, 45, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("day preserves time: got %v, want %v", got, want)
	}
}

func TestCalculatePeriodEnd_MonthCrossingYearBoundary(t *testing.T) {
	start := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	got := calculatePeriodEnd(start, model.SubscriptionIntervalMonth, 3)
	want := time.Date(2027, 2, 15, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("year boundary: got %v, want %v", got, want)
	}
}

func TestCalculatePeriodEnd_YearLeapYearEdge(t *testing.T) {
	start := time.Date(2028, 2, 28, 0, 0, 0, 0, time.UTC)
	got := calculatePeriodEnd(start, model.SubscriptionIntervalYear, 1)
	want := time.Date(2029, 2, 28, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("1 year from Feb 28: got %v, want %v", got, want)
	}
}

func TestExchangeRateService_Constructor_NilDB(t *testing.T) {
	svc := NewExchangeRateService(nil, nil)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if svc.db != nil {
		t.Error("db should be nil")
	}
}
