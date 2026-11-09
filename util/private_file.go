package util

import (
	"fmt"
	"os"
)

// ProtectPrivateFile tightens permissions on an existing managed file.
func ProtectPrivateFile(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("expected a regular private file: %s", path)
	}
	return os.Chmod(path, 0600)
}
