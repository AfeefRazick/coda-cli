package cmd

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/AfeefRazick/coda-cli/internal/api"
	"github.com/AfeefRazick/coda-cli/internal/auth"
	"github.com/spf13/cobra"
)

func newAuthCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage Coda authentication",
		Long:  "Log in, log out, and check the status of your Coda API token.",
	}

	loginCmd := &cobra.Command{
		Use:   "login",
		Short: "Save a Coda API token",
		Long: `Save a Coda API token to the local config file.

The token is written to ~/.config/coda-cli/config.yaml with 0600
permissions. Generate a token at https://coda.io/account under API settings.

If CODA_API_TOKEN is set in your environment it takes precedence over
the saved token and this command has no effect on that session.`,
		Example: strings.Join([]string{
			"  coda auth login                          # interactive prompt",
			"  coda auth login --token YOUR_TOKEN       # pass token directly",
			"  echo $TOKEN | coda auth login --with-token-stdin",
		}, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			token, _ := cmd.Flags().GetString("token")
			stdinToken, _ := cmd.Flags().GetBool("with-token-stdin")

			var err error
			switch {
			case token != "":
			case stdinToken:
				data, readErr := io.ReadAll(os.Stdin)
				if readErr != nil {
					return readErr
				}
				token = strings.TrimSpace(string(data))
			default:
				token, err = promptForToken()
				if err != nil {
					return err
				}
			}

			if token == "" {
				return errors.New("empty token")
			}

			path, err := auth.SaveAuthToken(token)
			if err != nil {
				return err
			}

			fmt.Printf("Saved token to %s\n", path)
			fmt.Printf("%s takes precedence when set\n", auth.TokenEnvVar)
			return nil
		},
	}
	loginCmd.Flags().String("token", "", "Coda API token to save (skips interactive prompt)")
	loginCmd.Flags().Bool("with-token-stdin", false, "Read the Coda API token from stdin")

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show current Coda auth status",
		Long: `Show the active Coda API token and verify it against the API.

Displays the token source (environment variable or config file), a masked
version of the token, and the authenticated user's name and login.`,
		Example: "  coda auth status",
		RunE: func(cmd *cobra.Command, args []string) error {
			token, source, err := auth.ResolveToken()
			if err != nil {
				return err
			}
			if token == "" {
				fmt.Printf("Not authenticated. Set %s or run 'coda auth login'.\n", auth.TokenEnvVar)
				return nil
			}

			fmt.Printf("Authenticated via %s\n", source)
			fmt.Printf("Token: %s\n", auth.MaskToken(token))

			client, _, err := api.NewClient()
			if err != nil {
				return err
			}

			body, _, _, err := client.Request(cmd.Context(), http.MethodGet, "/whoami", nil, nil)
			if err != nil {
				fmt.Printf("Unable to verify token: %v\n", err)
				return nil
			}

			var me codaUser
			if err := unmarshalJSON(body, &me); err == nil {
				fmt.Printf("User: %s\n", firstNonEmpty(me.Name, me.LoginId))
				if me.LoginId != "" {
					fmt.Printf("Login: %s\n", me.LoginId)
				}
				if me.TokenName != "" {
					fmt.Printf("Token name: %s\n", me.TokenName)
				}
			}

			return nil
		},
	}

	logoutCmd := &cobra.Command{
		Use:   "logout",
		Short: "Remove saved Coda auth",
		Long: `Remove the saved Coda API token from the local config file.

This only removes the token stored by 'coda auth login'. If CODA_API_TOKEN
is set in your environment it will still be used after logout.`,
		Example: "  coda auth logout",
		RunE: func(cmd *cobra.Command, args []string) error {
			removed, path, err := auth.DeleteAuthToken()
			if err != nil {
				return err
			}
			if !removed {
				fmt.Printf("No saved token found at %s\n", path)
				return nil
			}
			fmt.Printf("Removed saved token at %s\n", path)
			return nil
		},
	}

	cmd.AddCommand(loginCmd, statusCmd, logoutCmd)
	return cmd
}
