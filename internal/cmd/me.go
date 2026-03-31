package cmd

import (
	"encoding/json"
	"net/http"

	"github.com/AfeefRazick/coda-cli/internal/api"
	"github.com/spf13/cobra"
)

func newMeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "me",
		Short: "Show the current Coda user",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := api.NewClient()
			if err != nil {
				return err
			}
			body, _, _, err := client.Request(cmd.Context(), http.MethodGet, "/whoami", nil, nil)
			if err != nil {
				return err
			}
			return printJSON(body)
		},
	}
}

func unmarshalJSON(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
