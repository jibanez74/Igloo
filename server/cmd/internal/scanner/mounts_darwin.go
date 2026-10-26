package scanner

import "syscall"

// mntNoWait is MNT_NOWAIT from <sys/mount.h>, which the syscall package does
// not export on darwin.
const mntNoWait = 2

// Mounts reads the mount table through getfsstat without waiting on a
// filesystem, so a share whose server is down cannot stall it.
func Mounts() ([]Mount, error) {
	count, err := syscall.Getfsstat(nil, mntNoWait)
	if err != nil {
		return nil, err
	}
	stats := make([]syscall.Statfs_t, count)
	count, err = syscall.Getfsstat(stats, mntNoWait)
	if err != nil {
		return nil, err
	}
	mounts := make([]Mount, 0, count)
	for _, stat := range stats[:count] {
		mounts = append(mounts, Mount{Point: cString(stat.Mntonname[:]), Type: cString(stat.Fstypename[:])})
	}
	return mounts, nil
}

func cString(field []int8) string {
	bytes := make([]byte, 0, len(field))
	for _, c := range field {
		if c == 0 {
			break
		}
		bytes = append(bytes, byte(c))
	}
	return string(bytes)
}
