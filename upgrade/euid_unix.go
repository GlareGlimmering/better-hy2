//go:build !windows

package upgrade

import "os"

func currentEUID() int { return os.Geteuid() }
