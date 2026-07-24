package main

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestAuthCmdStructure(t *testing.T) {
	cmd := authCmd()

	if cmd.Use != "auth" {
		t.Errorf("expected Use='auth', got %q", cmd.Use)
	}
	if cmd.Short != "Authentication commands" {
		t.Errorf("expected Short='Authentication commands', got %q", cmd.Short)
	}

	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Use] = true
	}

	if !subNames["login"] {
		t.Error("auth command missing 'login' subcommand")
	}
	if !subNames["logout"] {
		t.Error("auth command missing 'logout' subcommand")
	}
	if len(cmd.Commands()) != 2 {
		t.Errorf("expected 2 subcommands, got %d", len(cmd.Commands()))
	}
}

func TestAuthLoginCmd(t *testing.T) {
	cmd := authLoginCmd()
	if cmd.Use != "login" {
		t.Errorf("expected Use='login', got %q", cmd.Use)
	}
	if cmd.Short != "Login to Helix Seller" {
		t.Errorf("expected Short='Login to Helix Seller', got %q", cmd.Short)
	}

	// Check required flags
	emailFlag := cmd.Flags().Lookup("email")
	if emailFlag == nil {
		t.Error("expected 'email' flag to exist")
	}
	passwordFlag := cmd.Flags().Lookup("password")
	if passwordFlag == nil {
		t.Error("expected 'password' flag to exist")
	}
}

func TestAuthLoginCmdRequiresFlags(t *testing.T) {
	// Missing both flags
	cmd := authLoginCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when required flags are missing, got nil")
	}

	// Missing password flag
	cmd2 := authLoginCmd()
	cmd2.SetArgs([]string{"--email", "test@example.com"})
	err = cmd2.Execute()
	if err == nil {
		t.Error("expected error when --password is missing, got nil")
	}

	// Missing email flag
	cmd3 := authLoginCmd()
	cmd3.SetArgs([]string{"--password", "secret"})
	err = cmd3.Execute()
	if err == nil {
		t.Error("expected error when --email is missing, got nil")
	}
}

func TestAuthLogoutCmd(t *testing.T) {
	cmd := authLogoutCmd()
	if cmd.Use != "logout" {
		t.Errorf("expected Use='logout', got %q", cmd.Use)
	}
	if cmd.Short != "Logout from Helix Seller" {
		t.Errorf("expected Short='Logout from Helix Seller', got %q", cmd.Short)
	}
	if cmd.RunE == nil {
		t.Error("expected RunE to be set")
	}
}

func TestAuthCmdHelp(t *testing.T) {
	cmd := authCmd()
	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()
	if err != nil {
		t.Errorf("expected nil error for --help, got %v", err)
	}
}

func TestAuthSubcommandHelp(t *testing.T) {
	subCmds := []func() *cobra.Command{authLoginCmd, authLogoutCmd}
	for _, fn := range subCmds {
		cmd := fn()
		cmd.SetArgs([]string{"--help"})
		err := cmd.Execute()
		if err != nil {
			t.Errorf("expected nil error for %s --help, got %v", cmd.Use, err)
		}
	}
}
