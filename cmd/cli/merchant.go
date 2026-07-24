package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func merchantCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "merchant",
		Short: "Manage merchants",
	}

	cmd.AddCommand(
		merchantListCmd(),
		merchantGetCmd(),
		merchantCreateCmd(),
	)

	return cmd
}

func merchantListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all merchants",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := apiGet("/api/v1/merchants")
			if err != nil {
				return err
			}
			var result struct {
				Merchants []map[string]interface{} `json:"merchants"`
			}
			json.Unmarshal(data, &result)

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tSTATUS\tCOUNTRY")
			for _, m := range result.Merchants {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					m["id"], m["legal_name"], m["status"], m["country"])
			}
			w.Flush()
			return nil
		},
	}
}

func merchantGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get [id]",
		Short: "Get merchant details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := apiGet("/api/v1/merchants/" + args[0])
			if err != nil {
				return err
			}
			var m map[string]interface{}
			json.Unmarshal(data, &m)
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			for k, v := range m {
				fmt.Fprintf(w, "%s:\t%v\n", k, v)
			}
			w.Flush()
			return nil
		},
	}
}

func merchantCreateCmd() *cobra.Command {
	var legalName, email, country, currency string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new merchant",
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]string{
				"legal_name": legalName,
				"email":      email,
				"country":    country,
				"currency":   currency,
			}
			data, err := apiPost("/api/v1/merchants", body)
			if err != nil {
				return err
			}
			var m map[string]interface{}
			json.Unmarshal(data, &m)
			fmt.Printf("Merchant created: %s\n", m["id"])
			return nil
		},
	}

	cmd.Flags().StringVar(&legalName, "name", "", "Legal name")
	cmd.Flags().StringVar(&email, "email", "", "Email")
	cmd.Flags().StringVar(&country, "country", "", "Country code")
	cmd.Flags().StringVar(&currency, "currency", "USD", "Currency")
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("email")
	cmd.MarkFlagRequired("country")

	return cmd
}
