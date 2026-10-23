package music

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/scannertest"
	spotifyapi "igloo/cmd/internal/spotify"
)

type spotifyStep struct {
	operation, target string
	artist            *spotifyapi.FullArtist
	album             *spotifyapi.FullAlbum
	err               error
	run               func(context.Context)
}

type spotifyScript struct {
	mu    sync.Mutex
	t     *testing.T
	steps []spotifyStep
	log   []spotifyStep
}

func scriptedSpotify(t *testing.T, steps ...spotifyStep) *musicScannerSpotifyStub {
	t.Helper()
	script := &spotifyScript{t: t, steps: steps}
	t.Cleanup(func() {
		script.mu.Lock()
		defer script.mu.Unlock()
		if len(script.steps) != 0 {
			t.Errorf("missing Spotify requests: %+v", script.steps)
		}
	})
	return &musicScannerSpotifyStub{script: script}
}

func (s *spotifyScript) request(ctx context.Context, operation, target string) spotifyStep {
	s.mu.Lock()
	s.log = append(s.log, spotifyStep{operation: operation, target: target})
	if len(s.steps) == 0 || s.steps[0].operation != operation || s.steps[0].target != target {
		s.t.Errorf("unexpected Spotify request %s %q; remaining %+v", operation, target, s.steps)
		s.mu.Unlock()
		return spotifyStep{err: errors.New("unexpected request")}
	}
	step := s.steps[0]
	s.steps = s.steps[1:]
	s.mu.Unlock()
	if step.run != nil {
		step.run(ctx)
	}
	if ctx.Err() != nil {
		step.err = ctx.Err()
	}
	return step
}

func nameMiss() error {
	return &spotifyapi.MatchError{Info: spotifyapi.MatchDebugInfo{Reason: spotifyapi.MatchReasonNoResults}}
}

func TestSpotifyRecoveryUnchangedAdoption(t *testing.T) {
	s := setupMusicScanner(t)
	dir := t.TempDir()
	scannertest.WriteFile(t, filepath.Join(dir, "track.m4a"), "audio")
	probe := &scannertest.CountingProbe{Default: testMusicMetadata()}
	s.ffprobe = probe
	s.spotify = scriptedSpotify(t,
		spotifyStep{operation: "search", target: "Test Artist", err: nameMiss()},
		spotifyStep{operation: "match", target: "Test Album", album: testAlbum()},
		spotifyStep{operation: "artist", target: "artist", err: errors.New("transport")},
		spotifyStep{operation: "artist", target: "artist", artist: testArtist()},
	)
	s.scan(dir)
	s.scan(dir)
	if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM musicians WHERE spotify_id='artist'") != 1 {
		t.Fatal("unchanged scan did not recover adoption")
	}
	s.scan(dir)
	if probe.Calls() != 1 {
		t.Fatalf("probes=%d", probe.Calls())
	}
}

func TestSpotifyRecoveryLaterArtistCredit(t *testing.T) {
	s := setupMusicScanner(t)
	album := testAlbum()
	album.Artists = append([]spotifyapi.SimpleArtist{{ID: "other", Name: "Other"}}, album.Artists...)
	s.spotify = scriptedSpotify(t,
		spotifyStep{operation: "search", target: "Test Artist", err: nameMiss()},
		spotifyStep{operation: "match", target: "Test Album", album: album},
		spotifyStep{operation: "artist", target: "artist", artist: testArtist()},
	)
	dir := t.TempDir()
	scannertest.WriteFile(t, filepath.Join(dir, "track.m4a"), "audio")
	s.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadata()}
	s.scan(dir)
	if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM musicians WHERE spotify_id='artist'") != 1 {
		t.Fatal("later agreeing artist credit was ignored")
	}
}

func TestSpotifyRecoveryAlbumAliasCache(t *testing.T) {
	for _, stopped := range []bool{false, true} {
		t.Run(map[bool]string{false: "ready", true: "stopped"}[stopped], func(t *testing.T) {
			s := setupMusicScanner(t)
			ctx := context.Background()
			owner, err := s.queries.UpsertAlbum(ctx, database.UpsertAlbumParams{Title: "Retained", SortTitle: "Retained", SpotifyID: helpers.NullString("album")})
			if err != nil {
				t.Fatal(err)
			}
			s.spotify = scriptedSpotify(t, spotifyStep{operation: "match", target: "Alias", album: testAlbum(spotifyTrack("t1", "Song", 1, 1, 180250, false))})
			scan := newMusicScanContext(nil)
			path := filepath.Join(t.TempDir(), "track.m4a")
			scanTaggedTrack(t, s, scan, path, 1, ffprobe.FormatTags{Title: "Song", Album: "Alias", Track: "1"})
			scan.breaker.Stopped = stopped
			linkScannedTracks(t, s, scan)
			assertLinked(t, trackLinkFor(t, s, path), "t1", false, 1)
			if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM tracks WHERE album_id=?", owner.ID) != 1 {
				t.Fatal("alias did not retain owner")
			}
		})
	}
}

