package ffmpeg

import "syscall"

// childProcessAttributes moves an FFmpeg child out of the terminal's
// foreground process group, so a Ctrl-C reaches only the server, whose
// shutdown stops each session in order. macOS has no parent-death signal, so
// unlike on Linux an FFmpeg orphaned by a crashed server runs until it
// finishes or its output directory is removed.
func childProcessAttributes() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}
