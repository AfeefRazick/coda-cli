package main

import (
	"os"

	"github.com/AfeefRazick/coda-cli/internal/cmd"
)

func main() {
	if err := cmd.NewRootCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