// fileDatabaseSource is a WAL database file, which a reopened scanner can read
// after cancellation discards a connection.
func fileDatabaseSource(t *testing.T) string {
	return filepath.Join(t.TempDir(), "music.db") + "?_foreign_keys=on&_journal_mode=WAL&_synchronous=NORMAL"
}

// markArtistsUnmatched records a final name-search miss for every musician.
func markArtistsUnmatched(t *testing.T, s *Scanner) {
	t.Helper()
	_, err := s.tx.DB.Exec("INSERT INTO music_provider_matches(provider,entity_type,entity_id,status,reason) SELECT 'spotify','musician',id,'unmatched','no_results' FROM musicians")
	if err != nil {
		t.Fatal(err)
	}
}

func reopenRecoveryScanner(t *testing.T, s *Scanner, source string) *Scanner {
	t.Helper()
	err := s.queries.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = s.tx.DB.Close()
	if err != nil {
		t.Fatal(err)
	}
	return setupMusicScannerDatabase(t, source)
}

func TestSpotifyRecoveryProviderFailures(t *testing.T) {
	cases := []struct {
		name              string
		err               error
		terminal, stopped bool
	}{
		{name: "transport", err: errors.New("connection reset")},
		{name: "rate limit", err: &spotifyapi.StatusError{StatusCode: 429}},
		{name: "server", err: &spotifyapi.StatusError{StatusCode: 503}},
		{name: "credentials", err: &spotifyapi.StatusError{StatusCode: 401}, stopped: true},
		{name: "forbidden", err: &spotifyapi.StatusError{StatusCode: 403}, stopped: true},
		{name: "token", err: &spotifyapi.TokenError{StatusCode: 400}, stopped: true},
		{name: "quota", err: &spotifyapi.StatusError{StatusCode: 429, Reason: "QUOTA_EXCEEDED"}, stopped: true},
		{name: "gone", err: &spotifyapi.StatusError{StatusCode: 404}, terminal: true},
	}
	for _, tc := range cases {
		for _, initial := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/initial=%v", tc.name, initial), func(t *testing.T) {
				source := fileDatabaseSource(t)
				s := setupMusicScannerDatabase(t, source)
				dir := t.TempDir()
				scannertest.WriteFile(t, filepath.Join(dir, "track.m4a"), "audio")
				probe := &scannertest.CountingProbe{Default: testMusicMetadata()}
				s.ffprobe = probe
				if !initial {
					s.scan(dir)
				}
				s.spotify = scriptedSpotify(t,
					spotifyStep{operation: "search", target: "Test Artist", err: nameMiss()},
					spotifyStep{operation: "match", target: "Test Album", album: testAlbum()},
					spotifyStep{operation: "artist", target: "artist", err: tc.err},
				)
				s.scan(dir)
				pending := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_spotify_adoptions WHERE spotify_id='artist'")
				expected := 1
				if tc.terminal {
					expected = 0
				}
				if pending != expected {
					t.Fatalf("pending=%d want %d", pending, expected)
				}
				status := s.Status()
				if (status.State == scanner.StateCompletedWithIssues) != tc.stopped {
					t.Fatalf("status=%+v", status)
				}
				// A final name-search miss must not hide a persisted ID adoption.
				if !tc.terminal {
					_, err := s.tx.DB.Exec("UPDATE music_provider_matches SET status='unmatched',reason='no_results' WHERE entity_type='musician'")
					if err != nil {
						t.Fatal(err)
					}
				}
				s = reopenRecoveryScanner(t, s, source)
				s.ffprobe = probe
				if tc.terminal {
					s.spotify = scriptedSpotify(t)
				} else {
					s.spotify = scriptedSpotify(t, spotifyStep{operation: "artist", target: "artist", artist: testArtist()})
				}
				s.scan(dir)
				want := 1
				if tc.terminal {
					want = 0
				}
				if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM musicians WHERE spotify_id='artist'") != want {
					t.Fatal("incorrect recovered identity")
				}
				if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_spotify_adoptions") != 0 {
					t.Fatal("pending adoption not cleared")
				}
				s.scan(dir)
				if probe.Calls() != 1 || s.Status().State != scanner.StateCompleted {
					t.Fatalf("probes=%d status=%+v", probe.Calls(), s.Status())
				}
			})
		}
	}
}

