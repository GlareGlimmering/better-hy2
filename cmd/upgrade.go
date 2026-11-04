package cmd

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	upgradeop "h-ui/upgrade"
)

var (
	upgradePrerelease bool
	upgradeYes        bool
	upgradeHealthURL  string
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade [latest|version]",
	Short: "Safely upgrade h-ui with verification, backup, health checks, and rollback",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := upgradeop.DefaultOptions()
		opts.CurrentVersion = installedVersion(opts.InstallDir, "unknown")
		opts.Prerelease = upgradePrerelease
		opts.Yes = upgradeYes
		opts.HealthURL = upgradeHealthURL
		if len(args) == 1 {
			opts.Version = args[0]
		}
		if err := upgradeop.Run(context.Background(), opts, upgradeop.DefaultDependencies()); err != nil {
			return fmt.Errorf("upgrade: %w", err)
		}
		return nil
	},
}

func installedVersion(installDir, fallback string) string {
	output, err := exec.Command(filepath.Join(installDir, "h-ui"), "version").CombinedOutput()
	if err != nil {
		return fallback
	}
	version := strings.TrimSpace(string(output))
	version = strings.TrimPrefix(version, "h-ui version ")
	if version == "" {
		return fallback
	}
	return version
}

func init() {
	upgradeCmd.Flags().BoolVar(&upgradePrerelease, "prerelease", false, "use the latest prerelease channel")
	upgradeCmd.Flags().BoolVarP(&upgradeYes, "yes", "y", false, "skip the confirmation prompt")
	upgradeCmd.Flags().StringVar(&upgradeHealthURL, "health-url", "", "override the loopback panel health URL")
	rootCmd.AddCommand(upgradeCmd)
}
