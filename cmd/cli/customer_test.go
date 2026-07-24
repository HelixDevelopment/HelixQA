package main

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestCustomerCmdStructure(t *testing.T) {
	cmd := customerCmd()

	if cmd.Use != "customer" {
		t.Errorf("expected Use='customer', got %q", cmd.Use)
	}
	if cmd.Short != "Manage customers" {
		t.Errorf("expected Short='Manage customers', got %q", cmd.Short)
	}

	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Use] = true
	}

	if !subNames["list"] {
		t.Error("customer command missing 'list' subcommand")
	}
	if !subNames["get [merchant-id] [customer-id]"] {
		t.Error("customer command missing 'get' subcommand")
	}
	if !subNames["create"] {
		t.Error("customer command missing 'create' subcommand")
	}
	if len(cmd.Commands()) != 3 {
		t.Errorf("expected 3 subcommands, got %d", len(cmd.Commands()))
	}
}

func TestCustomerListCmd(t *testing.T) {
	cmd := customerListCmd()
	if cmd.Use != "list" {
		t.Errorf("expected Use='list', got %q", cmd.Use)
	}
	if cmd.Short != "List customers for a merchant" {
		t.Errorf("expected Short='List customers for a merchant', got %q", cmd.Short)
	}
	if cmd.RunE == nil {
		t.Error("expected RunE to be set")
	}

	merchantFlag := cmd.Flags().Lookup("merchant")
	if merchantFlag == nil {
		t.Error("expected 'merchant' flag to exist")
	}
}

func TestCustomerGetCmd(t *testing.T) {
	cmd := customerGetCmd()
	if cmd.Use != "get [merchant-id] [customer-id]" {
		t.Errorf("expected Use='get [merchant-id] [customer-id]', got %q", cmd.Use)
	}
	if cmd.Short != "Get customer details" {
		t.Errorf("expected Short='Get customer details', got %q", cmd.Short)
	}

	// ExactArgs(2) validation: no args
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for no args, got nil")
	}

	// ExactArgs(2) validation: 1 arg
	cmd2 := customerGetCmd()
	cmd2.SetArgs([]string{"m1"})
	err = cmd2.Execute()
	if err == nil {
		t.Error("expected error for 1 arg, got nil")
	}

	// ExactArgs(2) validation: too many args
	cmd3 := customerGetCmd()
	cmd3.SetArgs([]string{"m1", "c1", "c2"})
	err = cmd3.Execute()
	if err == nil {
		t.Error("expected error for 3 args, got nil")
	}
}

func TestCustomerCreateCmd(t *testing.T) {
	cmd := customerCreateCmd()
	if cmd.Use != "create" {
		t.Errorf("expected Use='create', got %q", cmd.Use)
	}
	if cmd.Short != "Create a new customer" {
		t.Errorf("expected Short='Create a new customer', got %q", cmd.Short)
	}

	// Check required flags exist
	requiredFlags := []string{"merchant", "name", "email"}
	for _, flag := range requiredFlags {
		f := cmd.Flags().Lookup(flag)
		if f == nil {
			t.Errorf("expected required flag %q to exist", flag)
		}
	}

	// Check optional phone flag
	phoneFlag := cmd.Flags().Lookup("phone")
	if phoneFlag == nil {
		t.Error("expected optional 'phone' flag to exist")
	}
}

func TestCustomerCmdHelp(t *testing.T) {
	cmd := customerCmd()
	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()
	if err != nil {
		t.Errorf("expected nil error for --help, got %v", err)
	}
}

func TestCustomerSubcommandHelp(t *testing.T) {
	subCmds := []func() *cobra.Command{customerListCmd, customerGetCmd, customerCreateCmd}
	for _, fn := range subCmds {
		cmd := fn()
		cmd.SetArgs([]string{"--help"})
		err := cmd.Execute()
		if err != nil {
			t.Errorf("expected nil error for %s --help, got %v", cmd.Use, err)
		}
	}
}
