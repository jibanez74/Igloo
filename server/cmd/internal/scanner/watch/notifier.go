package watch

import "errors"

// op is a platform-neutral inotify event bit.
type op uint32

const (
	opCreate op = 1 << iota
	opWrite
	opCloseWrite
	opAttrib
	opDelete
	opMovedFrom
	opMovedTo
	opDeleteSelf
	opMoveSelf
	opUnmount
	opIgnored
	opOverflow
)

// rawEvent is one decoded kernel event. The session resolves wd to a path.
type rawEvent struct {
	wd    int32
	op    op
	isDir bool
	name  string
}

// notifier is a recursive-free directory watch set: one kernel instance shared
// by every library, with one watch per directory.
type notifier interface {
	add(path string) (int32, error)
	remove(wd int32) error
	// events is closed when the reader stops; err then says why.
	events() <-chan rawEvent
	err() error
	close() error
}

// errNotifierUnsupported means the platform has no event backend, so every
// library polls.
var errNotifierUnsupported = errors.New("filesystem events are not supported on this platform")
