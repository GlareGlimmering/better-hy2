//go:build windows

package upgrade

// Windows is rejected by the platform check before this value is used.
func currentEUID() int { return -1 }
