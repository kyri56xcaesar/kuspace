//go:build windows
// +build windows

// Code specifics that run only on windows
// For Windows systems\

// Package utils includes the windows specific utils
package utils

import "golang.org/x/sys/windows"

// GetAvailableSpace windows specific function to retrieve the system available free disk space
func GetAvailableSpace(path string) (uint64, error) {
	// Convert path to UTF-16 as required by Windows API
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}

	var freeBytesAvailable, totalBytes, totalFreeBytes uint64

	// Call Windows API to get disk space information
	err = windows.GetDiskFreeSpaceEx(
		pathPtr,
		&freeBytesAvailable,
		&totalBytes,
		&totalFreeBytes,
	)
	if err != nil {
		return 0, err
	}

	return freeBytesAvailable, nil
}
