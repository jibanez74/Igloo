//go:build linux || darwin

package main

import "syscall"

// freeDiskBytes reports the space available at path to an unprivileged writer.
//
// Statfs_t types the block size differently on the two supported platforms
// (int64 on Linux, uint32 on Darwin), but the conversion below compiles on
// both, so this needs no per-platform variant.
func freeDiskBytes(path string) (uint64, error) {
	var stat syscall.Statfs_t

	err := syscall.Statfs(path, &stat)
	if err != nil {
		return 0, err
	}

	return stat.Bavail * uint64(stat.Bsize), nil
}
