package watch

import (
	"encoding/binary"
	"sync"
	"syscall"
	"testing"
	"time"

	"igloo/cmd/internal/scanner/scannertest"
)

// A setup that outlives its session must not add watches through the closed
// instance's descriptor number, which the next instance may have reused.
func TestNotifierRefusesWatchesOnceClosed(t *testing.T) {
	wait := &sync.WaitGroup{}
	closed, err := newNotifier(wait)
	if err != nil {
		t.Fatal(err)
	}
	err = closed.close()
	if err != nil {
		t.Fatal(err)
	}
	_, err = closed.add(t.TempDir())
	if err == nil {
		t.Fatal("a closed instance added a watch")
	}

	replacement, err := newNotifier(wait)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = replacement.close()
		scannertest.WaitForGroup(t, wait, 5*time.Second, "the notifier readers to exit")
	})
	_, err = closed.add(t.TempDir())
	if err == nil {
		t.Fatal("a closed instance added a watch after another opened")
	}
	// The kernel numbers an instance's watches from 1, so a watch the closed
	// instance leaked into this one would have taken it.
	wd, err := replacement.add(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if wd != 1 {
		t.Fatalf("the replacement's first watch is %d: a closed instance added one to it", wd)
	}
}

func encodeEvent(wd int32, mask uint32, name string) []byte {
	length := 0
	if name != "" {
		length = (len(name) + 1 + 15) / 16 * 16
	}
	event := make([]byte, syscall.SizeofInotifyEvent+length)
	binary.NativeEndian.PutUint32(event[0:4], uint32(wd))
	binary.NativeEndian.PutUint32(event[4:8], mask)
	binary.NativeEndian.PutUint32(event[12:16], uint32(length))
	copy(event[syscall.SizeofInotifyEvent:], name)
	return event
}

func TestDecodeSplitsEventsAndTrimsNames(t *testing.T) {
	buffer := append(encodeEvent(3, syscall.IN_CLOSE_WRITE, "movie.mkv"), encodeEvent(4, syscall.IN_CREATE|syscall.IN_ISDIR, "Season 1")...)
	buffer = append(buffer, encodeEvent(-1, syscall.IN_Q_OVERFLOW, "")...)
	events := decode(buffer)
	want := []rawEvent{
		{wd: 3, op: opCloseWrite, name: "movie.mkv"},
		{wd: 4, op: opCreate, isDir: true, name: "Season 1"},
		{wd: -1, op: opOverflow},
	}
	if len(events) != len(want) {
		t.Fatalf("events = %+v", events)
	}
	for i := range want {
		if events[i] != want[i] {
			t.Errorf("event %d = %+v, want %+v", i, events[i], want[i])
		}
	}
}
