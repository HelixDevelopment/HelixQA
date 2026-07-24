package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func analyticsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analytics",
		Short: "View analytics and reports",
	}

	cmd.AddCommand(
		analyticsSummaryCmd(),
	)

	return cmd
}

func analyticsSummaryCmd() *cobra.Command {
	var merchantID, from, to string

	cmd := &cobra.Command{
		Use:   "summary",
		Short: "Get transaction summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			url := "/api/v1/merchants/" + merchantID + "/analytics/summary"
			if from != "" && to != "" {
				url += "?from=" + from + "&to=" + to
			}
			data, err := apiGet(url)
			if err != nil {
				return err
			}
			var s map[string]interface{}
			json.Unmarshal(data, &s)
			fmt.Printf("Revenue:    %v\n", s["total_revenue"])
			fmt.Printf("Transactions: %v\n", s["total_transactions"])
			fmt.Printf("Successful: %v\n", s["successful_transactions"])
			fmt.Printf("Failed:     %v\n", s["failed_transactions"])
			fmt.Printf("Avg Size:   %v\n", s["average_transaction_size"])
			return nil
		},
	}

	cmd.Flags().StringVar(&merchantID, "merchant", "", "Merchant ID")
	cmd.Flags().StringVar(&from, "from", "", "Start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&to, "to", "", "End date (YYYY-MM-DD)")
	cmd.MarkFlagRequired("merchant")

	return cmd
}
