package cmd

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/AfeefRazick/coda-cli/internal/api"
	"github.com/spf13/cobra"
)

func newDocsCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "docs", Short: "Manage Coda docs"}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List docs",
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
	listCmd.Flags().String("query", "", "Search query")
	listCmd.Flags().String("workspace", "", "Workspace ID filter")
	listCmd.Flags().String("folder", "", "Folder ID filter")
	listCmd.Flags().Bool("owned", false, "Only docs you own")
	listCmd.Flags().Bool("published", false, "Only published docs")
	listCmd.Flags().Bool("starred", false, "Only starred docs")
	listCmd.Flags().Int("limit", 25, "Maximum number of docs to request")
	listCmd.Flags().String("page-token", "", "Pagination token")
	listCmd.Flags().Bool("all", false, "Fetch all pages")
	listCmd.Flags().Bool("json", false, "Print raw JSON")
	cmd.AddCommand(listCmd)

	cmd.AddCommand(&cobra.Command{
		Use:     "get <doc>",
		Short:   "Get doc metadata",
		Args:    exactArgsFor("coda docs get <doc>", 1),
		Example: "  coda docs get AbCDeFGH",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSimpleGet(cmd.Context(), "/docs/"+escapeSegment(args[0]))
		},
	})

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a doc",
		Example: strings.Join([]string{
			"  coda docs create --title \"Launch Tracker\"",
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
	createCmd.Flags().String("input", "", "Path to a JSON request body")
	createCmd.Flags().String("title", "", "Doc title")
	createCmd.Flags().String("source-doc", "", "Source doc ID to copy")
	createCmd.Flags().String("timezone", "", "Timezone for the new doc")
	createCmd.Flags().String("folder", "", "Folder ID")
	createCmd.Flags().String("page-name", "", "Initial page name")
	createCmd.Flags().String("page-subtitle", "", "Initial page subtitle")
	createCmd.Flags().String("content", "", "Initial HTML page content")
	cmd.AddCommand(createCmd)

	updateCmd := &cobra.Command{
		Use:   "update <doc>",
		Short: "Update a doc",
		Args:  exactArgsFor("coda docs update <doc>", 1),
		Example: strings.Join([]string{
			"  coda docs update AbCDeFGH --title \"New Title\"",
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
	updateCmd.Flags().String("input", "", "Path to a JSON request body")
	updateCmd.Flags().String("title", "", "New doc title")
	updateCmd.Flags().String("icon", "", "New icon name")
	cmd.AddCommand(updateCmd)

	deleteCmd := &cobra.Command{
		Use:     "delete <doc>",
		Short:   "Delete a doc",
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
