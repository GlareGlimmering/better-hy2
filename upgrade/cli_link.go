package upgrade

import (
	"fmt"
	"os"
	"path/filepath"
)

func validateCLILink(link, target string) error {
	_, err := os.Lstat(link)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	existing, err := os.Readlink(link)
	if err == nil && filepath.Clean(existing) == filepath.Clean(target) {
		return nil
	}
	return fmt.Errorf("refusing to overwrite an existing unrelated hui command: %s", link)
}

// EnsureCLILink keeps the public CLI pointing at the stable legacy binary path,
// so atomic upgrades and rollbacks do not invalidate the command.
func EnsureCLILink(link, target string) error {
	if err := validateCLILink(link, target); err != nil {
		return err
	}
	if _, err := os.Lstat(link); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(link), 0755); err != nil {
		return err
	}
	return os.Symlink(target, link)
}
