package cmd

import (
	"net/http"
	"strings"

	"github.com/AfeefRazick/coda-cli/internal/api"
	"github.com/spf13/cobra"
)

func newTablesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tables",
		Short: "Inspect Coda tables",
		Long:  "List and inspect tables (and views) within a Coda document.",
	}

	listCmd := &cobra.Command{
		Use:   "list <doc>",
		Short: "List tables in a doc",
		Long:  "List all tables and views in a Coda document. Prints a table by default; use --json for raw output.",
		Args:  exactArgsFor("coda tables list <doc>", 1),
		Example: strings.Join([]string{
			"  coda tables list AbCDeFGH",
			"  coda tables list AbCDeFGH --json",
		}, "\n"),
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
	listCmd.Flags().Bool("json", false, "Print raw JSON instead of a table")
	cmd.AddCommand(listCmd)

	cmd.AddCommand(&cobra.Command{
		Use:     "get <doc> <table>",
		Short:   "Get a table",
		Long:    "Print the full metadata for a single table or view as JSON.",
		Args:    exactArgsFor("coda tables get <doc> <table>", 2),
		Example: "  coda tables get AbCDeFGH grid-pqrs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSimpleGet(cmd.Context(), "/docs/"+escapeSegment(args[0])+"/tables/"+escapeSegment(args[1]))
		},
	})

	return cmd
}
