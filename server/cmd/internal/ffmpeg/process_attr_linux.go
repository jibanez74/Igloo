package ffmpeg

import "syscall"

// childProcessAttributes keeps an FFmpeg child from outliving the server and
// from receiving signals meant for it. Setpgid moves it out of the terminal's
// foreground process group, so a Ctrl-C reaches only the server, whose
// shutdown stops each session in order instead of every encode dying at once
// and being reported as a failure. Pdeathsig kills it when the server dies
// without shutting down (a crash, SIGKILL, an OOM kill): a session encodes the
// whole remaining film, and an orphan would otherwise run for an hour into a
// directory the next startup deletes.
func childProcessAttributes() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid:   true,
		Pdeathsig: syscall.SIGKILL,
	}
}
