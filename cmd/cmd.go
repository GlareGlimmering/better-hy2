package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"h-ui/model/constant"
	"h-ui/util"
	"os"
	"path/filepath"
)

var rootCmd = &cobra.Command{
	Use:   "hui",
	Short: "Better HY2 — a lightweight, modern management panel for Hysteria2",
	Long:  "Better HY2 — a lightweight, modern management panel for Hysteria2. The legacy h-ui command remains supported.",
	Run:   run,
}

var (
	version          bool
	port             string
	panelBindAddress string
	executablePath   = os.Executable
)

func ensureCLIDataRoot() error {
	if os.Getenv("HUI_DATA") != "" {
		return nil
	}
	executable, err := executablePath()
	if err != nil {
		return fmt.Errorf("resolve h-ui executable path: %w", err)
	}
	if resolved, resolveErr := filepath.EvalSymlinks(executable); resolveErr == nil {
		executable = resolved
	}
	return os.Setenv("HUI_DATA", filepath.Dir(executable))
}

func init() {
	rootCmd.Flags().BoolVarP(&version, "version", "v", false, "Show version")
	rootCmd.Flags().StringVarP(&port, "port", "p", "", "The port of the web server")
	rootCmd.Flags().StringVar(&panelBindAddress, "bind", constant.PanelBindAddress, "The bind address of the web server")
}

func run(cmd *cobra.Command, args []string) {
	if version {
		fmt.Println("Better HY2 version", constant.Version)
		return
	}

	if err := util.VerifyPort(port); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	for {
		if err := runServer(port, panelBindAddress); err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
	}
}

func Execute() {
	if err := ensureCLIDataRoot(); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