func TestSpotifyRecoveryCreditSelection(t *testing.T) {
	cases := []struct {
		name    string
		credits []spotifyapi.SimpleArtist
		target  string
		matched bool
		pending string
	}{
		{name: "first", credits: []spotifyapi.SimpleArtist{{ID: "artist", Name: "Test Artist"}}, target: "artist"},
		{name: "second", credits: []spotifyapi.SimpleArtist{{ID: "other", Name: "Other"}, {ID: "artist", Name: "Test Artist"}}, target: "artist"},
		{name: "later skips empty IDs", credits: []spotifyapi.SimpleArtist{{Name: "Test Artist"}, {ID: "other", Name: "Other"}, {ID: "artist", Name: "Test Artist"}, {ID: "last", Name: "Test Artist"}}, target: "artist"},
		{name: "unrelated", credits: []spotifyapi.SimpleArtist{{ID: "other", Name: "Other"}}},
		{name: "empty ID", credits: []spotifyapi.SimpleArtist{{Name: "Test Artist"}}},
		{name: "various", credits: []spotifyapi.SimpleArtist{{ID: "various", Name: "Various Artists"}}},
		{name: "already matched", credits: []spotifyapi.SimpleArtist{{ID: "artist", Name: "Test Artist"}}, matched: true},
		{name: "pending wins", credits: []spotifyapi.SimpleArtist{{ID: "replacement", Name: "Test Artist"}}, pending: "artist", target: "artist"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := setupMusicScanner(t)
			scanTaggedTrack(t, s, newMusicScanContext(nil), filepath.Join(t.TempDir(), "track.m4a"), 1, albumTags("Song", "1"))
			if tc.matched {
				_, err := s.tx.DB.Exec("UPDATE musicians SET spotify_id='known'")
				if err != nil {
					t.Fatal(err)
				}
			}
			if tc.pending != "" {
				_, err := s.tx.DB.Exec("INSERT INTO music_spotify_adoptions SELECT id,? FROM musicians", tc.pending)
				if err != nil {
					t.Fatal(err)
				}
			}
			var steps []spotifyStep
			if tc.target != "" {
				steps = append(steps, spotifyStep{operation: "artist", target: tc.target, artist: testArtist()})
			}
			s.spotify = scriptedSpotify(t, steps...)
			album := testAlbum()
			album.Artists = tc.credits
			adopted, err := s.adoptAlbumArtist(context.Background(), newMusicScanContext(nil), "Test Artist", album)
			if err != nil {
				t.Fatal(err)
			}
			if (adopted != nil) != (tc.target != "") {
				t.Fatalf("adoption=%+v", adopted)
			}
		})
	}
}

func TestSpotifyRecoveryCancellation(t *testing.T) {
	for _, phase := range []string{"initial lookup", "lookup", "pending lookup"} {
		t.Run(phase, func(t *testing.T) {
			source := fileDatabaseSource(t)
			s := setupMusicScannerDatabase(t, source)
			dir := t.TempDir()
			scannertest.WriteFile(t, filepath.Join(dir, "track.m4a"), "audio")
			probe := &scannertest.CountingProbe{Default: testMusicMetadata()}
			s.ffprobe = probe
			if phase != "initial lookup" {
				s.scan(dir)
			}
			// Persist the final name miss before the album retry.
			markArtistsUnmatched(t, s)
			if phase == "pending lookup" {
				s.spotify = scriptedSpotify(t, spotifyStep{operation: "match", target: "Test Album", album: testAlbum()}, spotifyStep{operation: "artist", target: "artist", err: errors.New("transport")})
				s.scan(dir)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s.scanContext = ctx
			entered := make(chan struct{})
			step := spotifyStep{operation: "artist", target: "artist", artist: testArtist(), run: func(ctx context.Context) {
				close(entered)
				<-ctx.Done()
			}}
			var steps []spotifyStep
			if phase == "initial lookup" {
				steps = append(steps, spotifyStep{operation: "search", target: "Test Artist", err: nameMiss()})
			}
			if phase != "pending lookup" {
				steps = append(steps, spotifyStep{operation: "match", target: "Test Album", album: testAlbum()})
			}
			steps = append(steps, step)
			s.spotify = scriptedSpotify(t, steps...)
			invalidations := 0
			s.invalidateCommittedTrack = func(int64) { invalidations++ }
			done := make(chan struct{})
			go func() { s.scan(dir); close(done) }()
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("ID lookup not reached")
			}
			cancel()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("scan did not cancel")
			}
			status := s.Status()
			if status.State != scanner.StateCanceled || status.EnrichmentFailed != 0 || invalidations != 0 {
				t.Fatalf("canceled status=%+v invalidations=%d", status, invalidations)
			}
			if phase != "pending lookup" && scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM albums WHERE spotify_id IS NOT NULL") != 0 {
				t.Fatal("album committed without adoption")
			}
			s = reopenRecoveryScanner(t, s, source)
			s.ffprobe = probe
			steps = nil
			if phase == "initial lookup" {
				steps = append(steps, spotifyStep{operation: "search", target: "Test Artist", err: nameMiss()})
			}
			if phase != "pending lookup" {
				steps = append(steps, spotifyStep{operation: "match", target: "Test Album", album: testAlbum()})
			}
			steps = append(steps, spotifyStep{operation: "artist", target: "artist", artist: testArtist()})
			s.spotify = scriptedSpotify(t, steps...)
			s.scan(dir)
			expectedProbes := 1
			if phase == "initial lookup" {
				expectedProbes = 2
			}
			if s.Status().State != scanner.StateCompleted || probe.Calls() != expectedProbes || scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM musicians WHERE spotify_id='artist'") != 1 {
				t.Fatalf("recovery status=%+v probes=%d", s.Status(), probe.Calls())
			}
		})
	}
}

