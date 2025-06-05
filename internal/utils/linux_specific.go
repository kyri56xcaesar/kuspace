//go:build linux
// +build linux

// Code specifics that run only on linux
// For Unix-based systems (Linux, macOS)

// Package utils includes the linux specific utils
package utils

import "syscall"

// GetAvailableSpace Linux specific function to retrieve the system available free disk space
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
