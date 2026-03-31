package cmd

import (
	"context"
	"time"

	"github.com/AfeefRazick/coda-cli/internal/api"
	"github.com/spf13/cobra"
)

func newWaitCommand() *cobra.Command {
	var timeout time.Duration
	var interval time.Duration

	cmd := &cobra.Command{
		Use:   "wait <request-id>",
		Short: "Wait for an async Coda mutation to complete",
		Args:  exactArgsFor("coda wait <request-id>", 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := api.NewClient()
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()

			status, err := client.WaitForMutation(ctx, args[0], interval)
			if err != nil {
				return err
			}
			return printJSONMarshal(status)
		},
	}

	cmd.Flags().DurationVar(&timeout, "timeout", 2*time.Minute, "Maximum time to wait")
	cmd.Flags().DurationVar(&interval, "interval", 2*time.Second, "Polling interval")
	return cmd
}
