package main

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestTransactionCmdStructure(t *testing.T) {
	cmd := transactionCmd()

	if cmd.Use != "transaction" {
		t.Errorf("expected Use='transaction', got %q", cmd.Use)
	}
	if cmd.Short != "Manage transactions" {
		t.Errorf("expected Short='Manage transactions', got %q", cmd.Short)
	}

	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Use] = true
	}

	if !subNames["list"] {
		t.Error("transaction command missing 'list' subcommand")
	}
	if !subNames["get [merchant-id] [transaction-id]"] {
		t.Error("transaction command missing 'get' subcommand")
	}
	if len(cmd.Commands()) != 2 {
		t.Errorf("expected 2 subcommands, got %d", len(cmd.Commands()))
	}
}

func TestTransactionListCmd(t *testing.T) {
	cmd := transactionListCmd()
	if cmd.Use != "list" {
		t.Errorf("expected Use='list', got %q", cmd.Use)
	}
	if cmd.Short != "List transactions for a merchant" {
		t.Errorf("expected Short='List transactions for a merchant', got %q", cmd.Short)
	}
	if cmd.RunE == nil {
		t.Error("expected RunE to be set")
	}

	// Check required merchant flag
	merchantFlag := cmd.Flags().Lookup("merchant")
	if merchantFlag == nil {
		t.Error("expected 'merchant' flag to exist")
	}
}

func TestTransactionGetCmd(t *testing.T) {
	cmd := transactionGetCmd()
	if cmd.Use != "get [merchant-id] [transaction-id]" {
		t.Errorf("expected Use='get [merchant-id] [transaction-id]', got %q", cmd.Use)
	}
	if cmd.Short != "Get transaction details" {
		t.Errorf("expected Short='Get transaction details', got %q", cmd.Short)
	}

	// ExactArgs(2) validation: no args
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for no args, got nil")
	}

	// ExactArgs(2) validation: 1 arg
	cmd2 := transactionGetCmd()
	cmd2.SetArgs([]string{"m1"})
	err = cmd2.Execute()
	if err == nil {
		t.Error("expected error for 1 arg, got nil")
	}

	// ExactArgs(2) validation: too many args
	cmd3 := transactionGetCmd()
	cmd3.SetArgs([]string{"m1", "t1", "t2"})
	err = cmd3.Execute()
	if err == nil {
		t.Error("expected error for 3 args, got nil")
	}
}

func TestTransactionCmdHelp(t *testing.T) {
	cmd := transactionCmd()
	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()
	if err != nil {
		t.Errorf("expected nil error for --help, got %v", err)
	}
}

func TestTransactionSubcommandHelp(t *testing.T) {
	subCmds := []func() *cobra.Command{transactionListCmd, transactionGetCmd}
	for _, fn := range subCmds {
		cmd := fn()
		cmd.SetArgs([]string{"--help"})
		err := cmd.Execute()
		if err != nil {
			t.Errorf("expected nil error for %s --help, got %v", cmd.Use, err)
		}
	}
}
