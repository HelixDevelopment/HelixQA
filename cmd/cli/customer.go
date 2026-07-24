package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func customerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "customer",
		Short: "Manage customers",
	}

	cmd.AddCommand(
		customerListCmd(),
		customerGetCmd(),
		customerCreateCmd(),
	)

	return cmd
}

func customerListCmd() *cobra.Command {
	var merchantID string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List customers for a merchant",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := apiGet("/api/v1/merchants/" + merchantID + "/customers")
			if err != nil {
				return err
			}
			var result struct {
				Customers []map[string]interface{} `json:"customers"`
			}
			json.Unmarshal(data, &result)

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tEMAIL\tPHONE")
			for _, c := range result.Customers {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					c["id"], c["name"], c["email"], c["phone"])
			}
			w.Flush()
			return nil
		},
	}

	cmd.Flags().StringVar(&merchantID, "merchant", "", "Merchant ID")
	cmd.MarkFlagRequired("merchant")

	return cmd
}

func customerGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get [merchant-id] [customer-id]",
		Short: "Get customer details",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := apiGet("/api/v1/merchants/" + args[0] + "/customers/" + args[1])
			if err != nil {
				return err
			}
			var c map[string]interface{}
			json.Unmarshal(data, &c)
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			for k, v := range c {
				fmt.Fprintf(w, "%s:\t%v\n", k, v)
			}
			w.Flush()
			return nil
		},
	}
}

func customerCreateCmd() *cobra.Command {
	var merchantID, name, email, phone string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new customer",
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]string{
				"name":  name,
				"email": email,
				"phone": phone,
			}
			data, err := apiPost("/api/v1/merchants/"+merchantID+"/customers", body)
			if err != nil {
				return err
			}
			var c map[string]interface{}
			json.Unmarshal(data, &c)
			fmt.Printf("Customer created: %s\n", c["id"])
			return nil
		},
	}

	cmd.Flags().StringVar(&merchantID, "merchant", "", "Merchant ID")
	cmd.Flags().StringVar(&name, "name", "", "Customer name")
	cmd.Flags().StringVar(&email, "email", "", "Email")
	cmd.Flags().StringVar(&phone, "phone", "", "Phone")
	cmd.MarkFlagRequired("merchant")
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("email")

	return cmd
}
