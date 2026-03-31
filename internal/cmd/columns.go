package cmd

import (
	"net/http"

	"github.com/AfeefRazick/coda-cli/internal/api"
	"github.com/spf13/cobra"
)

func newColumnsCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "columns", Short: "Inspect Coda columns"}

	listCmd := &cobra.Command{
		Use:   "list <doc> <table>",
		Short: "List columns in a table",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := api.NewClient()
			if err != nil {
				return err
			}
			body, _, _, err := client.Request(cmd.Context(), http.MethodGet, "/docs/"+escapeSegment(args[0])+"/tables/"+escapeSegment(args[1])+"/columns", nil, nil)
			if err != nil {
				return err
			}
			jsonOut, _ := cmd.Flags().GetBool("json")
			if jsonOut {
				return printJSON(body)
			}
			return printColumnTableFromBody(body)
		},
	}
	listCmd.Flags().Bool("json", false, "Print raw JSON")
	cmd.AddCommand(listCmd)

	cmd.AddCommand(&cobra.Command{
		Use:   "get <doc> <table> <column>",
		Short: "Get a column",
		Args:  exactArgsFor("coda columns get <doc> <table> <column>", 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSimpleGet(cmd.Context(), "/docs/"+escapeSegment(args[0])+"/tables/"+escapeSegment(args[1])+"/columns/"+escapeSegment(args[2]))
		},
	})

	return cmd
}
