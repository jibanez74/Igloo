package music

import (
	"context"
	"sync"
	"testing"
	"time"

	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/scanner/scannertest"
)

// blockingLookups is a worker whose passes run until they are canceled, or
// until release, recording each request they get.
type blockingLookups struct {
	l        *lookups
	wait     *sync.WaitGroup
	started  chan lookupRequest
	release  chan struct{}
	mu       sync.Mutex
	finished []lookupRequest
}

func newBlockingLookups(ctx context.Context) *blockingLookups {
	b := &blockingLookups{wait: &sync.WaitGroup{}, started: make(chan lookupRequest, 8), release: make(chan struct{})}
	b.l = &lookups{ctx: func() context.Context { return ctx }, wait: b.wait, run: b.run}
	return b
}

func (b *blockingLookups) run(ctx context.Context, request lookupRequest) lookupRequest {
	b.started <- request
	select {
	case <-ctx.Done():
		return request
	case <-b.release:
		b.mu.Lock()
		b.finished = append(b.finished, request)
		b.mu.Unlock()
		return lookupRequest{}
	}
}

func (b *blockingLookups) nextStart(t *testing.T) lookupRequest {
	t.Helper()
	select {
	case request := <-b.started:
		return request
	case <-time.After(5 * time.Second):
		t.Fatal("no pass started")
		return lookupRequest{}
	}
}

func (b *blockingLookups) noStart(t *testing.T) {
	t.Helper()
	select {
	case request := <-b.started:
		t.Fatalf("a pass started with %+v", request)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestLookupsPauseStopsAPassAndResumeRunsWhatItLeft(t *testing.T) {
	b := newBlockingLookups(context.Background())
	b.l.request(albumsRequest([]int64{1, 2}))
	first := b.nextStart(t)
	if first.full || len(first.albums) != 2 {
		t.Fatalf("first pass = %+v, want albums 1 and 2", first)
	}
	if b.l.state() != LookupRunning {
		t.Fatalf("state = %s, want running", b.l.state())
	}

	b.l.pause()
	if b.l.state() != LookupPaused {
		t.Fatalf("state = %s after a pause, want paused", b.l.state())
	}
	// Requests while paused wait for the scan.
	b.l.request(albumsRequest([]int64{3}))
	b.noStart(t)

	b.l.resume(albumsRequest([]int64{4}))
	resumed := b.nextStart(t)
	for _, id := range []int64{1, 2, 3, 4} {
		if !resumed.albums[id] {
			t.Fatalf("resumed pass = %+v, want albums 1 to 4", resumed)
		}
	}
	close(b.release)
	scannertest.WaitForGroup(t, b.wait, 5*time.Second, "lookups")
	if b.l.state() != LookupIdle {
		t.Fatalf("state = %s after the pass, want idle", b.l.state())
	}
}

func TestLookupsFullRequestKeepsItsAlbums(t *testing.T) {
	b := newBlockingLookups(context.Background())
	b.l.pause()
	b.l.request(albumsRequest([]int64{1}))
	b.l.request(lookupRequest{full: true})
	b.l.resume(albumsRequest([]int64{2}))
	request := b.nextStart(t)
	if !request.full || request.after != 0 || len(request.albums) != 2 {
		t.Fatalf("pass = %+v, want a full pass from the start with albums 1 and 2", request)
	}
	close(b.release)
	scannertest.WaitForGroup(t, b.wait, 5*time.Second, "lookups")
}

func TestLookupRequestMergeKeepsTheResumePoint(t *testing.T) {
	tests := []struct {
		name      string
		a, b      lookupRequest
		wantFull  bool
		wantAfter int64
	}{
		{"albums keep a resumed pass where it was", lookupRequest{full: true, after: 40}, albumsRequest([]int64{3}), true, 40},
		{"a resumed pass joins albums", albumsRequest([]int64{3}), lookupRequest{full: true, after: 40}, true, 40},
		{"a new full request starts over", lookupRequest{full: true, after: 40}, lookupRequest{full: true}, true, 0},
		{"albums alone are no full pass", albumsRequest([]int64{1}), albumsRequest([]int64{2}), false, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.a
			got.merge(tt.b)
			if got.full != tt.wantFull || got.after != tt.wantAfter {
				t.Fatalf("merged = %+v, want full %v after %d", got, tt.wantFull, tt.wantAfter)
			}
		})
	}
}

func TestLookupsRequestsDuringAPassShareTheNextOne(t *testing.T) {
	b := newBlockingLookups(context.Background())
	b.l.request(albumsRequest([]int64{1}))
	b.nextStart(t)
	b.l.request(albumsRequest([]int64{2}))
	b.l.request(albumsRequest([]int64{3}))
	b.release <- struct{}{}
	next := b.nextStart(t)
	if len(next.albums) != 2 || !next.albums[2] || !next.albums[3] {
		t.Fatalf("next pass = %+v, want albums 2 and 3", next)
	}
	close(b.release)
	scannertest.WaitForGroup(t, b.wait, 5*time.Second, "lookups")
	if len(b.finished) != 2 {
		t.Fatalf("finished %d passes, want 2", len(b.finished))
	}
}

func TestLookupsStopAtShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	b := newBlockingLookups(ctx)
	b.l.request(lookupRequest{full: true})
	b.nextStart(t)
	cancel()
	scannertest.WaitForGroup(t, b.wait, 5*time.Second, "lookups")

	b.l.request(lookupRequest{full: true})
	b.l.resume(lookupRequest{full: true})
	b.noStart(t)
}