func TestSpotifyRecoveryAtomicity(t *testing.T) {
	for _, initial := range []bool{true, false} {
		for _, failure := range []string{"pending", "enrichment", "merge"} {
			t.Run(fmt.Sprintf("%s/initial=%v", failure, initial), func(t *testing.T) {
				s := setupMusicScanner(t)
				dir := t.TempDir()
				path := filepath.Join(dir, "track.m4a")
				scannertest.WriteFile(t, path, "audio")
				probe := &scannertest.CountingProbe{Default: testMusicMetadata()}
				s.ffprobe = probe
				// Keep a pre-existing local row for merge rollback even on initial import.
				_, err := s.queries.UpsertMusician(context.Background(), database.UpsertMusicianParams{Name: "Test Artist", SortName: "Test Artist"})
				if err != nil {
					t.Fatal(err)
				}
				_, err = s.tx.DB.Exec("INSERT INTO music_artist_identity(identity_key,musician_id) VALUES('test artist',1)")
				if err != nil {
					t.Fatal(err)
				}
				if !initial {
					s.scan(dir)
				}
				markArtistsUnmatched(t, s)
				trigger := "CREATE TRIGGER reject_adoption BEFORE INSERT ON music_spotify_adoptions BEGIN SELECT RAISE(ABORT,'pending failure'); END"
				step := spotifyStep{operation: "artist", target: "artist", err: errors.New("transport")}
				if failure != "pending" {
					step.err = nil
					step.artist = testArtist()
				}
				if failure == "enrichment" {
					trigger = "CREATE TRIGGER reject_adoption BEFORE UPDATE OF total_tracks ON albums BEGIN SELECT RAISE(ABORT,'enrichment failure'); END"
				}
				if failure == "merge" {
					_, err = s.queries.UpsertMusician(context.Background(), database.UpsertMusicianParams{Name: "Owner", SortName: "Owner", SpotifyID: helpers.NullString("artist")})
					if err != nil {
						t.Fatal(err)
					}
					trigger = "CREATE TRIGGER reject_adoption BEFORE DELETE ON musicians BEGIN SELECT RAISE(ABORT,'merge failure'); END"
				}
				_, err = s.tx.DB.Exec(trigger)
				if err != nil {
					t.Fatal(err)
				}
				s.spotify = scriptedSpotify(t, spotifyStep{operation: "match", target: "Test Album", album: testAlbum()}, step)
				invalidations := 0
				s.invalidateCommittedTrack = func(int64) { invalidations++ }
				scan := newMusicScanContext(nil)
				if initial {
					_, _, failures := s.processMusicFixtureBatch(t, context.Background(), scan, []scanner.ScanFile{{Path: path, Ext: "m4a", Size: 5}})
					if failures != 1 {
						t.Fatalf("failures=%d", failures)
					}
				} else {
					err = s.retrySpotify(context.Background(), scan, newScanReport(Status{}))
					if err == nil {
						t.Fatal("expected rollback")
					}
				}
				if invalidations != 0 || scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_spotify_adoptions") != 0 || scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM albums WHERE spotify_id IS NOT NULL") != 0 {
					t.Fatal("transaction published partial enrichment")
				}
				if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM musicians WHERE name='Test Artist' AND spotify_id IS NULL") != 1 {
					t.Fatal("rollback lost prior musician")
				}
				_, cachedAlbum := scan.albumIDs.Get(scanner.NormalizedScanCacheKey("Test Album", "Test Artist"))
				if cachedAlbum {
					t.Fatal("rollback published album ID")
				}
				// Resolution can cache a pre-existing ID; it must never publish the merged owner.
				id, ok := scan.musicianIDs.Get(scanner.NormalizedScanCacheKey("Test Artist"))
				if ok && id != 1 {
					t.Fatalf("rollback published musician %d", id)
				}
				_, err = s.tx.DB.Exec("DROP TRIGGER reject_adoption")
				if err != nil {
					t.Fatal(err)
				}
				// Reuse the provider results after rollback, with no further requests.
				if initial {
					imported, _, failures := s.processMusicFixtureBatch(t, context.Background(), scan, []scanner.ScanFile{{Path: path, Ext: "m4a", Size: 5}})
					if imported != 1 || failures != 0 {
						t.Fatalf("retry imported=%d failures=%d", imported, failures)
					}
				} else {
					err = s.retrySpotify(context.Background(), scan, newScanReport(Status{}))
					if err != nil {
						t.Fatal(err)
					}
				}
				if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM albums WHERE spotify_id='album'") != 1 {
					t.Fatal("retry did not commit album")
				}
				if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM pragma_foreign_key_check") != 0 {
					t.Fatal("foreign key violation")
				}
			})
		}
	}
}

