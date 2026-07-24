package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage CLI configuration",
	}

	cmd.AddCommand(
		configShowCmd(),
		configSetCmd(),
	)

	return cmd
}

func configShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Base URL: %s\n", viper.GetString("base_url"))
			fmt.Printf("API Key:  %s\n", maskKey(viper.GetString("api_key")))
		},
	}
}

func configSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set [key] [value]",
		Short: "Set configuration value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			viper.Set(args[0], args[1])
			if err := viper.WriteConfigAs("$HOME/.helix.yaml"); err != nil {
				return viper.SafeWriteConfigAs("$HOME/.helix.yaml")
			}
			fmt.Printf("Set %s = %s\n", args[0], args[1])
			return nil
		},
	}
}

func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}
