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
		Args:  exactArgsFor("coda api <path>", 1),
		Example: strings.Join([]string{
			"  coda api /docs --paginate",
			"  coda api /docs/<doc-id> --method PATCH --field title='New Title'",
		}, "\n"),
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
				return errors.New("--paginate only supports GET")
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

	cmd.Flags().Var(&fields, "field", "Add a JSON field as key=value (repeatable)")
	cmd.Flags().StringVar(&input, "input", "", "Path to a JSON file for the request body")
	cmd.Flags().StringVar(&method, "method", http.MethodGet, "HTTP method to use")
	cmd.Flags().BoolVar(&paginate, "paginate", false, "Fetch all pages for list endpoints")
	return cmd
}
