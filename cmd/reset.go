package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"h-ui/dao"
	"h-ui/util"
)

func init() {
	rootCmd.AddCommand(newResetCommand())
}

func newResetCommand() *cobra.Command {
	var username string
	var interactive, passwordStdin bool
	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Recover the local panel administrator",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if interactive && passwordStdin {
				return errors.New("--interactive and --password-stdin cannot be combined")
			}
			var requestedName *string
			if cmd.Flags().Changed("username") {
				requestedName = &username
			}
			if interactive && requestedName == nil {
				name, err := readResetUsername(cmd)
				if err != nil {
					return err
				}
				requestedName = &name
			}
			var password string
			var err error
			switch {
			case interactive:
				password, err = promptResetPassword(cmd)
			case passwordStdin:
				password, err = readResetLine(cmd.InOrStdin(), 128)
			}
			if err != nil {
				return err
			}
			generated := !interactive && !passwordStdin
			name, password, err := applyResetCredentials(requestedName, password, false, generated)
			if err == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Better HY2 Login Username: %s\n", name)
				if generated {
					fmt.Fprintf(cmd.OutOrStdout(), "Better HY2 Login Password: %s\n", password)
				}
			}
			return err
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "set a custom administrator username")
	cmd.Flags().BoolVar(&interactive, "interactive", false, "prompt for custom credentials without echoing the password")
	cmd.Flags().BoolVar(&passwordStdin, "password-stdin", false, "read one custom password from standard input")
	cmd.AddCommand(&cobra.Command{
		Use:   "username [name]",
		Short: "Change the administrator username and preserve the password",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var name string
			var err error
			if len(args) == 1 {
				name = args[0]
			} else {
				name, err = readResetUsername(cmd)
			}
			if err != nil {
				return err
			}
			name, _, err = applyResetCredentials(&name, "", true, false)
			if err == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Better HY2 Login Username: %s\n", name)
			}
			return err
		},
	})
	passwordCmd := &cobra.Command{
		Use:   "password",
		Short: "Change the administrator password and preserve the username",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			stdin, _ := cmd.Flags().GetBool("stdin")
			var password string
			var err error
			if stdin {
				password, err = readResetLine(cmd.InOrStdin(), 128)
			} else {
				password, err = promptResetPassword(cmd)
			}
			if err != nil {
				return err
			}
			name, _, err := applyResetCredentials(nil, password, false, false)
			if err == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Better HY2 Login Username: %s\n", name)
				fmt.Fprintln(cmd.OutOrStdout(), "Better HY2 administrator password changed.")
			}
			return err
		},
	}
	passwordCmd.Flags().Bool("stdin", false, "read one custom password from standard input")
	cmd.AddCommand(passwordCmd)
	return cmd
}

func readResetUsername(cmd *cobra.Command) (string, error) {
	fmt.Fprint(cmd.ErrOrStderr(), "Administrator username: ")
	if file, ok := cmd.InOrStdin().(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		// Read without read-ahead because the next prompt switches the terminal mode.
		var line []byte
		for len(line) <= 128 {
			var ch [1]byte
			if _, err := io.ReadFull(file, ch[:]); err != nil {
				return "", err
			}
			if ch[0] == '\n' {
				return strings.TrimSuffix(string(line), "\r"), nil
			}
			line = append(line, ch[0])
		}
		return "", errors.New("administrator username is too long")
	}
	return readResetLine(cmd.InOrStdin(), 128)
}

func readResetLine(input io.Reader, max int) (string, error) {
	value, err := bufio.NewReader(io.LimitReader(input, int64(max+2))).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	value = strings.TrimSuffix(strings.TrimSuffix(value, "\n"), "\r")
	if len(value) > max {
		return "", errors.New("input is too long")
	}
	return value, nil
}

func promptResetPassword(cmd *cobra.Command) (string, error) {
	file, ok := cmd.InOrStdin().(*os.File)
	if !ok || !term.IsTerminal(int(file.Fd())) {
		return "", errors.New("a terminal is required for the password prompt; use the standard input option for scripts")
	}
	read := func(prompt string) ([]byte, error) {
		fmt.Fprint(cmd.ErrOrStderr(), prompt)
		password, err := term.ReadPassword(int(file.Fd()))
		fmt.Fprintln(cmd.ErrOrStderr())
		return password, err
	}
	password, err := read("New administrator password: ")
	if err != nil {
		return "", err
	}
	confirmation, err := read("Confirm new administrator password: ")
	if err != nil {
		return "", err
	}
	if string(password) != string(confirmation) {
		return "", errors.New("passwords do not match")
	}
	return string(password), nil
}

func resetAdminCredentials() (string, string, error) {
	return applyResetCredentials(nil, "", false, true)
}

func applyResetCredentials(username *string, password string, usernameOnly, generate bool) (string, string, error) {
	if username != nil && (*username == "" || len(*username) > 128 || strings.TrimSpace(*username) != *username ||
		strings.ContainsFunc(*username, unicode.IsControl)) {
		return "", "", errors.New("administrator username must be 1 to 128 bytes without surrounding whitespace or control characters")
	}
	var hash *string
	if !usernameOnly {
		if generate {
			generated, err := util.RandomString(24)
			if err != nil {
				return "", "", err
			}
			password = generated
		}
		if len(password) < 12 || len(password) > 128 {
			return "", "", errors.New("administrator password must be 12 to 128 bytes")
		}
		encoded, err := util.HashPanelPassword(password)
		if err != nil {
			return "", "", err
		}
		hash = &encoded
	}
	if err := ensureCLIDataRoot(); err != nil {
		return "", "", err
	}
	if err := dao.InitSqliteSchema(); err != nil {
		return "", "", err
	}
	name, err := dao.UpdatePanelAdminCredentials(username, hash)
	if err = errors.Join(err, dao.CloseSqliteDB()); err != nil {
		return "", "", err
	}
	return name, password, nil
}
