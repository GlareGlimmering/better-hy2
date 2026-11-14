package cmd

import (
	"h-ui/upgrade"

	"github.com/spf13/cobra"
)

func init() {
	var purge bool
	command := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove a standard Better HY2 systemd installation",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return upgrade.Uninstall(cmd.Context(), purge, cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
	command.Flags().BoolVar(&purge, "purge", false, "also remove panel data and backups")
	rootCmd.AddCommand(command)
}
