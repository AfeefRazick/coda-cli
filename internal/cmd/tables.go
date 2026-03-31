package cmd

import (
	"net/http"

	"github.com/AfeefRazick/coda-cli/internal/api"
	"github.com/spf13/cobra"
)

func newTablesCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "tables", Short: "Inspect Coda tables"}

	listCmd := &cobra.Command{
		Use:   "list <doc>",
		Short: "List tables in a doc",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := api.NewClient()
			if err != nil {
				return err
			}
			body, _, _, err := client.Request(cmd.Context(), http.MethodGet, "/docs/"+escapeSegment(args[0])+"/tables", nil, nil)
			if err != nil {
				return err
			}
			jsonOut, _ := cmd.Flags().GetBool("json")
			if jsonOut {
				return printJSON(body)
			}
			return printTableTableFromBody(body)
		},
	}
	listCmd.Flags().Bool("json", false, "Print raw JSON")
	cmd.AddCommand(listCmd)

	cmd.AddCommand(&cobra.Command{
		Use:   "get <doc> <table>",
		Short: "Get a table",
		Args:  exactArgsFor("coda tables get <doc> <table>", 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSimpleGet(cmd.Context(), "/docs/"+escapeSegment(args[0])+"/tables/"+escapeSegment(args[1]))
		},
	})

	return cmd
}
