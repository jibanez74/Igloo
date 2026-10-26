package music

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/scannertest"
	spotifyapi "igloo/cmd/internal/spotify"
)

// scanChanges runs one run of watcher-detected changes synchronously.
func (s *Scanner) scanChanges(directory string, changes scanner.Changes) {
	s.beginReport(changes.Scope())
	s.runMusicScan(directory, &changes)
}

func albumQueryTitles(stub *musicScannerSpotifyStub) []string {
	titles := make([]string, 0, len(stub.albumQueries))
	for _, query := range stub.albumQueries {
		titles = append(titles, query.Title)
	}
	slices.Sort(titles)
	return slices.Compact(titles)
}

func TestMusicChangesRunKeepsEnrichmentToTheTouchedAlbum(t *testing.T) {
	s := setupMusicScanner(t)
	root := t.TempDir()
	tags := func(title, artist, album, track string) *ffprobe.FfprobeResult {
		return testMusicMetadataWithTags(ffprobe.FormatTags{Title: title, Artist: artist, Album: album, Track: track})
	}
	a1 := scannertest.WriteFile(t, filepath.Join(root, "Artist A", "Album A", "01.flac"), "a1")
	a2 := filepath.Join(root, "Artist A", "Album A", "02.flac")
	b1 := scannertest.WriteFile(t, filepath.Join(root, "Artist B", "Album B", "01.flac"), "b1")
	s.ffprobe = newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{
		a1: tags("One", "Artist A", "Album A", "1"),
		a2: tags("Two", "Artist A", "Album A", "2"),
		b1: tags("Uno", "Artist B", "Album B", "1"),
	})
	// Every lookup misses the way a 404 does, so both albums stay retry
	// candidates and every scan's retry pass would search them again.
	stub := &musicScannerSpotifyStub{artistErr: entityLookupFailure, albumErr: entityLookupFailure}
	s.spotify = stub
	s.scan(root)
	if s.Status().Imported != 2 {
		t.Fatalf("library scan: %+v", s.Status().Counts)
	}

	stub.albumQueries, stub.albumCalls, stub.artistCalls = nil, 0, 0
	scannertest.WriteFile(t, a2, "a2")
	s.scanChanges(root, scanner.Changes{Paths: []string{"Artist A/Album A/02.flac"}})
	status := s.Status()
	if status.Scope != scanner.ScopeChanges || status.Total != 1 || status.Imported != 1 {
		t.Fatalf("changes run: scope=%s %+v", status.Scope, status.Counts)
	}
	titles := albumQueryTitles(stub)
	if !slices.Equal(titles, []string{"Album A"}) {
		t.Fatalf("a run of changes searched %q; only the touched album may be searched", titles)
	}

	// A library scan still retries every pending album.
	stub.albumQueries = nil
	s.scan(root)
	titles = albumQueryTitles(stub)
	if !slices.Equal(titles, []string{"Album A", "Album B"}) {
		t.Fatalf("library scan searched %q", titles)
	}
}

func TestMusicChangesRunRemovesOnlyTheReportedTracks(t *testing.T) {
	s := setupMusicScanner(t)
	root := t.TempDir()
	a1 := scannertest.WriteFile(t, filepath.Join(root, "Album A", "01.flac"), "a1")
	b1 := scannertest.WriteFile(t, filepath.Join(root, "Album B", "01.flac"), "b1")
	s.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadata()}
	s.scan(root)
	for _, path := range []string{a1, b1} {
		err := os.Remove(path)
		if err != nil {
			t.Fatal(err)
		}
	}
	s.scanChanges(root, scanner.Changes{Paths: []string{"Album A"}})
	if s.Status().Deleted != 1 || scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM tracks") != 1 {
		t.Fatalf("deleted=%d", s.Status().Deleted)
	}

	s.scanChanges(root, scanner.Changes{CatchUp: true})
	status := s.Status()
	if status.Deleted != 0 || status.IssueCount != 1 || status.Issues[0].Reason != scanner.ReasonCleanupSkippedEmpty {
		t.Fatalf("catch-up over an empty root: %+v", status)
	}
}

