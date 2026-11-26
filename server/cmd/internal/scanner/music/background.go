package music

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"maps"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner"
)

// The background worker runs the slow music lookups the scan run leaves out,
// one pass at a time on a goroutine of its own: today the album art stage,
// which copies each album's own art. A scan pauses it before it starts and
// resumes it when it ends, so the worker never writes beside a scan and a
// long first pass never holds a scan or a watcher run back.

const (
	// lookupPageSize is how many album ids one listing query reads.
	lookupPageSize = 500
	// artworkRequestEvery and artworkRequestInterval space the artwork passes
	// a long lookup pass asks for, so what it changed is downloaded as it
	// goes rather than only at its end.
	artworkRequestEvery    = 50
	artworkRequestInterval = 2 * time.Minute
)

// LookupState mirrors the lookups state enum in docs/openapi.json.
type LookupState string

const (
	LookupIdle    LookupState = "idle"
	LookupRunning LookupState = "running"
	// LookupPaused is work waiting for a scan to end.
	LookupPaused LookupState = "paused"
)

// LookupStage mirrors the lookups stage enum in docs/openapi.json.
type LookupStage string

const StageLocalArt LookupStage = "local-art"

// LookupStatus is the current or latest pass of the background worker. Its
// counters count albums: matched found art of their own, unmatched have
// none, and failed had art that could not be copied.
type LookupStatus struct {
	State     LookupState  `json:"state"`
	Stage     *LookupStage `json:"stage"`
	Processed int          `json:"processed"`
	Total     int          `json:"total"`
	Matched   int          `json:"matched"`
	Failed    int          `json:"failed"`
	Unmatched int          `json:"unmatched"`
}

// lookupRequest is the work a pass does: every album with an id above after
// when full, and the albums in albums. A full pass a scan interrupted resumes
// above the last album it read, so runs of watcher-detected changes, each of
// which pauses the worker, never send it back to the first album; a new full
// request starts it over.
type lookupRequest struct {
	full   bool
	after  int64
	albums map[int64]bool
}

func (r *lookupRequest) merge(other lookupRequest) {
	switch {
	case r.full && other.full:
		r.after = min(r.after, other.after)
	case other.full:
		r.full, r.after = true, other.after
	}
	if len(other.albums) == 0 {
		return
	}
	if r.albums == nil {
		r.albums = make(map[int64]bool, len(other.albums))
	}
	maps.Copy(r.albums, other.albums)
}

func (r lookupRequest) empty() bool {
	return !r.full && len(r.albums) == 0
}

func albumsRequest(ids []int64) lookupRequest {
	request := lookupRequest{albums: make(map[int64]bool, len(ids))}
	for _, id := range ids {
		request.albums[id] = true
	}
	return request
}

// lookups runs the passes. run does one and returns what it left undone,
// which waits for the next.
type lookups struct {
	// ctx is the scan context, read at each start; shutdown cancels it.
	ctx  func() context.Context
	wait *sync.WaitGroup
	run  func(ctx context.Context, request lookupRequest) lookupRequest

	mu      sync.Mutex
	running bool
	paused  bool
	cancel  context.CancelFunc
	done    chan struct{}
	pending lookupRequest
}

// request asks for a pass, which starts unless a scan holds the worker
// paused or a pass runs; that pass's successor takes it up.
func (l *lookups) request(request lookupRequest) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pending.merge(request)
	l.startLocked()
}

// pause stops a running pass and waits for it to return; what it left undone
// stays pending until resume.
func (l *lookups) pause() {
	l.mu.Lock()
	l.paused = true
	if !l.running {
		l.mu.Unlock()
		return
	}
	l.cancel()
	done := l.done
	l.mu.Unlock()
	<-done
}

// resume lets passes run again and asks for one.
func (l *lookups) resume(request lookupRequest) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.paused = false
	l.pending.merge(request)
	l.startLocked()
}

func (l *lookups) state() LookupState {
	l.mu.Lock()
	defer l.mu.Unlock()
	switch {
	case l.paused && (l.running || !l.pending.empty()):
		return LookupPaused
	case l.running:
		return LookupRunning
	}
	return LookupIdle
}