func TestLookupsPauseWithoutAPassReturnsAtOnce(t *testing.T) {
	b := newBlockingLookups(context.Background())
	b.l.pause()
	if b.l.state() != LookupIdle {
		t.Fatalf("state = %s, want idle while nothing waits", b.l.state())
	}
	b.l.resume(lookupRequest{})
	b.noStart(t)
}

// A music scan pauses the worker for its whole run and resumes it with the
// albums a run of changes touched.
func TestMusicScanPausesTheLookupsAndResumesThemAfter(t *testing.T) {
	f := newArtFixture(t)
	f.track(t, "Band/Album/01.flac", "Album")
	f.image(t, "Band/Album/cover.jpg", jpegBytes)
	running := make(chan struct{})
	release := make(chan struct{})
	f.s.ffprobe = &blockingProbe{probe: newMusicScannerFfprobeByPath(f.probes), running: running, release: release}
	f.s.lookups.request(lookupRequest{full: true})
	f.wait(t)

	scanned := make(chan struct{})
	go func() {
		defer close(scanned)
		f.s.scan(f.musicDir)
	}()
	<-running
	if f.s.Status().Lookups.State != LookupIdle {
		t.Fatalf("lookups = %s during a scan with nothing waiting, want idle", f.s.Status().Lookups.State)
	}
	f.s.lookups.request(lookupRequest{full: true})
	if f.s.Status().Lookups.State != LookupPaused {
		t.Fatalf("lookups = %s during a scan, want paused", f.s.Status().Lookups.State)
	}
	close(release)
	<-scanned
	f.wait(t)
	if !f.album(t, "Album").coverLocal.Valid {
		t.Fatal("the lookups did not resume after the scan")
	}
}

type blockingProbe struct {
	probe   *musicScannerFfprobeByPath
	running chan struct{}
	release chan struct{}
	once    sync.Once
}

func (p *blockingProbe) GetMetadata(ctx context.Context, path string) (*ffprobe.FfprobeResult, error) {
	return p.probe.GetMetadata(ctx, path)
}

func (p *blockingProbe) GetAudioMetadata(ctx context.Context, path string) (*ffprobe.FfprobeResult, error) {
	p.once.Do(func() { close(p.running) })
	<-p.release
	return p.probe.GetAudioMetadata(ctx, path)
}
