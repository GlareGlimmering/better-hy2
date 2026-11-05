//go:build !windows

package upgrade

import (
	"os"
	"syscall"
)

func fileOwnership(info os.FileInfo) (int, int) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return -1, -1
	}
	return int(stat.Uid), int(stat.Gid)
}

func restoreFileMetadata(path string, metadata fileMetadata) error {
	if err := os.Chmod(path, os.FileMode(metadata.Mode)); err != nil {
		return err
	}
	if metadata.UID >= 0 && metadata.GID >= 0 {
		return os.Chown(path, metadata.UID, metadata.GID)
	}
	return nil
}
