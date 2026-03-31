package cmd

import (
	"net/http"
	"strings"

	"github.com/AfeefRazick/coda-cli/internal/api"
	"github.com/spf13/cobra"
)

func newColumnsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "columns",
		Short: "Inspect Coda columns",
		Long:  "List and inspect columns within a Coda table.",
	}

	listCmd := &cobra.Command{
		Use:   "list <doc> <table>",
		Short: "List columns in a table",
		Long:  "List all columns in a Coda table. Prints a table by default; use --json for raw output.",
		Args:  exactArgsFor("coda columns list <doc> <table>", 2),
		Example: strings.Join([]string{
			"  coda columns list AbCDeFGH grid-pqrs",
			"  coda columns list AbCDeFGH grid-pqrs --json",
		}, "\n"),
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
	listCmd.Flags().Bool("json", false, "Print raw JSON instead of a table")
	cmd.AddCommand(listCmd)

	cmd.AddCommand(&cobra.Command{
		Use:     "get <doc> <table> <column>",
		Short:   "Get a column",
		Long:    "Print the full metadata for a single column as JSON.",
		Args:    exactArgsFor("coda columns get <doc> <table> <column>", 3),
		Example: "  coda columns get AbCDeFGH grid-pqrs c-colName",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSimpleGet(cmd.Context(), "/docs/"+escapeSegment(args[0])+"/tables/"+escapeSegment(args[1])+"/columns/"+escapeSegment(args[2]))
		},
	})

	return cmd
}