func TestSpotifyRecoveryIDRequestDeduplication(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(fmt.Sprintf("failed=%v", failed), func(t *testing.T) {
			s := setupMusicScanner(t)
			dir := t.TempDir()
			results := map[string]*ffprobe.FfprobeResult{}
			names := []string{"Beatles", "The Beatles", "Beatles"}
			var steps []spotifyStep
			for i, name := range names {
				title := fmt.Sprintf("Album %d", i)
				path := scannertest.WriteFile(t, filepath.Join(dir, fmt.Sprintf("%d.m4a", i)), "audio")
				results[path] = testMusicMetadataWithTags(ffprobe.FormatTags{Title: "Song", Artist: name, Album: title})
				if i < 2 {
					steps = append(steps, spotifyStep{operation: "search", target: name, err: nameMiss()})
				}
				album := testAlbum()
				album.ID = spotifyapi.ID(fmt.Sprintf("album%d", i))
				album.Artists = []spotifyapi.SimpleArtist{{ID: "beatles", Name: "The Beatles"}}
				steps = append(steps, spotifyStep{operation: "match", target: title, album: album})
				if i == 0 {
					step := spotifyStep{operation: "artist", target: "beatles", artist: &spotifyapi.FullArtist{SimpleArtist: spotifyapi.SimpleArtist{ID: "beatles", Name: "The Beatles"}}}
					if failed {
						step.artist = nil
						step.err = errors.New("transport")
					}
					steps = append(steps, step)
				}
			}
			s.ffprobe = newMusicScannerFfprobeByPath(results)
			s.spotify = scriptedSpotify(t, steps...)
			s.scan(dir)
			want := 1
			if failed {
				want = 2
			}
			if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM musicians") != want {
				t.Fatalf("incorrect musician count, status=%+v", s.Status())
			}
			if failed && scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_spotify_adoptions WHERE spotify_id='beatles'") != 2 {
				t.Fatal("aliases lost pending targets")
			}
			if !failed && scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM tracks WHERE musician_id=(SELECT id FROM musicians WHERE spotify_id='beatles')") != 3 {
				t.Fatal("merge did not preserve all tracks")
			}
		})
	}
}

func TestSpotifyRecoveryTrackListOutcomes(t *testing.T) {
	for _, outcome := range []string{"empty", "gone", "transient"} {
		t.Run(outcome, func(t *testing.T) {
			s := setupMusicScanner(t)
			scan := newMusicScanContext(nil)
			path := filepath.Join(t.TempDir(), "one.m4a")
			scanTaggedTrack(t, s, scan, path, 1, ffprobe.FormatTags{Title: "Song", Album: "Local"})
			_, err := s.tx.DB.Exec("UPDATE albums SET spotify_id='album'")
			if err != nil {
				t.Fatal(err)
			}
			candidates, err := s.queries.MusicTrackLinkAlbumCandidates(context.Background(), 0)
			if err != nil || len(candidates) != 1 {
				t.Fatalf("candidates=%v %v", candidates, err)
			}
			step := spotifyStep{operation: "album", target: "album", album: testAlbum()}
			if outcome == "gone" {
				step.album = nil
				step.err = &spotifyapi.StatusError{StatusCode: 404}
			}
			if outcome == "transient" {
				step.album = nil
				step.err = errors.New("transport")
			}
			s.spotify = scriptedSpotify(t, step)
			for i := 0; i < 3; i++ {
				candidate := candidates[0]
				candidate.Title = fmt.Sprintf("Alias %d", i)
				if i == 2 {
					scan.breaker.Stopped = true
				}
				err = s.linkAlbum(context.Background(), scan, candidate)
				if err != nil {
					t.Fatal(err)
				}
			}
			link := trackLinkFor(t, s, path)
			if outcome == "transient" {
				if link.status.Valid {
					t.Fatalf("transient settled track: %+v", link)
				}
			} else if link.status.String != musicStatusUnmatched {
				t.Fatalf("empty list did not settle track: %+v", link)
			}
		})
	}
}

func TestSpotifyRecoveryPendingPagination(t *testing.T) {
	s := setupMusicScanner(t)
	dir := t.TempDir()
	results := map[string]*ffprobe.FfprobeResult{}
	var steps []spotifyStep
	for i := 0; i < 105; i++ {
		name := fmt.Sprintf("Artist %03d", i)
		path := scannertest.WriteFile(t, filepath.Join(dir, fmt.Sprintf("%03d.m4a", i)), "audio")
		results[path] = testMusicMetadataWithTags(ffprobe.FormatTags{Title: "Song", Artist: name})
		steps = append(steps, spotifyStep{operation: "artist", target: name, err: &spotifyapi.StatusError{StatusCode: 404}})
	}
	probe := newMusicScannerFfprobeByPath(results)
	s.ffprobe = probe
	s.scan(dir)
	_, err := s.tx.DB.Exec(`
 INSERT INTO musicians(name,sort_name) VALUES('Orphan','Orphan');
 INSERT INTO music_provider_matches(provider,entity_type,entity_id,status,reason) SELECT 'spotify','musician',id,'unmatched','no_results' FROM musicians;
 INSERT INTO music_spotify_adoptions SELECT id,name FROM musicians;
 `)
	if err != nil {
		t.Fatal(err)
	}
	count, err := s.queries.CountMusicArtistRetryCandidates(context.Background())
	if err != nil || count != 105 {
		t.Fatalf("candidates=%d %v", count, err)
	}
	page, err := s.queries.MusicArtistRetryCandidates(context.Background(), 0)
	if err != nil || len(page) != 100 {
		t.Fatalf("first page=%d %v", len(page), err)
	}
	page, err = s.queries.MusicArtistRetryCandidates(context.Background(), page[99].ID)
	if err != nil || len(page) != 5 {
		t.Fatalf("second page=%d %v", len(page), err)
	}
	s.spotify = scriptedSpotify(t, steps...)
	s.scan(dir)
	if s.Status().EnrichmentProcessed != 105 || probe.totalAudioCalls() != 105 {
		t.Fatalf("status=%+v probes=%d", s.Status(), probe.totalAudioCalls())
	}
	if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_spotify_adoptions") != 1 {
		t.Fatal("terminal lookups did not clear pending rows")
	}
	_, err = s.tx.DB.Exec("DELETE FROM musicians WHERE name='Orphan'")
	if err != nil {
		t.Fatal(err)
	}
	if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_spotify_adoptions") != 0 {
		t.Fatal("orphan deletion did not cascade")
	}
	s.scan(dir)
}

