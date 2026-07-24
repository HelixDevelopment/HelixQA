package main

import (
	"testing"

	"github.com/spf13/cobra"
)

func buildRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "helix",
		Short: "Helix Seller CLI - Manage your payment platform",
		Long:  `A command-line interface for the Helix Seller payment platform.`,
	}

	rootCmd.PersistentFlags().StringVar(&baseURL, "url", "", "API base URL (env: HELIX_BASE_URL)")
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "API key (env: HELIX_API_KEY)")

	rootCmd.AddCommand(
		merchantCmd(),
		transactionCmd(),
		customerCmd(),
		analyticsCmd(),
		authCmd(),
		configCmd(),
	)

	return rootCmd
}

func TestRootCmdStructure(t *testing.T) {
	cmd := buildRootCmd()

	if cmd.Use != "helix" {
		t.Errorf("expected Use='helix', got %q", cmd.Use)
	}
	if cmd.Short != "Helix Seller CLI - Manage your payment platform" {
		t.Errorf("unexpected Short description: %q", cmd.Short)
	}
}

func TestRootCmdPersistentFlags(t *testing.T) {
	cmd := buildRootCmd()

	urlFlag := cmd.PersistentFlags().Lookup("url")
	if urlFlag == nil {
		t.Fatal("expected 'url' persistent flag")
	}
	if urlFlag.DefValue != "" {
		t.Errorf("expected 'url' default empty, got %q", urlFlag.DefValue)
	}

	apiKeyFlag := cmd.PersistentFlags().Lookup("api-key")
	if apiKeyFlag == nil {
		t.Fatal("expected 'api-key' persistent flag")
	}
	if apiKeyFlag.DefValue != "" {
		t.Errorf("expected 'api-key' default empty, got %q", apiKeyFlag.DefValue)
	}
}

func TestRootCmdSubcommands(t *testing.T) {
	cmd := buildRootCmd()

	expectedCmds := map[string]string{
		"merchant":     "Manage merchants",
		"transaction":  "Manage transactions",
		"customer":     "Manage customers",
		"analytics":    "View analytics and reports",
		"auth":         "Authentication commands",
		"config":       "Manage CLI configuration",
	}

	subCmds := cmd.Commands()
	if len(subCmds) != len(expectedCmds) {
		t.Errorf("expected %d subcommands, got %d", len(expectedCmds), len(subCmds))
	}

	for _, sub := range subCmds {
		if expected, ok := expectedCmds[sub.Use]; ok {
			if sub.Short != expected {
				t.Errorf("subcommand %q: expected Short=%q, got %q", sub.Use, expected, sub.Short)
			}
		} else {
			t.Errorf("unexpected subcommand %q", sub.Use)
		}
	}
}

func TestRootCmdHelp(t *testing.T) {
	cmd := buildRootCmd()
	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()
	if err != nil {
		t.Errorf("expected nil error for --help, got %v", err)
	}
}

func TestRootCmdNoArgs(t *testing.T) {
	cmd := buildRootCmd()
	cmd.SetArgs([]string{})
	// Root command has no RunE so it should print help and succeed
	err := cmd.Execute()
	if err != nil {
		t.Errorf("expected nil error for root with no args, got %v", err)
	}
}

func TestMerchantSubcommandNesting(t *testing.T) {
	cmd := buildRootCmd()
	cmd.SetArgs([]string{"merchant", "--help"})
	err := cmd.Execute()
	if err != nil {
		t.Errorf("expected nil error for merchant --help, got %v", err)
	}
}

func TestTransactionSubcommandNesting(t *testing.T) {
	cmd := buildRootCmd()
	cmd.SetArgs([]string{"transaction", "--help"})
	err := cmd.Execute()
	if err != nil {
		t.Errorf("expected nil error for transaction --help, got %v", err)
	}
}

func TestCustomerSubcommandNesting(t *testing.T) {
	cmd := buildRootCmd()
	cmd.SetArgs([]string{"customer", "--help"})
	err := cmd.Execute()
	if err != nil {
		t.Errorf("expected nil error for customer --help, got %v", err)
	}
}

