package perf

import (
	"context"
	"strings"
	"testing"
)

func TestK6Runner_AvailableReturnsActionableError(t *testing.T) {
	r, _ := NewK6Runner("definitely-not-installed-k6-binary-xyz", "")
	err := r.Available()
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
	if !strings.Contains(err.Error(), "install") {
		t.Errorf("error should mention installing k6, got %q", err.Error())
	}
}

func TestK6Runner_RunScenarioRejectsEmptyURL(t *testing.T) {
	r, _ := NewK6Runner("k6", t.TempDir())
	_, err := r.RunScenario(context.Background(), Scenario{})
	if err == nil {
		t.Fatal("expected Scenario validation error")
	}
}

func TestK6Runner_RunScenarioFailsClearlyWhenK6Missing(t *testing.T) {
	r, _ := NewK6Runner("definitely-not-installed-k6-binary-xyz", t.TempDir())
	_, err := r.RunScenario(context.Background(), Scenario{URL: "https://example.com"})
	if err == nil {
		t.Fatal("expected missing-binary error")
	}
	if !strings.Contains(err.Error(), "not found in PATH") {
		t.Errorf("error should explain binary is missing, got %q", err.Error())
	}
}
