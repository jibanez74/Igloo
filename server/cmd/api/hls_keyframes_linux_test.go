package main

import (
	"context"
	"errors"
	"os"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"igloo/cmd/internal/database"
)

// watchOpens counts the times path is opened from now on, through inotify.
func watchOpens(t *testing.T, path string) func() int {
	t.Helper()
	fd, err := syscall.InotifyInit1(syscall.IN_NONBLOCK | syscall.IN_CLOEXEC)
	if err != nil {
		t.Fatalf("inotify init: %v", err)
	}
	t.Cleanup(func() { _ = syscall.Close(fd) })
	_, err = syscall.InotifyAddWatch(fd, path, syscall.IN_OPEN)
	if err != nil {
		t.Fatalf("inotify watch %s: %v", path, err)
	}

	opens := 0
	buf := make([]byte, 64*(syscall.SizeofInotifyEvent+syscall.NAME_MAX+1))
	return func() int {
		for {
			n, readErr := syscall.Read(fd, buf)
			if errors.Is(readErr, syscall.EAGAIN) {
				return opens
			}
			if readErr != nil {
				t.Fatalf("read inotify events: %v", readErr)
			}
			for offset := 0; offset+syscall.SizeofInotifyEvent <= n; {
				event := (*syscall.InotifyEvent)(unsafe.Pointer(&buf[offset]))
				if event.Mask&syscall.IN_OPEN != 0 {
					opens++
				}
				offset += syscall.SizeofInotifyEvent + int(event.Len)
			}
		}
	}
}

// A seek during a file's first copy-video play starts a second session before
// the first one's extraction has persisted the index, and each used to read
// the container's seek tables again. Holding the database's only connection
// keeps the first extraction inside its flight, waiting to persist, while the
// seek's session arrives.
func TestResolveHLSActualStart_ExtractsEachIndexOnce(t *testing.T) {
	app := setupTestApp(t)
	app.Ffprobe = &stubKeyframeFfprobe{err: os.ErrInvalid}

	movieID := writeTestMKVFixture(t, app, []float64{0, 42, 84, 126, 168, 210})
	source, err := app.loadPlaybackSource(context.Background(), movieRef(movieID))
	if err != nil {
		t.Fatalf("load playback source: %v", err)
	}
	fingerprint := keyframeIndexFingerprint(source, &database.VideoStream{StreamIndex: 0})
	opens := watchOpens(t, source.FilePath)

	tx, err := app.DB.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("hold the database connection: %v", err)
	}
	releaseDB := func() { _ = tx.Rollback() }
	defer releaseDB()

	sessions := make([]*HLSSession, 0, 2)
	resolve := func(startSec float64) {
		session := &HLSSession{ActualStartSec: hlsUnknownActualStart}
		sessions = append(sessions, session)
		app.Wait.Add(1)
		go app.resolveHLSActualStart(context.Background(), hlsActualStartParams{
			Session:           session,
			Source:            source,
			StreamIndex:       0,
			Fingerprint:       fingerprint,
			RequestedStartSec: startSec,
		})
	}

	// Read between the two: inotify folds identical unread events into one,
	// which would hide a second open.
	resolve(100)
	first := 0
	deadline := time.Now().Add(5 * time.Second)
	for first == 0 && time.Now().Before(deadline) {
		first += opens()
		time.Sleep(10 * time.Millisecond)
	}
	if first != 1 {
		t.Fatalf("the first play opened the file %d times, want 1", first)
	}
	// singleflight exposes no join signal, so the seek's session gets a
	// moment to reach the flight; an extraction of its own would open the
	// file in that time, which is what the count below catches.
	resolve(200)
	time.Sleep(200 * time.Millisecond)

	got := opens()
	if got != 1 {
		t.Fatalf("the file was opened %d times for one keyframe index, want 1", got)
	}

	releaseDB()
	app.Wait.Wait()

	// Each joiner still answers its own seek from the shared index.
	if sessions[0].actualStartSec() != 84 || sessions[1].actualStartSec() != 168 {
		t.Fatalf("actual starts = %v, %v; want 84 and 168",
			sessions[0].actualStartSec(), sessions[1].actualStartSec())
	}
}
