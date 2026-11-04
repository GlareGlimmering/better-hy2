package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"h-ui/model/constant"
	"h-ui/util"
)

var installHysteria2Cmd = &cobra.Command{
	Use:    "install-hysteria2",
	Short:  "Install the pinned H-UI Hysteria2 kernel",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if dataDir := os.Getenv("HUI_DATA"); dataDir != "" {
			if err := os.Chdir(dataDir); err != nil {
				return err
			}
		}
		if err := util.DownloadHysteria2(constant.Hysteria2CustomVersion); err != nil {
			return err
		}
		version, err := util.InstalledHysteria2Version()
		if err != nil {
			return err
		}
		fmt.Println("Hysteria2 installed:", version)
		return nil
	},
}

func init() { rootCmd.AddCommand(installHysteria2Cmd) }
