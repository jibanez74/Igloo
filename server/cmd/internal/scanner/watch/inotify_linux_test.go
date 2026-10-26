package watch

import (
	"sync"
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
