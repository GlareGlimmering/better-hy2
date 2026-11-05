package cmd

import (
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
	fmt.Printf("Better HY2 Connection Password: %s.%s\n", username, password)
}

func resetAdminCredentials() (string, string, error) {
	if err := ensureCLIDataRoot(); err != nil {
		return "", "", err
	}
	username, err := util.RandomString(6)
	if err != nil {
		return "", "", err
	}
	password, err := util.RandomString(6)
	if err != nil {
		return "", "", err
	}
	if err = dao.InitSqliteSchema(); err != nil {
		return "", "", err
	}
	if err = dao.UpdateAccount([]int64{1}, map[string]interface{}{
		"username": username,
		"pass":     util.SHA224String(password),
		"con_pass": fmt.Sprintf("%s.%s", username, password),
	}); err != nil {
		_ = dao.CloseSqliteDB()
		return "", "", err
	}
	if err = dao.CloseSqliteDB(); err != nil {
		return "", "", err
	}
	return username, password, nil
}