func TestSpotifyRecoveryBreakerDefersAdoption(t *testing.T) {
	s := setupMusicScanner(t)
	dir := t.TempDir()
	path := scannertest.WriteFile(t, filepath.Join(dir, "track.m4a"), "audio")
	s.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadata()}
	s.scan(dir)
	markArtistsUnmatched(t, s)
	scan := newMusicScanContext(nil)
	s.spotify = scriptedSpotify(t, spotifyStep{operation: "match", target: "Test Album", album: testAlbum()})
	album, err := s.resolveAlbum(context.Background(), scan, albumQuery{title: "Test Album", artist: "Test Artist"})
	if err != nil {
		t.Fatal(err)
	}
	scan.breaker.Stopped = true
	adopted, err := s.adoptAlbumArtist(context.Background(), scan, "Test Artist", album.spotifyAlbum)
	if err != nil {
		t.Fatal(err)
	}
	if adopted == nil || adopted.spotifyMatch != nil {
		t.Fatalf("skipped request recorded an outcome: %+v", adopted)
	}
	err = s.persistEnrichment(context.Background(), scan, func(q *database.Queries, txScan *musicScanContext) error {
		_, err := s.persistMusician(context.Background(), q, txScan, *adopted)
		if err != nil {
			return err
		}
		_, err = s.persistAlbum(context.Background(), q, txScan, *album)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	linkScannedTracks(t, s, scan)
	if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_provider_matches WHERE entity_type='musician' AND status='unmatched'") != 1 {
		t.Fatal("deferral changed final name miss")
	}
	if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_spotify_adoptions") != 1 {
		t.Fatal("deferred target lost")
	}
	s.spotify = scriptedSpotify(t, spotifyStep{operation: "artist", target: "artist", artist: testArtist()})
	s.scan(dir)
	if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM musicians WHERE spotify_id='artist'") != 1 {
		t.Fatal("deferred adoption did not resume")
	}
	if trackLinkFor(t, s, path).status.String != musicStatusUnmatched {
		t.Fatal("cached empty album list did not settle track")
	}
}

func TestSpotifyRecoveryPendingBreaker(t *testing.T) {
	s := setupMusicScanner(t)
	dir := t.TempDir()
	results := map[string]*ffprobe.FfprobeResult{}
	for i := 0; i < 4; i++ {
		path := scannertest.WriteFile(t, filepath.Join(dir, fmt.Sprintf("%d.m4a", i)), "audio")
		results[path] = testMusicMetadataWithTags(ffprobe.FormatTags{Title: "Song", Artist: fmt.Sprintf("Artist %d", i)})
	}
	s.ffprobe = newMusicScannerFfprobeByPath(results)
	s.scan(dir)
	_, err := s.tx.DB.Exec(`INSERT INTO music_provider_matches(provider,entity_type,entity_id,status) SELECT 'spotify','musician',id,'unmatched' FROM musicians;
 INSERT INTO music_spotify_adoptions SELECT id,name FROM musicians;`)
	if err != nil {
		t.Fatal(err)
	}
	var steps []spotifyStep
	for i := 0; i < 3; i++ {
		steps = append(steps, spotifyStep{operation: "artist", target: fmt.Sprintf("Artist %d", i), err: errors.New("transport")})
	}
	s.spotify = scriptedSpotify(t, steps...)
	s.scan(dir)
	if s.Status().State != scanner.StateCompletedWithIssues || s.Status().EnrichmentFailed != 3 {
		t.Fatalf("status=%+v", s.Status())
	}
	if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_spotify_adoptions") != 4 || scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_provider_matches WHERE status='unmatched'") != 1 {
		t.Fatal("breaker discarded pending work or recorded skipped failure")
	}
}

