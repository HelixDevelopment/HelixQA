package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func transactionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transaction",
		Short: "Manage transactions",
	}

	cmd.AddCommand(
		transactionListCmd(),
		transactionGetCmd(),
	)

	return cmd
}

func transactionListCmd() *cobra.Command {
	var merchantID string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List transactions for a merchant",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := apiGet("/api/v1/merchants/" + merchantID + "/transactions")
			if err != nil {
				return err
			}
			var result struct {
				Transactions []map[string]interface{} `json:"transactions"`
			}
			json.Unmarshal(data, &result)

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tAMOUNT\tCURRENCY\tSTATUS\tPROVIDER")
			for _, t := range result.Transactions {
				fmt.Fprintf(w, "%s\t%v\t%s\t%s\t%s\n",
					t["id"], t["amount"], t["currency"], t["status"], t["provider"])
			}
			w.Flush()
			return nil
		},
	}

	cmd.Flags().StringVar(&merchantID, "merchant", "", "Merchant ID")
	cmd.MarkFlagRequired("merchant")

	return cmd
}

func transactionGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get [merchant-id] [transaction-id]",
		Short: "Get transaction details",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := apiGet("/api/v1/merchants/" + args[0] + "/transactions/" + args[1])
			if err != nil {
				return err
			}
			var t map[string]interface{}
			json.Unmarshal(data, &t)
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			for k, v := range t {
				fmt.Fprintf(w, "%s:\t%v\n", k, v)
			}
			w.Flush()
			return nil
		},
	}
}
