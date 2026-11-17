package cmd

import (
	"h-ui/upgrade"

	"github.com/spf13/cobra"
)

func init() {
	var purge bool
	command := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove Better HY2, its managed services, and standard installation data",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return upgrade.Uninstall(cmd.Context(), purge, cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
	command.Flags().BoolVar(&purge, "purge", false, "compatibility alias for full uninstall (confirmation: PURGE)")
	rootCmd.AddCommand(command)
}
