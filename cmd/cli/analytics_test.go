package main

import (
	"testing"
)

func TestAnalyticsCmdStructure(t *testing.T) {
	cmd := analyticsCmd()

	if cmd.Use != "analytics" {
		t.Errorf("expected Use='analytics', got %q", cmd.Use)
	}
	if cmd.Short != "View analytics and reports" {
		t.Errorf("expected Short='View analytics and reports', got %q", cmd.Short)
	}

	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Use] = true
	}

	if !subNames["summary"] {
		t.Error("analytics command missing 'summary' subcommand")
	}
	if len(cmd.Commands()) != 1 {
		t.Errorf("expected 1 subcommand, got %d", len(cmd.Commands()))
	}
}

func TestAnalyticsSummaryCmd(t *testing.T) {
	cmd := analyticsSummaryCmd()
	if cmd.Use != "summary" {
		t.Errorf("expected Use='summary', got %q", cmd.Use)
	}
	if cmd.Short != "Get transaction summary" {
		t.Errorf("expected Short='Get transaction summary', got %q", cmd.Short)
	}
	if cmd.RunE == nil {
		t.Error("expected RunE to be set")
	}

	// Check required flag
	merchantFlag := cmd.Flags().Lookup("merchant")
	if merchantFlag == nil {
		t.Error("expected 'merchant' flag to exist")
	}

	// Check optional date range flags
	fromFlag := cmd.Flags().Lookup("from")
	if fromFlag == nil {
		t.Error("expected 'from' flag to exist")
	}
	toFlag := cmd.Flags().Lookup("to")
	if toFlag == nil {
		t.Error("expected 'to' flag to exist")
	}
}

func TestAnalyticsSummaryCmdFlagDefaults(t *testing.T) {
	cmd := analyticsSummaryCmd()

	tests := []struct {
		flagName     string
		defaultValue string
	}{
		{"merchant", ""},
		{"from", ""},
		{"to", ""},
	}

	for _, tt := range tests {
		t.Run(tt.flagName, func(t *testing.T) {
			f := cmd.Flags().Lookup(tt.flagName)
			if f == nil {
				t.Fatalf("flag %q not found", tt.flagName)
			}
			if f.DefValue != tt.defaultValue {
				t.Errorf("flag %q default = %q, want %q", tt.flagName, f.DefValue, tt.defaultValue)
			}
		})
	}
}

func TestAnalyticsCmdHelp(t *testing.T) {
	cmd := analyticsCmd()
	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()
	if err != nil {
		t.Errorf("expected nil error for --help, got %v", err)
	}
}

func TestAnalyticsSummarySubcommandHelp(t *testing.T) {
	cmd := analyticsSummaryCmd()
	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()
	if err != nil {
		t.Errorf("expected nil error for summary --help, got %v", err)
	}
}

func TestAnalyticsSummaryRequiresMerchantFlag(t *testing.T) {
	cmd := analyticsSummaryCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when --merchant is missing, got nil")
	}
}