func (l *lookups) startLocked() {
	parent := l.ctx()
	canStart := !l.running && !l.paused && !l.pending.empty() && parent.Err() == nil
	if !canStart {
		return
	}
	passCtx, cancel := context.WithCancel(parent)
	l.running, l.cancel, l.done = true, cancel, make(chan struct{})
	l.wait.Go(func() { l.loop(passCtx) })
}

func (l *lookups) loop(ctx context.Context) {
	for {
		l.mu.Lock()
		stop := l.paused || l.pending.empty() || ctx.Err() != nil
		if stop {
			l.running = false
			l.cancel()
			close(l.done)
			l.mu.Unlock()
			return
		}
		request := l.pending
		l.pending = lookupRequest{}
		l.mu.Unlock()

		rest := l.run(ctx, request)

		l.mu.Lock()
		l.pending.merge(rest)
		l.mu.Unlock()
	}
}

// RequestLookups asks the background worker for a pass over every album; a
// settings save that moves the static directory calls it, since the copies
// live there.
func (s *Scanner) RequestLookups() {
	s.lookups.request(lookupRequest{full: true})
}

// runLookups is one pass of the worker. A pass stopped by a pause or by
// shutdown returns the albums it did not reach; one that failed returns
// nothing, so a broken database does not spin the worker, and the next scan
// asks again.
func (s *Scanner) runLookups(ctx context.Context, request lookupRequest) lookupRequest {
	started := s.now()
	ids, err := s.lookupAlbums(ctx, request)
	if err != nil {
		if ctx.Err() != nil {
			return request
		}
		s.logger.Error("failed to list albums for music lookups", "error", err)
		return lookupRequest{}
	}

	stage := StageLocalArt
	s.setLookups(func(status *LookupStatus) {
		*status = LookupStatus{Stage: &stage, Total: len(ids)}
	})
	defer s.setLookups(func(status *LookupStatus) { status.Stage = nil })

	done, changed, removed, err := s.localArtStage(ctx, request.full, ids)
	if err != nil && ctx.Err() == nil && !errors.Is(err, errSettingsChanged) {
		s.logger.Error("music album art stage failed", "error", err)
	}
	if changed > 0 || removed > 0 {
		s.lookupsChanged()
	}
	status := s.Status().Lookups
	summary := []any{
		"processed", status.Processed, "total", status.Total, "matched", status.Matched, "unmatched", status.Unmatched,
		"failed", status.Failed, "changed", changed, "removed_folders", removed, "duration", s.now().Sub(started).Round(time.Millisecond),
	}
	if changed > 0 || removed > 0 || status.Failed > 0 {
		s.logger.Info("music album art pass finished", summary...)
	} else {
		s.logger.Debug("music album art pass finished", summary...)
	}

	stopped := ctx.Err() != nil || errors.Is(err, errSettingsChanged)
	if !stopped {
		return lookupRequest{}
	}
	if request.full {
		// The ids are sorted, so every album up to the last one read is
		// done; the rest, requested or not, is above it.
		rest := lookupRequest{full: true, after: request.after}
		if done > 0 {
			rest.after = max(rest.after, ids[done-1])
		}
		for _, id := range ids[done:] {
			if id <= rest.after {
				rest.merge(albumsRequest([]int64{id}))
			}
		}
		return rest
	}
	return albumsRequest(ids[done:])
}

// lookupAlbums lists the albums a pass reads, by id.
func (s *Scanner) lookupAlbums(ctx context.Context, request lookupRequest) ([]int64, error) {
	requested, err := s.withNeighborAlbums(ctx, slices.Sorted(maps.Keys(request.albums)))
	if err != nil || !request.full {
		return requested, err
	}
	ids := requested
	afterID := request.after
	for {
		page, err := s.queries.ListMusicAlbumIDs(ctx, database.ListMusicAlbumIDsParams{AfterID: afterID, PageSize: lookupPageSize})
		if err != nil {
			return nil, err
		}
		ids = append(ids, page...)
		if len(page) < lookupPageSize {
			slices.Sort(ids)
			return slices.Compact(ids), nil
		}
		afterID = page[len(page)-1]
	}
}

