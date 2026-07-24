package main

import (
	"testing"
)

func TestMaskKey(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "long key shows first 4 and last 4",
			input:    "abcdefghij1234",
			expected: "abcd****1234",
		},
		{
			name:     "exactly 9 chars shows first 4 and last 4",
			input:    "123456789",
			expected: "1234****6789",
		},
		{
			name:     "exactly 8 chars returns stars",
			input:    "12345678",
			expected: "****",
		},
		{
			name:     "short key returns stars",
			input:    "abc",
			expected: "****",
		},
		{
			name:     "empty key returns stars",
			input:    "",
			expected: "****",
		},
		{
			name:     "single char returns stars",
			input:    "a",
			expected: "****",
		},
		{
			name:     "long API key",
			input:    "sk_live_abcdefghijklmnop1234",
			expected: "sk_l****1234",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskKey(tt.input)
			if result != tt.expected {
				t.Errorf("maskKey(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestConfigCmdStructure(t *testing.T) {
	cmd := configCmd()

	if cmd.Use != "config" {
		t.Errorf("expected Use='config', got %q", cmd.Use)
	}
	if cmd.Short != "Manage CLI configuration" {
		t.Errorf("expected Short='Manage CLI configuration', got %q", cmd.Short)
	}

	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Use] = true
	}

	if !subNames["show"] {
		t.Error("config command missing 'show' subcommand")
	}
	if !subNames["set [key] [value]"] {
		t.Error("config command missing 'set' subcommand")
	}
	if len(cmd.Commands()) != 2 {
		t.Errorf("expected 2 subcommands, got %d", len(cmd.Commands()))
	}
}

func TestConfigShowCmd(t *testing.T) {
	cmd := configShowCmd()
	if cmd.Use != "show" {
		t.Errorf("expected Use='show', got %q", cmd.Use)
	}
	if cmd.Short != "Show current configuration" {
		t.Errorf("expected Short='Show current configuration', got %q", cmd.Short)
	}
}

func TestConfigSetCmd(t *testing.T) {
	cmd := configSetCmd()
	if cmd.Use != "set [key] [value]" {
		t.Errorf("expected Use='set [key] [value]', got %q", cmd.Use)
	}
	if cmd.Short != "Set configuration value" {
		t.Errorf("expected Short='Set configuration value', got %q", cmd.Short)
	}

	// ExactArgs(2) validation: too few args
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for no args, got nil")
	}

	// ExactArgs(2) validation: too many args
	cmd2 := configSetCmd()
	cmd2.SetArgs([]string{"a", "b", "c"})
	err = cmd2.Execute()
	if err == nil {
		t.Error("expected error for 3 args, got nil")
	}
}

func TestConfigCmdHelp(t *testing.T) {
	cmd := configCmd()
	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()
	// --help causes cobra to print help and return nil
	if err != nil {
		t.Errorf("expected nil error for --help, got %v", err)
	}
}
