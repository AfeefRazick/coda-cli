package cmd

import (
	"errors"
	"net/http"
	"strings"

	"github.com/AfeefRazick/coda-cli/internal/api"
	"github.com/spf13/cobra"
)

func newAPICommand() *cobra.Command {
	var fields stringSliceFlag
	var input string
	var method string
	var paginate bool

	cmd := &cobra.Command{
		Use:   "api <path>",
		Short: "Call a raw Coda API endpoint",
		Long: `Send a request to any Coda REST API endpoint and print the response as JSON.

This is an escape hatch for endpoints not yet covered by a dedicated command.
Paths are relative to https://coda.io/apis/v1 (e.g. /docs, /whoami).
Absolute URLs are also accepted.

Use --field key=value to build a JSON request body, or --input to pass a
JSON file. Use --paginate to automatically follow nextPageToken on GET
requests and return all items in a single response.`,
		Example: strings.Join([]string{
			"  coda api /docs --paginate",
			"  coda api /whoami",
			"  coda api /docs/<doc-id> --method PATCH --field title='New Title'",
			"  coda api /docs --method POST --input doc-create.json",
		}, "\n"),
		Args: exactArgsFor("coda api <path>", 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := api.NewClient()
			if err != nil {
				return err
			}

			body, err := bodyFromFlags(fields, input)
			if err != nil {
				return err
			}

			verb := strings.ToUpper(strings.TrimSpace(method))
			if verb == "" {
				verb = http.MethodGet
			}

			if !paginate {
				respBody, _, _, err := client.Request(cmd.Context(), verb, args[0], nil, body)
				if err != nil {
					return err
				}
				return printJSON(respBody)
			}

			if verb != http.MethodGet {
				return errors.New("--paginate only supports GET requests")
			}

			items, meta, err := paginateItems(cmd.Context(), client, args[0], nil)
			if err != nil {
				return err
			}
			return printJSONMarshal(map[string]any{
				"items":         items,
				"nextPageToken": meta.NextPageToken,
				"nextPageLink":  meta.NextPageLink,
				"href":          meta.Href,
			})
		},
	}

	cmd.Flags().Var(&fields, "field", "Add a JSON body field as key=value (repeatable); cannot be combined with --input")
	cmd.Flags().StringVar(&input, "input", "", "Path to a JSON file to use as the request body; cannot be combined with --field")
	cmd.Flags().StringVar(&method, "method", http.MethodGet, "HTTP method (GET, POST, PUT, PATCH, DELETE)")
	cmd.Flags().BoolVar(&paginate, "paginate", false, "Follow nextPageToken and return all items (GET only)")
	return cmd
}