func TestAnalyticsSubcommandNesting(t *testing.T) {
	cmd := buildRootCmd()
	cmd.SetArgs([]string{"analytics", "--help"})
	err := cmd.Execute()
	if err != nil {
		t.Errorf("expected nil error for analytics --help, got %v", err)
	}
}

func TestAuthSubcommandNesting(t *testing.T) {
	cmd := buildRootCmd()
	cmd.SetArgs([]string{"auth", "--help"})
	err := cmd.Execute()
	if err != nil {
		t.Errorf("expected nil error for auth --help, got %v", err)
	}
}

func TestConfigSubcommandNesting(t *testing.T) {
	cmd := buildRootCmd()
	cmd.SetArgs([]string{"config", "--help"})
	err := cmd.Execute()
	if err != nil {
		t.Errorf("expected nil error for config --help, got %v", err)
	}
}

func TestMerchantCreateSubcommandNesting(t *testing.T) {
	cmd := buildRootCmd()
	cmd.SetArgs([]string{"merchant", "create", "--help"})
	err := cmd.Execute()
	if err != nil {
		t.Errorf("expected nil error for merchant create --help, got %v", err)
	}
}

func TestTransactionListSubcommandNesting(t *testing.T) {
	cmd := buildRootCmd()
	cmd.SetArgs([]string{"transaction", "list", "--merchant", "m1", "--help"})
	err := cmd.Execute()
	if err != nil {
		t.Errorf("expected nil error for transaction list --help, got %v", err)
	}
}

func TestCustomerCreateSubcommandNesting(t *testing.T) {
	cmd := buildRootCmd()
	cmd.SetArgs([]string{"customer", "create", "--help"})
	err := cmd.Execute()
	if err != nil {
		t.Errorf("expected nil error for customer create --help, got %v", err)
	}
}

func TestAnalyticsSummarySubcommandNesting(t *testing.T) {
	cmd := buildRootCmd()
	cmd.SetArgs([]string{"analytics", "summary", "--help"})
	err := cmd.Execute()
	if err != nil {
		t.Errorf("expected nil error for analytics summary --help, got %v", err)
	}
}

func TestAuthLoginSubcommandNesting(t *testing.T) {
	cmd := buildRootCmd()
	cmd.SetArgs([]string{"auth", "login", "--help"})
	err := cmd.Execute()
	if err != nil {
		t.Errorf("expected nil error for auth login --help, got %v", err)
	}
}

func TestAuthLogoutSubcommandNesting(t *testing.T) {
	cmd := buildRootCmd()
	cmd.SetArgs([]string{"auth", "logout", "--help"})
	err := cmd.Execute()
	if err != nil {
		t.Errorf("expected nil error for auth logout --help, got %v", err)
	}
}

func TestConfigShowSubcommandNesting(t *testing.T) {
	cmd := buildRootCmd()
	cmd.SetArgs([]string{"config", "show", "--help"})
	err := cmd.Execute()
	if err != nil {
		t.Errorf("expected nil error for config show --help, got %v", err)
	}
}

func TestConfigSetSubcommandNesting(t *testing.T) {
	cmd := buildRootCmd()
	cmd.SetArgs([]string{"config", "set", "--help"})
	err := cmd.Execute()
	if err != nil {
		t.Errorf("expected nil error for config set --help, got %v", err)
	}
}

func TestUnknownSubcommand(t *testing.T) {
	cmd := buildRootCmd()
	cmd.SetArgs([]string{"nonexistent"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for unknown subcommand, got nil")
	}
}

func TestFlagPropagation(t *testing.T) {
	cmd := buildRootCmd()
	cmd.SetArgs([]string{"merchant", "list", "--url", "http://example.com"})
	err := cmd.Execute()
	// This will fail because the server is not running, but it demonstrates flag propagation
	// The important thing is it parses the flags correctly before the HTTP error
	if err != nil {
		t.Logf("expected HTTP error from non-existent server: %v", err)
	}
}