// A disk mounted inside the library that has not come up leaves its empty
// mount point; the catch-up keeps its tracks, with their likes and playlist
// entries.
func TestMusicCatchUpKeepsTheTracksOfADiskThatIsNotMounted(t *testing.T) {
	s := setupMusicScanner(t)
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	disk := filepath.Join(root, "disk2")
	scannertest.WriteFile(t, filepath.Join(root, "disk1", "Album A", "01.flac"), "a1")
	scannertest.WriteFile(t, filepath.Join(disk, "Album B", "01.flac"), "b1")
	s.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadata()}
	s.mountPoints = func() ([]string, error) { return []string{disk}, nil }
	s.scan(root)
	err = os.RemoveAll(filepath.Join(disk, "Album B"))
	if err != nil {
		t.Fatal(err)
	}
	s.mountPoints = func() ([]string, error) { return nil, nil }

	s.scanChanges(root, scanner.Changes{CatchUp: true})
	status := s.Status()
	if status.Deleted != 0 || scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM tracks") != 2 {
		t.Fatalf("catch-up removed tracks: deleted=%d", status.Deleted)
	}
	if status.IssueCount != 1 || status.Issues[0].Reason != scanner.ReasonCleanupKeptUnmounted {
		t.Fatalf("issues = %+v", status.Issues)
	}
}

func TestMusicChangesRunHandsBackDeferredTracks(t *testing.T) {
	s := setupMusicScanner(t)
	root := t.TempDir()
	path := scannertest.WriteFile(t, filepath.Join(root, "Album", "01.flac"), "fresh")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	s.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadata()}
	s.now = func() time.Time { return info.ModTime().Add(time.Second) }
	var deferred []string
	s.scanChanges(root, scanner.Changes{
		Paths: []string{"Album/01.flac"},
		OnDeferred: func(path string, eligible time.Time) {
			if !eligible.IsZero() {
				t.Errorf("music hands back no eligible time; got %v", eligible)
			}
			deferred = append(deferred, path)
		},
	})
	status := s.Status()
	if status.Deferred != 1 || status.Imported != 0 || !slices.Equal(deferred, []string{"Album/01.flac"}) {
		t.Fatalf("deferred=%q %+v", deferred, status.Counts)
	}
	if status.Issues[0].Reason != scanner.ReasonFileDeferredWatched {
		t.Fatalf("issue = %+v", status.Issues)
	}
}

func TestMusicChangesRunFollowsAlbumMergesIntoTheOwner(t *testing.T) {
	s := setupMusicScanner(t)
	root := t.TempDir()
	tags := func(title, album, track string) *ffprobe.FfprobeResult {
		return testMusicMetadataWithTags(ffprobe.FormatTags{Title: title, Artist: "Artist A", Album: album, Track: track})
	}
	a1 := scannertest.WriteFile(t, filepath.Join(root, "Artist A", "Album A", "01.flac"), "a1")
	a2 := scannertest.WriteFile(t, filepath.Join(root, "Artist A", "Album A (Deluxe)", "02.flac"), "a2")
	s.ffprobe = newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{
		a1: tags("One", "Album A", "1"),
		a2: tags("Two", "Album A (Deluxe)", "2"),
	})
	release := &spotifyapi.FullAlbum{
		SimpleAlbum: spotifyapi.SimpleAlbum{ID: "album", Name: "Album A", AlbumType: "album", Artists: []spotifyapi.SimpleArtist{{ID: "artist", Name: "Artist A"}}},
		Tracks: []spotifyapi.SimpleTrack{
			spotifyTrack("t1", "One", 1, 1, 180250, false),
			spotifyTrack("t2", "Two", 1, 2, 180250, false),
		},
	}
	// The deluxe title misses the way a 404 does in the library scan, so its
	// album stays a retry candidate.
	stub := &musicScannerSpotifyStub{
		artist:        &spotifyapi.FullArtist{SimpleArtist: spotifyapi.SimpleArtist{ID: "artist", Name: "Artist A"}},
		albumsByTitle: map[string]*spotifyapi.FullAlbum{"album a": release},
	}
	s.spotify = stub
	s.scan(root)
	if albums := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM albums"); albums != 2 {
		t.Fatalf("library scan albums = %d, want the two local albums", albums)
	}
	assertLinked(t, trackLinkFor(t, s, a1), "t1", false, 1)

	// The run reconsiders the deluxe folder unchanged; its retry pass now
	// matches the album to the owner's release, which merges the two.
	stub.albumsByTitle["album a (deluxe)"] = release
	s.scanChanges(root, scanner.Changes{Paths: []string{"Artist A/Album A (Deluxe)"}})
	status := s.Status()
	if status.Scope != scanner.ScopeChanges || status.Unchanged != 1 || status.State != scanner.StateCompleted {
		t.Fatalf("changes run: scope=%s state=%s %+v", status.Scope, status.State, status.Counts)
	}
	if albums := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM albums"); albums != 1 {
		t.Fatalf("albums after the merge = %d, want one owner", albums)
	}
	// The merge moved the track to the owner; the run's track pairing must
	// follow it there rather than keep to the album it deleted.
	assertLinked(t, trackLinkFor(t, s, a2), "t2", false, 2)
}

