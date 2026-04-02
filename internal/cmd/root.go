package cmd

import "github.com/spf13/cobra"

func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "coda",
		Short: "A CLI for the Coda API",
		Long:  "coda is an open-source command-line tool for interacting with Coda docs, pages, tables, and rows.",
	}

	cmd.AddCommand(newAuthCommand())
	cmd.AddCommand(newAPICommand())
	cmd.AddCommand(newMeCommand())
	cmd.AddCommand(newWaitCommand())
	cmd.AddCommand(newDocsCommand())
	cmd.AddCommand(newPagesCommand())
	cmd.AddCommand(newTablesCommand())
	cmd.AddCommand(newColumnsCommand())
	cmd.AddCommand(newRowsCommand())
	cmd.AddCommand(newExperimentalCommand())

	return cmd
}
