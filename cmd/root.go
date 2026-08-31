package cmd

import (
	"crypto/sha512"

	"github.com/spf13/cobra"
)

var HASHER = sha512.New()

func RootCmd() *cobra.Command {
	command := &cobra.Command{
		Use:   "td [command]",
		Short: "Download files",
	}

	command.AddCommand(DownloadCmd(), DedupeCmd())
	return command
}