// withNeighborAlbums adds to ids the albums whose tracks share a folder, or
// the parent of a disc folder, with theirs: a track that joins or leaves a
// folder changes whether the folder's image is another album's.
func (s *Scanner) withNeighborAlbums(ctx context.Context, ids []int64) ([]int64, error) {
	folders := make(map[string]bool)
	for _, id := range ids {
		tracks, err := s.queries.ListMusicAlbumArtTracks(ctx, sql.NullInt64{Int64: id, Valid: true})
		if err != nil {
			return nil, err
		}
		for _, track := range tracks {
			dir := filepath.Dir(track.FilePath)
			folders[dir] = true
			if isDiscFolder(filepath.Base(dir)) {
				folders[filepath.Dir(dir)] = true
			}
		}
	}
	all := slices.Clone(ids)
	for folder := range folders {
		lower, upper := scanner.ChildBounds(folder)
		rows, err := s.queries.ListMusicTrackAlbumsUnder(ctx, database.ListMusicTrackAlbumsUnderParams{Lower: lower, Upper: upper})
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if row.AlbumID.Valid && isNearTrack(folder, row.FilePath) {
				all = append(all, row.AlbumID.Int64)
			}
		}
	}
	slices.Sort(all)
	return slices.Compact(all), nil
}

// localArtStage gives each album in ids the art of its own, then, after a
// full pass, removes the folders no album points at. It reports how many
// albums it got through, how many changed and how many folders it removed.
// The static directory must be set and must not overlap a library: the stage
// never writes into one.
func (s *Scanner) localArtStage(ctx context.Context, full bool, ids []int64) (int, int, int, error) {
	// A stage that cannot run reads no album, so its line shows none.
	skip := func() (int, int, int, error) {
		s.setLookups(func(status *LookupStatus) { status.Total = 0 })
		return len(ids), 0, 0, nil
	}
	staticDir := s.staticDir()
	if staticDir == "" {
		return skip()
	}
	roots := s.libraryDirs()
	library, overlaps := helpers.OverlappingLibrary(filepath.Join(staticDir, "local"), roots)
	if overlaps {
		s.logger.Error("not copying album art into a folder that overlaps a media library; choose another static directory", "folder", filepath.Join(staticDir, "local"), "library", library)
		return skip()
	}

	art := &localArt{
		staticDir: staticDir,
		roots:     roots,
		current:   func() bool { return s.staticDir() == staticDir && slices.Equal(s.libraryDirs(), roots) },
		owners:    make(map[string]int64),
	}
	changed, sinceRequest := 0, 0
	lastRequest := s.now()
	for i, id := range ids {
		if ctx.Err() != nil {
			return i, changed, 0, ctx.Err()
		}
		outcome, albumChanged, err := s.syncAlbumArt(ctx, art, id)
		// A change that committed counts even when the pass stops right
		// after it: the next pass finds the album unchanged.
		if albumChanged {
			changed++
			sinceRequest++
			s.noteLookupChange()
		}
		if ctx.Err() != nil || errors.Is(err, errSettingsChanged) {
			return i, changed, 0, cmp.Or(ctx.Err(), err)
		}
		if err != nil {
			// A library that is not mounted fails every album; the pass
			// summary reports how many.
			s.logger.Debug("failed to read the album's own art", "album_id", id, "error", err)
			outcome = artFailed
		}
		s.setLookups(func(status *LookupStatus) {
			status.Processed++
			switch outcome {
			case artFound:
				status.Matched++
			case artNone:
				status.Unmatched++
			default:
				status.Failed++
			}
		})
		isRequestDue := sinceRequest >= artworkRequestEvery || (sinceRequest > 0 && s.now().Sub(lastRequest) >= artworkRequestInterval)
		if isRequestDue {
			s.lookupsChanged()
			sinceRequest, lastRequest = 0, s.now()
		}
	}

	if !full {
		return len(ids), changed, 0, nil
	}
	removed, err := s.pruneLocalCovers(ctx, art)
	return len(ids), changed, removed, err
}

// setLookups changes the published lookup status.
func (s *Scanner) setLookups(change func(*LookupStatus)) {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	change(&s.lookupStatus)
}

// noteLookupChange counts one catalog change of the worker in the scanner's
// revision, so clients refetch what it changed.
func (s *Scanner) noteLookupChange() {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	s.revision.Add(&s.status.Progress, 1)
}
