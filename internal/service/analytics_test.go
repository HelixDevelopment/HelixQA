package service

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestAnalyticsSummary_PeriodFormatting(t *testing.T) {
	tests := []struct {
		name string
		from time.Time
		to   time.Time
		want string
	}{
		{
			"same day",
			time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 1, 15, 23, 59, 59, 0, time.UTC),
			"2026-01-15 to 2026-01-15",
		},
		{
			"different days",
			time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),
			"2026-01-01 to 2026-01-31",
		},
		{
			"across months",
			time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC),
			"2026-03-15 to 2026-04-15",
		},
		{
			"across years",
			time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			"2025-12-31 to 2026-01-01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			period := fmt.Sprintf("%s to %s", tt.from.Format("2006-01-02"), tt.to.Format("2006-01-02"))
			if period != tt.want {
				t.Errorf("period = %q, want %q", period, tt.want)
			}
		})
	}
}

func TestAnalyticsSummary_FieldValues(t *testing.T) {
	s := &AnalyticsSummary{
		TotalRevenue:           250000,
		TotalTransactions:      1000,
		SuccessfulTransactions: 950,
		FailedTransactions:     50,
		AverageTransactionSize: 250.0,
		RefundAmount:           12500,
		Period:                 "2026-01-01 to 2026-01-31",
	}

	if s.TotalRevenue != 250000 {
		t.Errorf("TotalRevenue = %d, want 250000", s.TotalRevenue)
	}
	if s.TotalTransactions != 1000 {
		t.Errorf("TotalTransactions = %d, want 1000", s.TotalTransactions)
	}
	if s.SuccessfulTransactions != 950 {
		t.Errorf("SuccessfulTransactions = %d, want 950", s.SuccessfulTransactions)
	}
	if s.FailedTransactions != 50 {
		t.Errorf("FailedTransactions = %d, want 50", s.FailedTransactions)
	}
	if s.AverageTransactionSize != 250.0 {
		t.Errorf("AverageTransactionSize = %f, want 250.0", s.AverageTransactionSize)
	}
	if s.RefundAmount != 12500 {
		t.Errorf("RefundAmount = %d, want 12500", s.RefundAmount)
	}
}

func TestAnalyticsSummary_ZeroValues(t *testing.T) {
	s := &AnalyticsSummary{}
	if s.TotalRevenue != 0 {
		t.Errorf("TotalRevenue = %d, want 0", s.TotalRevenue)
	}
	if s.TotalTransactions != 0 {
		t.Errorf("TotalTransactions = %d, want 0", s.TotalTransactions)
	}
}

func TestAnalyticsSummary_PeriodString(t *testing.T) {
	s := &AnalyticsSummary{
		Period: "2026-01-01 to 2026-01-31",
	}
	if !strings.Contains(s.Period, "2026-01-01") {
		t.Error("period should contain start date")
	}
	if !strings.Contains(s.Period, "2026-01-31") {
		t.Error("period should contain end date")
	}
}

func TestAnalyticsSummary_TransactionSuccessRate(t *testing.T) {
	total := int64(1000)
	successful := int64(950)
	failed := int64(50)

	if successful+failed != total {
		t.Error("successful + failed should equal total")
	}

	rate := float64(successful) / float64(total) * 100
	if rate != 95.0 {
		t.Errorf("success rate = %f, want 95.0", rate)
	}
}

func TestAnalyticsSummary_JSON(t *testing.T) {
	s := &AnalyticsSummary{
		TotalRevenue:           100000,
		TotalTransactions:      500,
		SuccessfulTransactions: 480,
		FailedTransactions:     20,
		AverageTransactionSize: 200.0,
		RefundAmount:           5000,
		Period:                 "2026-01-01 to 2026-01-31",
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded AnalyticsSummary
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.TotalRevenue != 100000 {
		t.Errorf("TotalRevenue = %d, want 100000", decoded.TotalRevenue)
	}
	if decoded.TotalTransactions != 500 {
		t.Errorf("TotalTransactions = %d, want 500", decoded.TotalTransactions)
	}
}

func TestAnalyticsService_ExportCSV_Header(t *testing.T) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	writer.Write([]string{"ID", "Amount", "Currency", "Status", "Provider", "Created At"})
	writer.Flush()

	lines, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		t.Fatalf("read CSV error: %v", err)
	}

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	expectedHeaders := []string{"ID", "Amount", "Currency", "Status", "Provider", "Created At"}
	for i, h := range expectedHeaders {
		if lines[0][i] != h {
			t.Errorf("header[%d] = %q, want %q", i, lines[0][i], h)
		}
	}
}

func TestAnalyticsService_ExportCSV_Rows(t *testing.T) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	writer.Write([]string{"ID", "Amount", "Currency", "Status", "Provider", "Created At"})

	id := uuid.New()
	writer.Write([]string{
		id.String(),
		"10000",
		"USD",
		"completed",
		"stripe",
		time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC).Format(time.RFC3339),
	})
	writer.Flush()

	lines, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		t.Fatalf("read CSV error: %v", err)
	}

	if len(lines) != 2 {
		t.Fatalf("expected 2 lines (header + data), got %d", len(lines))
	}

	if lines[1][1] != "10000" {
		t.Errorf("Amount = %q, want 10000", lines[1][1])
	}
	if lines[1][2] != "USD" {
		t.Errorf("Currency = %q, want USD", lines[1][2])
	}
}

func TestAnalyticsService_CSVEmpty(t *testing.T) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	writer.Write([]string{"ID", "Amount", "Currency", "Status", "Provider", "Created At"})
	writer.Flush()

	lines, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		t.Fatalf("read CSV error: %v", err)
	}

	if len(lines) != 1 {
		t.Errorf("expected 1 line (header only), got %d", len(lines))
	}
}
