//go:build linux || darwin

package main

import "syscall"

// freeDiskBytes reports the space available at path to an unprivileged writer.
// The conversion compiles for Statfs_t's block size on Linux and Darwin alike.
func freeDiskBytes(path string) (uint64, error) {
	var stat syscall.Statfs_t

	err := syscall.Statfs(path, &stat)
	if err != nil {
		return 0, err
	}

	return stat.Bavail * uint64(stat.Bsize), nil
}
