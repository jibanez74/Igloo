package scanner

import (
	"os"
	"strconv"
	"syscall"
)

func fingerprintFromInfo(info os.FileInfo) FileFingerprint {
	stat := info.Sys().(*syscall.Stat_t)
	return FileFingerprint{
		Size: info.Size(), MtimeNS: info.ModTime().UnixNano(),
		CtimeNS: stat.Ctimespec.Sec*1e9 + stat.Ctimespec.Nsec,
		Device:  strconv.FormatUint(uint64(uint32(stat.Dev)), 10), Inode: strconv.FormatUint(stat.Ino, 10),
	}
}
