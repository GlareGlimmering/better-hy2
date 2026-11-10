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
	upgradeChannel    string
	upgradeYes        bool
	upgradeHealthURL  string
)

var upgradeCmd = &cobra.Command{
	Use:     "upgrade [latest|version]",
	Aliases: []string{"update"},
	Short:   "Safely update Better HY2 with verification, backup, health checks, and rollback",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := upgradeop.DefaultOptions()
		opts.CurrentVersion = installedVersion(opts.InstallDir, "unknown")
		switch upgradeChannel {
		case "stable":
			if cmd.Flags().Changed("channel") && upgradePrerelease {
				return fmt.Errorf("--prerelease conflicts with --channel stable")
			}
		case "prerelease":
		default:
			return fmt.Errorf("unknown update channel %q; use stable or prerelease", upgradeChannel)
		}
		opts.Prerelease = upgradePrerelease || upgradeChannel == "prerelease"
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
	version = strings.TrimPrefix(version, "Better HY2 version ")
	if version == "" {
		return fallback
	}
	return version
}

func init() {
	upgradeCmd.Flags().StringVar(&upgradeChannel, "channel", "stable", "release channel for automatic updates (stable or prerelease)")
	upgradeCmd.Flags().BoolVar(&upgradePrerelease, "prerelease", false, "use the latest prerelease channel")
	upgradeCmd.Flags().BoolVarP(&upgradeYes, "yes", "y", false, "skip the confirmation prompt")
	upgradeCmd.Flags().StringVar(&upgradeHealthURL, "health-url", "", "override the loopback panel health URL")
	rootCmd.AddCommand(upgradeCmd)
}
