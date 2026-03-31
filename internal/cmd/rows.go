package cmd

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/AfeefRazick/coda-cli/internal/api"
	"github.com/spf13/cobra"
)

func newRowsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rows",
		Short: "Read and write Coda rows",
		Long:  "List, get, insert, upsert, update, and delete rows in a Coda table.",
	}

	listCmd := &cobra.Command{
		Use:   "list <doc> <table>",
		Short: "List rows in a table",
		Long: `List rows in a Coda table.

Use --all to automatically paginate and fetch every row. Use --sync-token
to perform an incremental sync and receive only rows changed since the
last sync. Output is always JSON.`,
		Args: exactArgsFor("coda rows list <doc> <table>", 2),
		Example: strings.Join([]string{
			"  coda rows list AbCDeFGH grid-pqrs",
			"  coda rows list AbCDeFGH grid-pqrs --all --use-column-names",
			"  coda rows list AbCDeFGH grid-pqrs --query \"Status=Done\" --limit 100",
			"  coda rows list AbCDeFGH grid-pqrs --sync-token <token>",
		}, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := api.NewClient()
			if err != nil {
				return err
			}
			query := url.Values{}
			copyStringQueryFlag(cmd, query, "query")
			copyStringQueryFlag(cmd, query, "sort-by", "sortBy")
			copyStringQueryFlag(cmd, query, "limit")
			copyStringQueryFlag(cmd, query, "page-token", "pageToken")
			copyStringQueryFlag(cmd, query, "sync-token", "syncToken")
			copyStringQueryFlag(cmd, query, "value-format", "valueFormat")
			if value, _ := cmd.Flags().GetBool("visible-only"); value {
				query.Set("visibleOnly", strconv.FormatBool(value))
			}
			if value, _ := cmd.Flags().GetBool("use-column-names"); value {
				query.Set("useColumnNames", strconv.FormatBool(value))
			}
			path := "/docs/" + escapeSegment(args[0]) + "/tables/" + escapeSegment(args[1]) + "/rows"
			all, _ := cmd.Flags().GetBool("all")
			if all {
				items, meta, err := paginateItems(cmd.Context(), client, path, query)
				if err != nil {
					return err
				}
				return printJSONMarshal(map[string]any{"items": items, "nextSyncToken": meta.NextSyncToken})
			}
			body, _, _, err := client.Request(cmd.Context(), http.MethodGet, path, query, nil)
			if err != nil {
				return err
			}
			return printJSON(body)
		},
	}
	listCmd.Flags().String("query", "", "Filter rows using a Coda query string (e.g. \"Status=Done\")")
	listCmd.Flags().String("sort-by", "", "Sort order: \"natural\" or a column ID/name")
	listCmd.Flags().Int("limit", 25, "Number of rows to return per page")
	listCmd.Flags().String("page-token", "", "Token to fetch the next page of results")
	listCmd.Flags().String("sync-token", "", "Incremental sync token to receive only changed rows")
	listCmd.Flags().String("value-format", "simple", "Row value format: simple, simpleWithArrays, rich")
	listCmd.Flags().Bool("visible-only", false, "Only return visible rows (excludes filtered-out rows)")
	listCmd.Flags().Bool("use-column-names", false, "Use column names instead of IDs in the response")
	listCmd.Flags().Bool("all", false, "Fetch all pages and return every row")
	cmd.AddCommand(listCmd)

	getCmd := &cobra.Command{
		Use:   "get <doc> <table> <row>",
		Short: "Get a row",
		Long:  "Print the full data for a single row as JSON.",
		Args:  exactArgsFor("coda rows get <doc> <table> <row>", 3),
		Example: strings.Join([]string{
			"  coda rows get AbCDeFGH grid-pqrs i-abc123",
			"  coda rows get AbCDeFGH grid-pqrs i-abc123 --use-column-names",
		}, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := api.NewClient()
			if err != nil {
				return err
			}
			query := url.Values{}
			copyStringQueryFlag(cmd, query, "value-format", "valueFormat")
			if value, _ := cmd.Flags().GetBool("use-column-names"); value {
				query.Set("useColumnNames", strconv.FormatBool(value))
			}
			body, _, _, err := client.Request(cmd.Context(), http.MethodGet, "/docs/"+escapeSegment(args[0])+"/tables/"+escapeSegment(args[1])+"/rows/"+escapeSegment(args[2]), query, nil)
			if err != nil {
				return err
			}
			return printJSON(body)
		},
	}
	getCmd.Flags().String("value-format", "simple", "Row value format: simple, simpleWithArrays, rich")
	getCmd.Flags().Bool("use-column-names", false, "Use column names instead of IDs in the response")
	cmd.AddCommand(getCmd)

	insertCmd := &cobra.Command{
		Use:   "insert <doc> <table>",
		Short: "Insert a row",
		Long: `Insert a new row into a Coda table.

Use --value column=value (repeatable) to specify cell values. Values are
parsed automatically — numbers, booleans, JSON arrays and objects are
detected. Use --disable-parsing to treat all values as literal strings.

Use --input to pass a full JSON request body instead of --value flags.
Use --wait to block until the row is committed.`,
		Args: exactArgsFor("coda rows insert <doc> <table>", 2),
		Example: strings.Join([]string{
			"  coda rows insert AbCDeFGH grid-pqrs --value Name='Quarterly plan' --value Status=Draft --wait",
			"  coda rows insert AbCDeFGH grid-pqrs --value Count=42 --value Active=true",
			"  coda rows insert AbCDeFGH grid-pqrs --input rows-insert.json --wait",
		}, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRowsUpsert(cmd, args, nil)
		},
	}
	insertCmd.Flags().String("input", "", "Path to a JSON request body (uses Coda API schema directly)")
	addRowEditFlags(insertCmd)
	addWaitFlags(insertCmd)
	cmd.AddCommand(insertCmd)

	upsertCmd := &cobra.Command{
		Use:   "upsert <doc> <table>",
		Short: "Upsert a row",
		Long: `Insert or update a row based on one or more key columns.

If a row with matching key column values exists it will be updated;
otherwise a new row is inserted. At least one --key is required.

Use --value column=value (repeatable) to specify cell values.
Use --input to pass a full JSON request body instead.`,
		Args: exactArgsFor("coda rows upsert <doc> <table>", 2),
		Example: strings.Join([]string{
			"  coda rows upsert AbCDeFGH grid-pqrs --key Name --value Name='Q3 plan' --value Status=Active --wait",
			"  coda rows upsert AbCDeFGH grid-pqrs --key Name --key Status --value Name=Foo --value Status=Done --wait",
			"  coda rows upsert AbCDeFGH grid-pqrs --input rows-upsert.json --wait",
		}, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			keys, err := cmd.Flags().GetStringArray("key")
			if err != nil {
				return err
			}
			if len(keys) == 0 {
				return errors.New("at least one --key is required")
			}
			return runRowsUpsert(cmd, args, keys)
		},
	}
	upsertCmd.Flags().String("input", "", "Path to a JSON request body (uses Coda API schema directly)")
	addRowEditFlags(upsertCmd)
	upsertCmd.Flags().StringArray("key", nil, "Column ID or name to use as the upsert key (repeatable)")
	addWaitFlags(upsertCmd)
	cmd.AddCommand(upsertCmd)

	updateCmd := &cobra.Command{
		Use:   "update <doc> <table> <row>",
		Short: "Update a row",
		Long: `Update cell values in an existing row.

Use --value column=value (repeatable) to specify cell updates.
Use --input to pass a full JSON request body instead.
Use --wait to block until the mutation completes.`,
		Args: exactArgsFor("coda rows update <doc> <table> <row>", 3),
		Example: strings.Join([]string{
			"  coda rows update AbCDeFGH grid-pqrs i-abc123 --value Status=Done --wait",
			"  coda rows update AbCDeFGH grid-pqrs i-abc123 --value Name='New Name' --value Count=5",
			"  coda rows update AbCDeFGH grid-pqrs i-abc123 --input row-update.json --wait",
		}, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := api.NewClient()
			if err != nil {
				return err
			}
			query := url.Values{}
			if disable, _ := cmd.Flags().GetBool("disable-parsing"); disable {
				query.Set("disableParsing", "true")
			}
			payload, err := rowUpdatePayload(cmd)
			if err != nil {
				return err
			}
			body, _, _, err := client.Request(cmd.Context(), http.MethodPut, "/docs/"+escapeSegment(args[0])+"/tables/"+escapeSegment(args[1])+"/rows/"+escapeSegment(args[2]), query, payload)
			if err != nil {
				return err
			}
			return maybeWaitAndPrint(cmd, client, body)
		},
	}
	updateCmd.Flags().String("input", "", "Path to a JSON request body (uses Coda API schema directly)")
	addRowEditFlags(updateCmd)
	addWaitFlags(updateCmd)
	cmd.AddCommand(updateCmd)

	deleteCmd := &cobra.Command{
		Use:   "delete <doc> <table> <row>",
		Short: "Delete a row",
		Long: `Permanently delete a single row from a Coda table.

Prompts for confirmation unless --yes is provided. Use --wait to block
until the deletion mutation completes.`,
		Args:    exactArgsFor("coda rows delete <doc> <table> <row>", 3),
		Example: "  coda rows delete AbCDeFGH grid-pqrs i-abc123 --yes --wait",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDelete(cmd, "/docs/"+escapeSegment(args[0])+"/tables/"+escapeSegment(args[1])+"/rows/"+escapeSegment(args[2]), "row", args[2])
		},
	}
	addConfirmFlag(deleteCmd)
	addWaitFlags(deleteCmd)
	cmd.AddCommand(deleteCmd)

	deleteManyCmd := &cobra.Command{
		Use:   "delete-many <doc> <table>",
		Short: "Delete multiple rows",
		Long: `Delete multiple rows from a Coda table in a single request.

Provide one --row flag per row ID, or use --input with a JSON body
containing a rowIds array. Prompts for confirmation unless --yes is set.`,
		Args: exactArgsFor("coda rows delete-many <doc> <table>", 2),
		Example: strings.Join([]string{
			"  coda rows delete-many AbCDeFGH grid-pqrs --row i-123 --row i-456 --yes --wait",
			"  coda rows delete-many AbCDeFGH grid-pqrs --input rows-delete.json --yes --wait",
		}, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := confirmDestructive(cmd, "rows", args[1]); err != nil {
				return err
			}
			client, _, err := api.NewClient()
			if err != nil {
				return err
			}
			payload, err := rowsDeletePayload(cmd)
			if err != nil {
				return err
			}
			body, _, _, err := client.Request(cmd.Context(), http.MethodDelete, "/docs/"+escapeSegment(args[0])+"/tables/"+escapeSegment(args[1])+"/rows", nil, payload)
			if err != nil {
				return err
			}
			return maybeWaitAndPrint(cmd, client, body)
		},
	}
	deleteManyCmd.Flags().String("input", "", "Path to a JSON request body (uses Coda API schema directly)")
	deleteManyCmd.Flags().StringArray("row", nil, "Row ID to delete (repeatable)")
	addConfirmFlag(deleteManyCmd)
	addWaitFlags(deleteManyCmd)
	cmd.AddCommand(deleteManyCmd)

	pushButtonCmd := &cobra.Command{
		Use:   "push-button <doc> <table> <row> <column>",
		Short: "Push a button column for a row",
		Long: `Trigger a button column action for a specific row.

The column argument must be the ID or name of a button-type column.
Use --wait to block until the button action mutation completes.`,
		Args:    exactArgsFor("coda rows push-button <doc> <table> <row> <column>", 4),
		Example: "  coda rows push-button AbCDeFGH grid-pqrs i-abc123 c-button --wait",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := api.NewClient()
			if err != nil {
				return err
			}
			body, _, _, err := client.Request(cmd.Context(), http.MethodPost, "/docs/"+escapeSegment(args[0])+"/tables/"+escapeSegment(args[1])+"/rows/"+escapeSegment(args[2])+"/buttons/"+escapeSegment(args[3]), nil, nil)
			if err != nil {
				return err
			}
			return maybeWaitAndPrint(cmd, client, body)
		},
	}
	addWaitFlags(pushButtonCmd)
	cmd.AddCommand(pushButtonCmd)

	return cmd
}

func runRowsUpsert(cmd *cobra.Command, args []string, keys []string) error {
	client, _, err := api.NewClient()
	if err != nil {
		return err
	}

	query := url.Values{}
	if disable, _ := cmd.Flags().GetBool("disable-parsing"); disable {
		query.Set("disableParsing", "true")
	}

	payload, err := rowsUpsertPayload(cmd, keys)
	if err != nil {
		return err
	}

	body, _, _, err := client.Request(cmd.Context(), http.MethodPost, "/docs/"+escapeSegment(args[0])+"/tables/"+escapeSegment(args[1])+"/rows", query, payload)
	if err != nil {
		return err
	}

	return maybeWaitAndPrint(cmd, client, body)
}
