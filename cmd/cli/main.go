package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	baseURL string
	apiKey  string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "helix",
		Short: "Helix Seller CLI - Manage your payment platform",
		Long:  `A command-line interface for the Helix Seller payment platform.`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			viper.SetConfigName(".helix")
			viper.SetConfigType("yaml")
			viper.AddConfigPath("$HOME")
			viper.AddConfigPath(".")
			viper.AutomaticEnv()
			viper.SetEnvPrefix("HELIX")
			_ = viper.ReadInConfig()

			if baseURL == "" {
				baseURL = viper.GetString("base_url")
			}
			if baseURL == "" {
				baseURL = "http://localhost:8080"
			}
			if apiKey == "" {
				apiKey = viper.GetString("api_key")
			}
		},
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

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