func TestAlbumRetryCandidatesInBindsEveryTouchedAlbum(t *testing.T) {
	s := setupMusicScanner(t)
	// Three pending albums; the last two carry Apple catalog hints, so they
	// wait for Apple only while Apple lookups are enabled.
	_, err := s.tx.DB.Exec(`
		INSERT INTO albums(id, title, sort_title) VALUES (1, 'One', 'One'), (2, 'Two', 'Two'), (3, 'Three', 'Three');
		INSERT INTO tracks(title, sort_title, file_path, file_name, container, mime_type, codec, size, track_index, duration, disc, channels, channel_layout, bit_rate, profile, album_id)
		SELECT 'Track', 'Track', '/music/' || id || '.m4a', id || '.m4a', 'm4a', 'audio/mp4', 'aac', 1, 1, 1, 1, '2', 'stereo', 1, 'LC', id FROM albums;
		INSERT INTO music_track_metadata(track_id, artist_tag, artist_key, artist_sort, album_sort, apple_album_id)
		SELECT id, '', '', '', '', 1000 + id FROM tracks WHERE album_id > 1;`)
	if err != nil {
		t.Fatal(err)
	}
	candidates := func(appleEnabled bool, albums []int64) []int64 {
		t.Helper()
		scan := newMusicScanContext(nil)
		scan.appleEnabled = appleEnabled
		scan.scope = &musicScope{albums: albums}
		rows, err := s.albumRetryCandidates(context.Background(), scan, 0)
		if err != nil {
			t.Fatalf("retry candidates of %v: %v", albums, err)
		}
		ids := make([]int64, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.ID)
		}
		return ids
	}
	got := candidates(false, []int64{1, 2, 3})
	if !slices.Equal(got, []int64{1, 2, 3}) {
		t.Fatalf("with Apple off, every touched album is a Spotify candidate; got %v", got)
	}
	got = candidates(true, []int64{1, 2, 3})
	if !slices.Equal(got, []int64{1}) {
		t.Fatalf("with Apple on, hinted albums wait for Apple; got %v", got)
	}
	got = candidates(false, nil)
	if len(got) != 0 {
		t.Fatalf("a run that touched no album has no candidates; got %v", got)
	}
}

func TestMusicChangesRunThatOnlyRemovesTracksCompletes(t *testing.T) {
	s := setupMusicScanner(t)
	root := t.TempDir()
	path := scannertest.WriteFile(t, filepath.Join(root, "Album", "01.flac"), "a1")
	s.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadata()}
	s.spotify = &musicScannerSpotifyStub{artistErr: entityLookupFailure, albumErr: entityLookupFailure}
	s.scan(root)
	err := os.Remove(path)
	if err != nil {
		t.Fatal(err)
	}
	s.scanChanges(root, scanner.Changes{Paths: []string{"Album/01.flac"}})
	status := s.Status()
	if status.State != scanner.StateCompleted || status.Deleted != 1 {
		t.Fatalf("deletion-only run with Spotify configured: state=%s %+v issues=%+v", status.State, status.Counts, status.Issues)
	}
}
