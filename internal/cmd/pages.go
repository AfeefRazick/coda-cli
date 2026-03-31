package cmd

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/AfeefRazick/coda-cli/internal/api"
	"github.com/spf13/cobra"
)

func newPagesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pages",
		Short: "Manage Coda pages",
		Long:  "List, get, create, update, and delete pages within a Coda document.",
	}

	listCmd := &cobra.Command{
		Use:   "list <doc>",
		Short: "List pages in a doc",
		Long: `List all pages in a Coda document.

Results are printed as a table by default. Use --json for raw API output.
Use --all to automatically paginate and return every page.`,
		Args: exactArgsFor("coda pages list <doc>", 1),
		Example: strings.Join([]string{
			"  coda pages list AbCDeFGH",
			"  coda pages list AbCDeFGH --all --json",
		}, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := api.NewClient()
			if err != nil {
				return err
			}
			query := url.Values{}
			copyStringQueryFlag(cmd, query, "limit")
			copyStringQueryFlag(cmd, query, "page-token", "pageToken")
			all, _ := cmd.Flags().GetBool("all")
			jsonOut, _ := cmd.Flags().GetBool("json")
			path := "/docs/" + escapeSegment(args[0]) + "/pages"
			if all {
				items, meta, err := paginateItems(cmd.Context(), client, path, query)
				if err != nil {
					return err
				}
				if jsonOut {
					return printJSONMarshal(map[string]any{"items": items, "nextPageToken": meta.NextPageToken})
				}
				return printPageTable(items)
			}
			body, _, _, err := client.Request(cmd.Context(), http.MethodGet, path, query, nil)
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSON(body)
			}
			return printPageTableFromBody(body)
		},
	}
	listCmd.Flags().Int("limit", 25, "Number of pages to return per page (max 25)")
	listCmd.Flags().String("page-token", "", "Token to fetch the next page of results")
	listCmd.Flags().Bool("all", false, "Fetch all pages and return every result")
	listCmd.Flags().Bool("json", false, "Print raw JSON instead of a table")
	cmd.AddCommand(listCmd)

	cmd.AddCommand(&cobra.Command{
		Use:     "get <doc> <page>",
		Short:   "Get a page",
		Long:    "Print the full metadata for a single page as JSON.",
		Args:    exactArgsFor("coda pages get <doc> <page>", 2),
		Example: "  coda pages get AbCDeFGH canvas-tuVwxYz",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSimpleGet(cmd.Context(), "/docs/"+escapeSegment(args[0])+"/pages/"+escapeSegment(args[1]))
		},
	})

	contentCmd := &cobra.Command{
		Use:   "content <doc> <page>",
		Short: "Read page content",
		Long:  "Export the content of a Coda page in the specified format.",
		Args:  exactArgsFor("coda pages content <doc> <page>", 2),
		Example: strings.Join([]string{
			"  coda pages content AbCDeFGH canvas-tuVwxYz",
			"  coda pages content AbCDeFGH canvas-tuVwxYz --format markdown",
		}, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := api.NewClient()
			if err != nil {
				return err
			}
			query := url.Values{}
			copyStringQueryFlag(cmd, query, "limit")
			copyStringQueryFlag(cmd, query, "page-token", "pageToken")
			copyStringQueryFlag(cmd, query, "format", "contentFormat")
			body, _, _, err := client.Request(cmd.Context(), http.MethodGet, "/docs/"+escapeSegment(args[0])+"/pages/"+escapeSegment(args[1])+"/content", query, nil)
			if err != nil {
				return err
			}
			return printJSON(body)
		},
	}
	contentCmd.Flags().Int("limit", 25, "Number of content items to return per page")
	contentCmd.Flags().String("page-token", "", "Token to fetch the next page of content")
	contentCmd.Flags().String("format", "plainText", "Content export format: plainText, markdown, html")
	cmd.AddCommand(contentCmd)

	createCmd := &cobra.Command{
		Use:   "create <doc>",
		Short: "Create a page",
		Long: `Create a new page in a Coda document.

At least one of --name, --content, or --embed-url must be provided
unless --input is used. Use --wait to block until the page is created.`,
		Args: exactArgsFor("coda pages create <doc>", 1),
		Example: strings.Join([]string{
			"  coda pages create AbCDeFGH --name Roadmap",
			"  coda pages create AbCDeFGH --name Roadmap --content '<p>Hello</p>' --wait",
			"  coda pages create AbCDeFGH --name Embed --embed-url https://example.com",
			"  coda pages create AbCDeFGH --input page-create.json",
		}, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := api.NewClient()
			if err != nil {
				return err
			}
			payload, err := inputOrPayload(cmd, pageCreatePayload)
			if err != nil {
				return err
			}
			body, _, _, err := client.Request(cmd.Context(), http.MethodPost, "/docs/"+escapeSegment(args[0])+"/pages", nil, payload)
			if err != nil {
				return err
			}
			return maybeWaitAndPrint(cmd, client, body)
		},
	}
	createCmd.Flags().String("input", "", "Path to a JSON request body (uses Coda API schema directly)")
	addPageCreateFlags(createCmd)
	addWaitFlags(createCmd)
	cmd.AddCommand(createCmd)

	updateCmd := &cobra.Command{
		Use:   "update <doc> <page>",
		Short: "Update a page",
		Long: `Update the name, subtitle, or content of a Coda page.

At least one flag must be provided unless --input is used. Use --wait
to block until the update mutation completes.`,
		Args: exactArgsFor("coda pages update <doc> <page>", 2),
		Example: strings.Join([]string{
			"  coda pages update AbCDeFGH canvas-tuVwxYz --name \"New Name\"",
			"  coda pages update AbCDeFGH canvas-tuVwxYz --content '<p>Updated</p>' --wait",
			"  coda pages update AbCDeFGH canvas-tuVwxYz --hidden",
			"  coda pages update AbCDeFGH canvas-tuVwxYz --input page-update.json",
		}, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := api.NewClient()
			if err != nil {
				return err
			}
			payload, err := inputOrPayload(cmd, pageUpdatePayload)
			if err != nil {
				return err
			}
			body, _, _, err := client.Request(cmd.Context(), http.MethodPut, "/docs/"+escapeSegment(args[0])+"/pages/"+escapeSegment(args[1]), nil, payload)
			if err != nil {
				return err
			}
			return maybeWaitAndPrint(cmd, client, body)
		},
	}
	updateCmd.Flags().String("input", "", "Path to a JSON request body (uses Coda API schema directly)")
	addPageUpdateFlags(updateCmd)
	addWaitFlags(updateCmd)
	cmd.AddCommand(updateCmd)

	deleteCmd := &cobra.Command{
		Use:   "delete <doc> <page>",
		Short: "Delete a page",
		Long: `Permanently delete a page from a Coda document.

Prompts for confirmation unless --yes is provided. Use --wait to block
until the deletion mutation completes.`,
		Args:    exactArgsFor("coda pages delete <doc> <page>", 2),
		Example: "  coda pages delete AbCDeFGH canvas-tuVwxYz --yes --wait",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDelete(cmd, "/docs/"+escapeSegment(args[0])+"/pages/"+escapeSegment(args[1]), "page", args[1])
		},
	}
	addConfirmFlag(deleteCmd)
	addWaitFlags(deleteCmd)
	cmd.AddCommand(deleteCmd)

	deleteContentCmd := &cobra.Command{
		Use:   "delete-content <doc> <page>",
		Short: "Delete page content",
		Long: `Delete all content or specific elements from a Coda page.

Without --element-id, deletes all content on the page. With one or more
--element-id flags, only those elements are removed. Prompts for
confirmation unless --yes is provided.`,
		Args: exactArgsFor("coda pages delete-content <doc> <page>", 2),
		Example: strings.Join([]string{
			"  coda pages delete-content AbCDeFGH canvas-tuVwxYz --yes",
			"  coda pages delete-content AbCDeFGH canvas-tuVwxYz --element-id cl-123 --element-id cl-456 --yes",
			"  coda pages delete-content AbCDeFGH canvas-tuVwxYz --input delete-content.json --yes",
		}, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := confirmDestructive(cmd, "page content", args[1]); err != nil {
				return err
			}
			client, _, err := api.NewClient()
			if err != nil {
				return err
			}
			payload, err := pageDeleteContentPayload(cmd)
			if err != nil {
				return err
			}
			if input, _ := cmd.Flags().GetString("input"); input != "" {
				payload, err = inputOrPayload(cmd, pageDeleteContentPayload)
				if err != nil {
					return err
				}
			}
			body, _, _, err := client.Request(cmd.Context(), http.MethodDelete, "/docs/"+escapeSegment(args[0])+"/pages/"+escapeSegment(args[1])+"/content", nil, payload)
			if err != nil {
				return err
			}
			return maybeWaitAndPrint(cmd, client, body)
		},
	}
	deleteContentCmd.Flags().String("input", "", "Path to a JSON request body (uses Coda API schema directly)")
	deleteContentCmd.Flags().StringArray("element-id", nil, "ID of a page element to delete (repeatable; omit to delete all content)")
	addConfirmFlag(deleteContentCmd)
	addWaitFlags(deleteContentCmd)
	cmd.AddCommand(deleteContentCmd)

	return cmd
}
