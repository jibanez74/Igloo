package watch

import (
	"encoding/binary"
	"errors"
	"os"
	"sync"
	"syscall"
)

// watchMask watches a directory for its entries' changes and for its own
// removal or unmount. IN_DONT_FOLLOW keeps directory symlinks unwatched, as
// the scanner walk does, and IN_EXCL_UNLINK drops events from files already
// unlinked while still open.
const watchMask = syscall.IN_CREATE | syscall.IN_DELETE | syscall.IN_MOVED_FROM | syscall.IN_MOVED_TO |
	syscall.IN_MODIFY | syscall.IN_CLOSE_WRITE | syscall.IN_ATTRIB | syscall.IN_DELETE_SELF | syscall.IN_MOVE_SELF |
	syscall.IN_ONLYDIR | syscall.IN_DONT_FOLLOW | syscall.IN_EXCL_UNLINK

// readBuffer holds many events per read; one event is at most the header plus
// NAME_MAX+1 name bytes.
const readBuffer = 256 * (syscall.SizeofInotifyEvent + syscall.NAME_MAX + 1)

type inotify struct {
	file     *os.File
	conn     syscall.RawConn
	out      chan rawEvent
	done     chan struct{}
	closeErr sync.Once
	readErr  error
}

// newNotifier opens one inotify instance. The descriptor is nonblocking, so
// os.NewFile parks reads on the runtime poller and Close unblocks them; the
// reader goroutine joins wait.
func newNotifier(wait *sync.WaitGroup) (notifier, error) {
	fd, err := syscall.InotifyInit1(syscall.IN_NONBLOCK | syscall.IN_CLOEXEC)
	if err != nil {
		return nil, os.NewSyscallError("inotify_init1", err)
	}
	file := os.NewFile(uintptr(fd), "inotify")
	conn, err := file.SyscallConn()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	n := &inotify{file: file, conn: conn, out: make(chan rawEvent, 4096), done: make(chan struct{})}
	wait.Go(n.read)
	return n, nil
}

func (n *inotify) add(path string) (int32, error) {
	var wd int
	err := n.control("inotify_add_watch", func(fd int) (err error) {
		wd, err = syscall.InotifyAddWatch(fd, path, watchMask)
		return err
	})
	return int32(wd), err
}

func (n *inotify) remove(wd int32) error {
	return n.control("inotify_rm_watch", func(fd int) error {
		_, err := syscall.InotifyRmWatch(fd, uint32(wd))
		return err
	})
}

// control runs call on the descriptor while holding it open. A setup walk
// outlives its session, so once the instance is closed it must fail rather
// than reach whatever instance reused the descriptor's number.
func (n *inotify) control(name string, call func(fd int) error) error {
	var callErr error
	err := n.conn.Control(func(fd uintptr) { callErr = call(int(fd)) })
	if err != nil {
		return err
	}
	if callErr != nil {
		return os.NewSyscallError(name, callErr)
	}
	return nil
}

func (n *inotify) events() <-chan rawEvent {
	return n.out
}

// err is valid once events is closed.
func (n *inotify) err() error {
	return n.readErr
}

func (n *inotify) close() error {
	var err error
	n.closeErr.Do(func() {
		close(n.done)
		err = n.file.Close()
	})
	return err
}

func (n *inotify) read() {
	defer close(n.out)
	buffer := make([]byte, readBuffer)
	for {
		count, err := n.file.Read(buffer)
		if errors.Is(err, os.ErrClosed) {
			return
		}
		if err != nil {
			n.readErr = err
			return
		}
		for _, event := range decode(buffer[:count]) {
			select {
			case n.out <- event:
			case <-n.done:
				return
			}
		}
	}
}

// decode splits one read into events. The kernel never splits an event across
// reads.
func decode(buffer []byte) []rawEvent {
	var events []rawEvent
	for offset := 0; offset+syscall.SizeofInotifyEvent <= len(buffer); {
		header := buffer[offset : offset+syscall.SizeofInotifyEvent]
		wd := int32(binary.NativeEndian.Uint32(header[0:4]))
		mask := binary.NativeEndian.Uint32(header[4:8])
		length := int(binary.NativeEndian.Uint32(header[12:16]))
		start := offset + syscall.SizeofInotifyEvent
		end := min(start+length, len(buffer))
		name := buffer[start:end]
		for len(name) > 0 && name[len(name)-1] == 0 {
			name = name[:len(name)-1]
		}
		events = append(events, rawEvent{wd: wd, op: translate(mask), isDir: mask&syscall.IN_ISDIR != 0, name: string(name)})
		offset = end
	}
	return events
}

var opBits = []struct {
	mask uint32
	op   op
}{
	{syscall.IN_CREATE, opCreate},
	{syscall.IN_MODIFY, opWrite},
	{syscall.IN_CLOSE_WRITE, opCloseWrite},
	{syscall.IN_ATTRIB, opAttrib},
	{syscall.IN_DELETE, opDelete},
	{syscall.IN_MOVED_FROM, opMovedFrom},
	{syscall.IN_MOVED_TO, opMovedTo},
	{syscall.IN_DELETE_SELF, opDeleteSelf},
	{syscall.IN_MOVE_SELF, opMoveSelf},
	{syscall.IN_UNMOUNT, opUnmount},
	{syscall.IN_IGNORED, opIgnored},
	{syscall.IN_Q_OVERFLOW, opOverflow},
}

func translate(mask uint32) op {
	var result op
	for _, bit := range opBits {
		if mask&bit.mask != 0 {
			result |= bit.op
		}
	}
	return result
}

// watchLimitReached reports the inotify limits: max_user_watches answers
// ENOSPC, max_user_instances EMFILE.
func watchLimitReached(err error) bool {
	return errors.Is(err, syscall.ENOSPC) || errors.Is(err, syscall.EMFILE)
}
