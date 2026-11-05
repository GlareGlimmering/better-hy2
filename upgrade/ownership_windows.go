//go:build windows

package upgrade

import "os"

func fileOwnership(os.FileInfo) (int, int) { return -1, -1 }

func restoreFileMetadata(path string, metadata fileMetadata) error {
	return os.Chmod(path, os.FileMode(metadata.Mode))
}
