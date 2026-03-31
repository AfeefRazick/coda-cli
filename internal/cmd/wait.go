package cmd

import (
	"context"
	"strings"
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
		Long: `Poll the Coda mutation status endpoint until the request completes.

Coda mutations (create, update, delete) return a requestId that can be
polled to check completion. This command blocks until the mutation is done
or the timeout is reached, then prints the final status as JSON.

Use --wait on individual commands (e.g. coda rows insert --wait) to do
this automatically after a mutation.`,
		Example: strings.Join([]string{
			"  coda wait abc-123",
			"  coda wait abc-123 --timeout 5m --interval 5s",
		}, "\n"),
		Args: exactArgsFor("coda wait <request-id>", 1),
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

	cmd.Flags().DurationVar(&timeout, "timeout", 2*time.Minute, "Maximum time to wait before giving up")
	cmd.Flags().DurationVar(&interval, "interval", 2*time.Second, "How often to poll the mutation status")
	return cmd
}