func TestSpotifyRecoveryCanceledBeforePersistence(t *testing.T) {
	source := fileDatabaseSource(t)
	s := setupMusicScannerDatabase(t, source)
	dir := t.TempDir()
	scannertest.WriteFile(t, filepath.Join(dir, "track.m4a"), "audio")
	probe := &scannertest.CountingProbe{Default: testMusicMetadata()}
	s.ffprobe = probe
	s.scan(dir)
	_, err := s.tx.DB.Exec("INSERT INTO music_spotify_adoptions SELECT id,'artist' FROM musicians")
	if err != nil {
		t.Fatal(err)
	}
	s.spotify = scriptedSpotify(t, spotifyStep{operation: "artist", target: "artist", artist: testArtist()})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	scan := newMusicScanContext(nil)
	resolved, err := s.resolveMusician(ctx, scan, "Test Artist", "")
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	invalidations := 0
	s.invalidateCommittedTrack = func(int64) { invalidations++ }
	err = s.persistMusicianEnrichment(ctx, scan, resolved)
	if !errors.Is(err, context.Canceled) || invalidations != 0 {
		t.Fatalf("persistence=%v invalidations=%d", err, invalidations)
	}
	s = reopenRecoveryScanner(t, s, source)
	s.ffprobe = probe
	if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_spotify_adoptions") != 1 || scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM musicians WHERE spotify_id IS NOT NULL") != 0 {
		t.Fatal("canceled persistence altered identity")
	}
	s.spotify = scriptedSpotify(t, spotifyStep{operation: "artist", target: "artist", artist: testArtist()}, spotifyStep{operation: "match", target: "Test Album", album: testAlbum()})
	s.scan(dir)
	if probe.Calls() != 1 || s.Status().State != scanner.StateCompleted {
		t.Fatalf("recovery status=%+v probes=%d", s.Status(), probe.Calls())
	}
}

func TestSpotifyRecoveryAdoptionMergesPreserveReferences(t *testing.T) {
	s := setupMusicScanner(t)
	dir := t.TempDir()
	paths := []string{filepath.Join(dir, "1.m4a"), filepath.Join(dir, "2.m4a")}
	results := map[string]*ffprobe.FfprobeResult{}
	for i, path := range paths {
		scannertest.WriteFile(t, path, "audio")
		artist, album := "Owner", "Retained"
		if i == 1 {
			artist, album = "Test Artist", "Alias"
		}
		results[path] = testMusicMetadataWithTags(ffprobe.FormatTags{Title: fmt.Sprintf("Song %d", i+1), Artist: artist, Album: album})
	}
	probe := newMusicScannerFfprobeByPath(results)
	s.ffprobe = probe
	s.scan(dir)
	_, err := s.tx.DB.Exec(`
 UPDATE musicians SET spotify_id='artist' WHERE name='Owner';
 UPDATE albums SET spotify_id='album' WHERE title='Retained';
 INSERT INTO music_provider_matches(provider,entity_type,entity_id,status) SELECT 'spotify','musician',id,CASE WHEN spotify_id IS NULL THEN 'unmatched' ELSE 'matched' END FROM musicians;
 INSERT INTO music_provider_matches(provider,entity_type,entity_id,status) SELECT 'spotify','album',id,'matched' FROM albums WHERE spotify_id IS NOT NULL;
 INSERT INTO users(id,email,password,name) VALUES(1,'test@example.com','hash','Test');
 INSERT INTO playlists(id,name,user_id) VALUES(1,'Music',1);
 INSERT INTO playlist_tracks(playlist_id,track_id,added_by,position) VALUES(1,2,1,0);
 INSERT INTO user_play_history(user_id,track_id,duration_played) VALUES(1,2,60);
 INSERT INTO music_spotify_adoptions(musician_id,spotify_id) VALUES(2,'artist');
 `)
	if err != nil {
		t.Fatal(err)
	}
	album := testAlbum(spotifyTrack("t1", "Song 1", 1, 1, 180250, false), spotifyTrack("t2", "Song 2", 1, 2, 180250, false))
	s.spotify = scriptedSpotify(t, spotifyStep{operation: "artist", target: "artist", artist: testArtist()}, spotifyStep{operation: "match", target: "Alias", album: album})
	invalidations := map[int64]int{}
	s.invalidateCommittedTrack = func(id int64) { invalidations[id]++ }
	s.scan(dir)
	for _, query := range []string{
		"SELECT count(*) FROM musicians WHERE id=1 AND spotify_id='artist'",
		"SELECT count(*) FROM albums WHERE id=1 AND spotify_id='album'",
		"SELECT count(*) FROM playlist_tracks WHERE track_id=2",
		"SELECT count(*) FROM user_play_history WHERE track_id=2 AND duration_played=60",
	} {
		if scannertest.CountRows(t, s.tx.DB, query) != 1 {
			t.Fatalf("lost reference: %s", query)
		}
	}
	for _, query := range []string{
		"SELECT count(*) FROM tracks WHERE id IN (1,2) AND album_id=1 AND musician_id=1 AND spotify_id IS NOT NULL",
		"SELECT count(*) FROM music_artist_identity WHERE musician_id=1",
		"SELECT count(*) FROM music_album_identity WHERE album_id=1",
	} {
		if scannertest.CountRows(t, s.tx.DB, query) != 2 {
			t.Fatalf("lost identity: %s", query)
		}
	}
	if len(invalidations) != 2 || invalidations[2] < 2 {
		t.Fatalf("missing post-commit invalidations: %v", invalidations)
	}
	if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_spotify_adoptions") != 0 || scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM pragma_foreign_key_check") != 0 {
		t.Fatal("merge left pending work or broken references")
	}
	assertLinked(t, trackLinkFor(t, s, paths[0]), "t1", false, 1)
	assertLinked(t, trackLinkFor(t, s, paths[1]), "t2", false, 2)
	s.scan(dir)
	if probe.totalAudioCalls() != 2 {
		t.Fatal("merge recovery re-probed files")
	}
}

