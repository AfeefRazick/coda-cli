package cmd

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/AfeefRazick/coda-cli/internal/api"
	"github.com/spf13/cobra"
)

func newDocsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Manage Coda docs",
		Long:  "List, get, create, update, and delete Coda documents.",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List docs",
		Long: `List Coda documents visible to the authenticated user.

Results are printed as a table by default. Use --json for raw API output.
Use --all to automatically paginate and return every document.`,
		Example: strings.Join([]string{
			"  coda docs list",
			"  coda docs list --all --owned",
			"  coda docs list --query roadmap --json",
			"  coda docs list --workspace ws-abc123 --limit 50",
		}, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := api.NewClient()
			if err != nil {
				return err
			}
			query := url.Values{}
			addStringFlag(cmd, query, "query")
			addStringFlag(cmd, query, "workspace")
			if value := query.Get("workspace"); value != "" {
				query.Del("workspace")
				query.Set("workspaceId", value)
			}
			addStringFlag(cmd, query, "folder")
			if value := query.Get("folder"); value != "" {
				query.Del("folder")
				query.Set("folderId", value)
			}
			addBoolFlag(cmd, query, "owned", "isOwner")
			addBoolFlag(cmd, query, "published", "isPublished")
			addBoolFlag(cmd, query, "starred", "isStarred")
			copyStringQueryFlag(cmd, query, "limit")
			copyStringQueryFlag(cmd, query, "page-token", "pageToken")

			all, _ := cmd.Flags().GetBool("all")
			jsonOut, _ := cmd.Flags().GetBool("json")
			if all {
				items, meta, err := paginateItems(cmd.Context(), client, "/docs", query)
				if err != nil {
					return err
				}
				if jsonOut {
					return printJSONMarshal(map[string]any{"items": items, "nextPageToken": meta.NextPageToken})
				}
				return printDocTable(items)
			}

			body, _, _, err := client.Request(cmd.Context(), http.MethodGet, "/docs", query, nil)
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSON(body)
			}
			return printDocTableFromBody(body)
		},
	}
	listCmd.Flags().String("query", "", "Filter docs by title search query")
	listCmd.Flags().String("workspace", "", "Filter by workspace ID")
	listCmd.Flags().String("folder", "", "Filter by folder ID")
	listCmd.Flags().Bool("owned", false, "Only return docs you own")
	listCmd.Flags().Bool("published", false, "Only return published docs")
	listCmd.Flags().Bool("starred", false, "Only return starred docs")
	listCmd.Flags().Int("limit", 25, "Number of docs to return per page (max 25)")
	listCmd.Flags().String("page-token", "", "Token to fetch the next page of results")
	listCmd.Flags().Bool("all", false, "Fetch all pages and return every doc")
	listCmd.Flags().Bool("json", false, "Print raw JSON instead of a table")
	cmd.AddCommand(listCmd)

	cmd.AddCommand(&cobra.Command{
		Use:   "get <doc>",
		Short: "Get doc metadata",
		Long:  "Print the full metadata for a single Coda document as JSON.",
		Args:  exactArgsFor("coda docs get <doc>", 1),
		Example: strings.Join([]string{
			"  coda docs get AbCDeFGH",
		}, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSimpleGet(cmd.Context(), "/docs/"+escapeSegment(args[0]))
		},
	})

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a doc",
		Long: `Create a new Coda document.

At least one of --title or --source-doc must be provided unless --input is used.
Use --input to pass a full JSON request body matching the Coda API schema.`,
		Example: strings.Join([]string{
			"  coda docs create --title \"Launch Tracker\"",
			"  coda docs create --title \"Copy\" --source-doc AbCDeFGH",
			"  coda docs create --title \"Notes\" --folder fld-xyz --page-name \"Home\"",
			"  coda docs create --input doc-create.json",
		}, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := api.NewClient()
			if err != nil {
				return err
			}
			payload, err := docCreatePayload(cmd)
			if err != nil {
				return err
			}

			body, _, _, err := client.Request(cmd.Context(), http.MethodPost, "/docs", nil, payload)
			if err != nil {
				return err
			}
			return printJSON(body)
		},
	}
	createCmd.Flags().String("input", "", "Path to a JSON request body (uses Coda API schema directly)")
	createCmd.Flags().String("title", "", "Title of the new doc")
	createCmd.Flags().String("source-doc", "", "Doc ID to copy as a template")
	createCmd.Flags().String("timezone", "", "Timezone for the doc (e.g. America/New_York)")
	createCmd.Flags().String("folder", "", "Folder ID to create the doc in")
	createCmd.Flags().String("page-name", "", "Name of the initial page")
	createCmd.Flags().String("page-subtitle", "", "Subtitle of the initial page")
	createCmd.Flags().String("content", "", "HTML content for the initial page")
	cmd.AddCommand(createCmd)

	updateCmd := &cobra.Command{
		Use:   "update <doc>",
		Short: "Update a doc",
		Long: `Update the title or icon of a Coda document.

At least one flag must be provided unless --input is used.`,
		Args: exactArgsFor("coda docs update <doc>", 1),
		Example: strings.Join([]string{
			"  coda docs update AbCDeFGH --title \"New Title\"",
			"  coda docs update AbCDeFGH --icon document",
			"  coda docs update AbCDeFGH --input doc-update.json",
		}, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := api.NewClient()
			if err != nil {
				return err
			}
			payload, err := inputOrPayload(cmd, docUpdatePayload)
			if err != nil {
				return err
			}
			body, _, _, err := client.Request(cmd.Context(), http.MethodPatch, "/docs/"+escapeSegment(args[0]), nil, payload)
			if err != nil {
				return err
			}
			return printJSON(body)
		},
	}
	updateCmd.Flags().String("input", "", "Path to a JSON request body (uses Coda API schema directly)")
	updateCmd.Flags().String("title", "", "New title for the doc")
	updateCmd.Flags().String("icon", "", "New icon name for the doc")
	cmd.AddCommand(updateCmd)

	deleteCmd := &cobra.Command{
		Use:   "delete <doc>",
		Short: "Delete a doc",
		Long: `Permanently delete a Coda document.

Prompts for confirmation unless --yes is provided. Use --wait to block
until the deletion mutation completes.`,
		Args:    exactArgsFor("coda docs delete <doc>", 1),
		Example: "  coda docs delete AbCDeFGH --yes --wait",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDelete(cmd, "/docs/"+escapeSegment(args[0]), "doc", args[0])
		},
	}
	addConfirmFlag(deleteCmd)
	addWaitFlags(deleteCmd)
	cmd.AddCommand(deleteCmd)

	return cmd
}
