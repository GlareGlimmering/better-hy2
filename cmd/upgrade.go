package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"h-ui/model/constant"
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
		ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		if err := upgradeop.Run(ctx, opts, upgradeop.DefaultDependencies()); err != nil {
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

// Used by the pinned installer after downloading the candidate and its pair.
var migrateV1Cmd = &cobra.Command{
	Use:    "migrate-v1 <v2-version> <asset-directory>",
	Hidden: true,
	Args:   cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if args[0] != constant.Version {
			return fmt.Errorf("migration must run using the verified target version")
		}
		opts := upgradeop.DefaultOptions()
		opts.Version = args[0]
		ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		return upgradeop.MigrateV1(ctx, args[1], opts, upgradeop.DefaultDependencies())
	},
}

func init() { rootCmd.AddCommand(migrateV1Cmd) }