func TestSpotifyRecoveryAlbumArtistWithoutTrackCredit(t *testing.T) {
	s := setupMusicScanner(t)
	dir := t.TempDir()
	path := scannertest.WriteFile(t, filepath.Join(dir, "track.m4a"), "audio")
	s.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadataWithTags(ffprobe.FormatTags{Title: "Song", Artist: "Guest", Album: "Test Album", AlbumArtist: "Test Artist"})}
	s.scan(dir)
	_, err := s.tx.DB.Exec(`
 INSERT INTO musicians(name,sort_name) VALUES('Test Artist','Test Artist');
 INSERT INTO music_artist_identity(identity_key,musician_id) SELECT 'test artist',id FROM musicians WHERE name='Test Artist';
 INSERT INTO music_provider_matches(provider,entity_type,entity_id,status) SELECT 'spotify','musician',id,'unmatched' FROM musicians;
 `)
	if err != nil {
		t.Fatal(err)
	}
	s.spotify = scriptedSpotify(t, spotifyStep{operation: "match", target: "Test Album", album: testAlbum()}, spotifyStep{operation: "artist", target: "artist", err: errors.New("transport")})
	s.scan(dir)
	count, err := s.queries.CountMusicArtistRetryCandidates(context.Background())
	if err != nil || count != 1 {
		t.Fatalf("uncredited album artist retry count=%d %v", count, err)
	}
	s.spotify = scriptedSpotify(t, spotifyStep{operation: "artist", target: "artist", artist: testArtist()})
	s.scan(dir)
	if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM musicians WHERE name='Test Artist' AND spotify_id='artist'") != 1 {
		t.Fatal("uncredited album artist did not recover")
	}
	if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM track_musicians tm JOIN musicians m ON m.id=tm.musician_id WHERE m.name='Guest'") != 1 {
		t.Fatal("adoption changed track credits")
	}
	if trackLinkFor(t, s, path).status.String != musicStatusUnmatched {
		t.Fatal("empty album tracks not settled")
	}
}

// An alias created this scan merges into the album already holding its
// Spotify ID; when the link pass drops that match, the alias's tally moves
// to unmatched instead of a second outcome being counted for the owner.
func TestSpotifyAlbumAliasContradictionCountsOnce(t *testing.T) {
	s := setupMusicScanner(t)
	ctx := context.Background()
	_, err := s.queries.UpsertAlbum(ctx, database.UpsertAlbumParams{Title: "Retained", SortTitle: "Retained", SpotifyID: helpers.NullString("album")})
	if err != nil {
		t.Fatal(err)
	}
	s.spotify = scriptedSpotify(t, spotifyStep{operation: "match", target: "Alias", album: testAlbum(spotifyTrack("other", "Unrelated", 1, 1, 1000, false))})
	scan := newMusicScanContext(nil)
	for i := 0; i < 3; i++ {
		scanTaggedTrack(t, s, scan, filepath.Join(t.TempDir(), "track.m4a"), 1, ffprobe.FormatTags{Title: fmt.Sprintf("Local %d", i), Album: "Alias"})
	}
	linkScannedTracks(t, s, scan)
	if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM albums WHERE spotify_id IS NOT NULL") != 0 {
		t.Fatal("contradicted album kept Spotify identity")
	}
	if scan.enrichmentCounts[musicStatusMatched] != 0 || scan.enrichmentCounts[musicStatusUnmatched] != 1 {
		t.Fatalf("enrichment counts = %v, want the one album moved to unmatched", scan.enrichmentCounts)
	}
}

func TestSpotifyRecoveryPreservesMatchedMusician(t *testing.T) {
	s := setupMusicScanner(t)
	dir := t.TempDir()
	scannertest.WriteFile(t, filepath.Join(dir, "track.m4a"), "audio")
	s.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadataWithTags(ffprobe.FormatTags{Title: "Song", Artist: "Test Artist"})}
	s.scan(dir)
	_, err := s.tx.DB.Exec(`
 UPDATE musicians SET spotify_id='retained';
 INSERT INTO music_provider_matches(provider,entity_type,entity_id,status) SELECT 'spotify','musician',id,'matched' FROM musicians;
 INSERT INTO music_spotify_adoptions SELECT id,'obsolete' FROM musicians;
 `)
	if err != nil {
		t.Fatal(err)
	}
	s.spotify = scriptedSpotify(t)
	s.scan(dir)
	if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM musicians WHERE spotify_id='retained'") != 1 || scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_spotify_adoptions") != 0 {
		t.Fatal("matched identity changed or obsolete adoption not cleared")
	}
}
