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
		CtimeNS: stat.Ctim.Sec*1e9 + stat.Ctim.Nsec,
		Device:  strconv.FormatUint(uint64(stat.Dev), 10), Inode: strconv.FormatUint(stat.Ino, 10),
	}
}
