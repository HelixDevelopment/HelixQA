package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func authCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authentication commands",
	}

	cmd.AddCommand(
		authLoginCmd(),
		authLogoutCmd(),
	)

	return cmd
}

func authLoginCmd() *cobra.Command {
	var email, password string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login to Helix Seller",
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]string{
				"email":    email,
				"password": password,
			}
			data, err := apiPost("/api/v1/auth/login", body)
			if err != nil {
				return err
			}
			var tokens struct {
				AccessToken  string `json:"access_token"`
				RefreshToken string `json:"refresh_token"`
			}
			json.Unmarshal(data, &tokens)
			viper.Set("api_key", tokens.AccessToken)
			viper.WriteConfigAs("$HOME/.helix.yaml")
			fmt.Println("Logged in successfully")
			return nil
		},
	}

	cmd.Flags().StringVar(&email, "email", "", "Email")
	cmd.Flags().StringVar(&password, "password", "", "Password")
	cmd.MarkFlagRequired("email")
	cmd.MarkFlagRequired("password")

	return cmd
}

func authLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Logout from Helix Seller",
		RunE: func(cmd *cobra.Command, args []string) error {
			viper.Set("api_key", "")
			viper.WriteConfigAs("$HOME/.helix.yaml")
			fmt.Println("Logged out")
			return nil
		},
	}
}
