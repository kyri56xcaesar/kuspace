//go:build linux
// +build linux

package utils

import "syscall"

// For Unix-based systems (Linux, macOS)
func GetAvailableSpace(path string) (uint64, error) {
	var stat syscall.Statfs_t

	// Get filesystem stats for the given path
	err := syscall.Statfs(path, &stat)
	if err != nil {
		return 0, err
	}

	// Calculate available space in bytes
	availableSpace := stat.Bavail * uint64(stat.Bsize)
	return availableSpace, nil
}
