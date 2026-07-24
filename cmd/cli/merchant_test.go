package main

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestMerchantCmdStructure(t *testing.T) {
	cmd := merchantCmd()

	if cmd.Use != "merchant" {
		t.Errorf("expected Use='merchant', got %q", cmd.Use)
	}
	if cmd.Short != "Manage merchants" {
		t.Errorf("expected Short='Manage merchants', got %q", cmd.Short)
	}

	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Use] = true
	}

	if !subNames["list"] {
		t.Error("merchant command missing 'list' subcommand")
	}
	if !subNames["get [id]"] {
		t.Error("merchant command missing 'get' subcommand")
	}
	if !subNames["create"] {
		t.Error("merchant command missing 'create' subcommand")
	}
	if len(cmd.Commands()) != 3 {
		t.Errorf("expected 3 subcommands, got %d", len(cmd.Commands()))
	}
}

func TestMerchantListCmd(t *testing.T) {
	cmd := merchantListCmd()
	if cmd.Use != "list" {
		t.Errorf("expected Use='list', got %q", cmd.Use)
	}
	if cmd.Short != "List all merchants" {
		t.Errorf("expected Short='List all merchants', got %q", cmd.Short)
	}
	if cmd.RunE == nil {
		t.Error("expected RunE to be set")
	}
}

func TestMerchantGetCmd(t *testing.T) {
	cmd := merchantGetCmd()
	if cmd.Use != "get [id]" {
		t.Errorf("expected Use='get [id]', got %q", cmd.Use)
	}
	if cmd.Short != "Get merchant details" {
		t.Errorf("expected Short='Get merchant details', got %q", cmd.Short)
	}
	if cmd.RunE == nil {
		t.Error("expected RunE to be set")
	}

	// ExactArgs(1) validation: no args
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for no args, got nil")
	}

	// ExactArgs(1) validation: too many args
	cmd2 := merchantGetCmd()
	cmd2.SetArgs([]string{"id1", "id2"})
	err = cmd2.Execute()
	if err == nil {
		t.Error("expected error for 2 args, got nil")
	}
}

func TestMerchantCreateCmd(t *testing.T) {
	cmd := merchantCreateCmd()
	if cmd.Use != "create" {
		t.Errorf("expected Use='create', got %q", cmd.Use)
	}
	if cmd.Short != "Create a new merchant" {
		t.Errorf("expected Short='Create a new merchant', got %q", cmd.Short)
	}

	// Check required flags
	requiredFlags := []string{"name", "email", "country"}
	for _, flag := range requiredFlags {
		f := cmd.Flags().Lookup(flag)
		if f == nil {
			t.Errorf("expected flag %q to exist", flag)
		}
	}

	// Check optional flag with default
	currencyFlag := cmd.Flags().Lookup("currency")
	if currencyFlag == nil {
		t.Error("expected 'currency' flag to exist")
	} else if currencyFlag.DefValue != "USD" {
		t.Errorf("expected currency default 'USD', got %q", currencyFlag.DefValue)
	}
}

func TestMerchantCreateCmdFlagDefaults(t *testing.T) {
	cmd := merchantCreateCmd()

	// Verify default values for flags
	tests := []struct {
		flagName     string
		defaultValue string
	}{
		{"name", ""},
		{"email", ""},
		{"country", ""},
		{"currency", "USD"},
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

func TestMerchantCmdHelp(t *testing.T) {
	cmd := merchantCmd()
	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()
	if err != nil {
		t.Errorf("expected nil error for --help, got %v", err)
	}
}

func TestMerchantSubcommandHelp(t *testing.T) {
	subCmds := []func() *cobra.Command{merchantListCmd, merchantGetCmd, merchantCreateCmd}
	for _, fn := range subCmds {
		cmd := fn()
		cmd.SetArgs([]string{"--help"})
		err := cmd.Execute()
		if err != nil {
			t.Errorf("expected nil error for %s --help, got %v", cmd.Use, err)
		}
	}
}
