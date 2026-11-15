package cmd

import (
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"h-ui/dao"
	"h-ui/util"
	"os"
)

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset username and password",
	Long:  "Reset username and password.",
	Run:   runReset,
}

func init() {
	rootCmd.AddCommand(resetCmd)
}

func runReset(cmd *cobra.Command, args []string) {
	username, password, err := resetAdminCredentials()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	fmt.Printf("Better HY2 Login Username: %s\n", username)
	fmt.Printf("Better HY2 Login Password: %s\n", password)
}

func resetAdminCredentials() (string, string, error) {
	if err := ensureCLIDataRoot(); err != nil {
		return "", "", err
	}
	username, err := util.RandomString(6)
	if err != nil {
		return "", "", err
	}
	password, err := util.RandomString(24)
	if err != nil {
		return "", "", err
	}
	hash, err := util.HashPanelPassword(password)
	if err != nil {
		return "", "", err
	}
	if err = dao.InitSqliteSchema(); err != nil {
		return "", "", err
	}
	username, err = dao.ResetPanelAdmin(username, hash)
	if err = errors.Join(err, dao.CloseSqliteDB()); err != nil {
		return "", "", err
	}
	return username, password, nil
}
