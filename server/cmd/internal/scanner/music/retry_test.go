package music

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
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

// A compound credit that stayed combined offline and later missed on Spotify
// is split by the reconciliation pass that runs after the retry loops, even
// when no retry candidate is left to trigger it. A canceled context stops
// before touching credits, and a closed database surfaces the query error.
func TestReconcilePendingCompoundCreditsSplitsCommittedMisses(t *testing.T) {
	s := setupMusicScanner(t)
	ctx := context.Background()
	scanTaggedTrack(t, s, newMusicScanContext(nil), filepath.Join(t.TempDir(), "one"), 1, ffprobe.FormatTags{Title: "One", Artist: "One & Two"})
	if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM musicians") != 1 {
		t.Fatal("offline scan split the compound credit early")
	}
	_, err := s.tx.DB.Exec("INSERT INTO music_provider_matches(provider, entity_type, entity_id, status, reason) SELECT 'spotify', 'musician', id, 'unmatched', 'no_results' FROM musicians")
	if err != nil {
		t.Fatal(err)
	}

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	err = s.reconcilePendingCompoundCredits(canceled, newMusicScanContext(nil))
	if !errors.Is(err, context.Canceled) || scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM musicians") != 1 {
		t.Fatalf("canceled reconciliation: err=%v", err)
	}

	err = s.reconcilePendingCompoundCredits(ctx, newMusicScanContext(nil))
	if err != nil {
		t.Fatal(err)
	}
	split := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM track_musicians tm JOIN musicians m ON m.id = tm.musician_id WHERE m.name IN ('One', 'Two')")
	combined := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM track_musicians tm JOIN musicians m ON m.id = tm.musician_id WHERE m.name = 'One & Two'")
	if split != 2 || combined != 0 {
		t.Fatalf("credits after reconciliation: split=%d combined=%d", split, combined)
	}

	err = s.tx.DB.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = s.reconcilePendingCompoundCredits(ctx, newMusicScanContext(nil))
	if err == nil {
		t.Fatal("closed database did not fail the candidate query")
	}
}

func spotifyTrack(id, name string, disc, number, durationMs int, explicit bool) spotifyapi.SimpleTrack {
	return spotifyapi.SimpleTrack{ID: spotifyapi.ID(id), Name: name, DiscNumber: spotifyapi.Numeric(disc), TrackNumber: spotifyapi.Numeric(number), Duration: spotifyapi.Numeric(durationMs), Explicit: explicit}
}

// testAlbum is the stub's matched album, credited to the stub artist.
func testAlbum(tracks ...spotifyapi.SimpleTrack) *spotifyapi.FullAlbum {
	return &spotifyapi.FullAlbum{
		SimpleAlbum: spotifyapi.SimpleAlbum{ID: "album", Name: "Test Album", AlbumType: "album", Artists: []spotifyapi.SimpleArtist{{ID: "artist", Name: "Test Artist"}}},
		Tracks:      tracks,
	}
}

func testArtist() *spotifyapi.FullArtist {
	return &spotifyapi.FullArtist{SimpleArtist: spotifyapi.SimpleArtist{ID: "artist", Name: "Test Artist"}}
}

// albumTags are the tags every track of the stub album shares; the fixture's
// probed length is 180250 ms.
func albumTags(title, track string) ffprobe.FormatTags {
	return ffprobe.FormatTags{Title: title, Artist: "Test Artist", Album: "Test Album", Track: track}
}

type trackLinkRow struct {
	spotifyID  sql.NullString
	explicit   bool
	trackIndex int64
	status     sql.NullString
	reason     sql.NullString
	candidate  sql.NullString
	score      sql.NullInt64
}

func trackLinkFor(t *testing.T, s *Scanner, path string) trackLinkRow {
	t.Helper()
	var row trackLinkRow
	err := s.tx.DB.QueryRow(`
		SELECT t.spotify_id, t.explicit, t.track_index, m.status, m.reason, m.candidate, m.score
		FROM tracks t LEFT JOIN music_provider_matches m ON m.entity_type = 'track' AND m.entity_id = t.id
		WHERE t.file_path = ?`, path).Scan(&row.spotifyID, &row.explicit, &row.trackIndex, &row.status, &row.reason, &row.candidate, &row.score)
	if err != nil {
		t.Fatalf("track link for %s: %v", path, err)
	}
	return row
}

// linkScannedTracks runs the link pass that pairs the tracks scan persisted,
// from the track lists scan's album matches brought along.
func linkScannedTracks(t *testing.T, s *Scanner, scan *musicScanContext) {
	t.Helper()
	err := s.linkAlbumTracks(context.Background(), scan, newScanReport(Status{}))
	if err != nil {
		t.Fatalf("link album tracks: %v", err)
	}
}

func assertLinked(t *testing.T, row trackLinkRow, spotifyID string, explicit bool, trackIndex int64) {
	t.Helper()
	if row.spotifyID.String != spotifyID || row.explicit != explicit || row.trackIndex != trackIndex || row.status.String != musicStatusMatched {
		t.Fatalf("link = %+v, want %s explicit=%v index=%d matched", row, spotifyID, explicit, trackIndex)
	}
}

func TestTrackLinkingPairsTracksWithTheAlbumList(t *testing.T) {
	fixtureDir := t.TempDir()
	s := setupMusicScanner(t)
	stub := &musicScannerSpotifyStub{artist: testArtist(), album: testAlbum(
		spotifyTrack("t1", "Opening Song", 1, 1, 180250, true),
		spotifyTrack("t2", "Second Song", 1, 2, 181000, false),
		spotifyTrack("t3", "Third Song", 1, 3, 240000, false),
	)}
	s.spotify = stub
	scan := newMusicScanContext(nil)

	byPosition := fixtureDir + "/01.m4a"
	scanTaggedTrack(t, s, scan, byPosition, 1, albumTags("Opening Song", "1/3"))
	// A file-name title still pairs by position when the length agrees.
	byLength := fixtureDir + "/02.m4a"
	scanTaggedTrack(t, s, scan, byLength, 1, albumTags("02 - Track 2", "2"))
	// Without a track number the title decides, and Spotify's number fills the gap.
	byTitle := fixtureDir + "/03.m4a"
	scanTaggedTrack(t, s, scan, byTitle, 1, albumTags("Third Song", ""))
	linkScannedTracks(t, s, scan)

	assertLinked(t, trackLinkFor(t, s, byPosition), "t1", true, 1)
	assertLinked(t, trackLinkFor(t, s, byLength), "t2", false, 2)
	assertLinked(t, trackLinkFor(t, s, byTitle), "t3", false, 3)
	if scan.tracksLinked != 3 || stub.albumCalls != 1 || stub.albumFetches != 0 {
		t.Fatalf("linked=%d album searches=%d fetches=%d, want 3 tracks from one search", scan.tracksLinked, stub.albumCalls, stub.albumFetches)
	}
}

func TestTrackLinkingRecordsExtrasAsUnmatched(t *testing.T) {
	fixtureDir := t.TempDir()
	s := setupMusicScanner(t)
	stub := &musicScannerSpotifyStub{artist: testArtist(), album: testAlbum(spotifyTrack("t1", "Opening Song", 1, 1, 180250, false))}
	s.spotify = stub
	ctx := context.Background()

	extra := fixtureDir + "/09.m4a"
	scan := newMusicScanContext(nil)
	scanTaggedTrack(t, s, scan, extra, 1, albumTags("Hidden Bonus Jam", "9"))
	linkScannedTracks(t, s, scan)
	row := trackLinkFor(t, s, extra)
	if row.spotifyID.Valid || row.status.String != musicStatusUnmatched || row.reason.String != spotifyapi.MatchReasonScoreBelowThreshold {
		t.Fatalf("extra track = %+v, want an unmatched score_below_threshold outcome", row)
	}
	if row.candidate.String != "Opening Song" || !row.score.Valid || row.score.Int64 >= 70 {
		t.Fatalf("extra track = %+v, want the nearest title recorded with its score", row)
	}

	// The outcome is final: a later retry pass neither searches nor fetches.
	err := s.retrySpotify(ctx, newMusicScanContext(nil), newScanReport(Status{}))
	if err != nil {
		t.Fatal(err)
	}
	if stub.albumCalls != 1 || stub.albumFetches != 0 {
		t.Fatalf("album searches=%d fetches=%d, want no further requests", stub.albumCalls, stub.albumFetches)
	}
}

func TestTrackLinkingGivesEachSpotifyTrackToOneFile(t *testing.T) {
	fixtureDir := t.TempDir()
	s := setupMusicScanner(t)
	s.spotify = &musicScannerSpotifyStub{artist: testArtist(), album: testAlbum(
		spotifyTrack("t1", "Chan Chan", 1, 1, 180250, false),
		spotifyTrack("t2", "Second Song", 1, 2, 240000, false),
	)}
	scan := newMusicScanContext(nil)
	original := fixtureDir + "/01.m4a"
	scanTaggedTrack(t, s, scan, original, 1, albumTags("Chan Chan", "1"))
	bonus := fixtureDir + "/03.m4a"
	scanTaggedTrack(t, s, scan, bonus, 1, albumTags("Chan Chan (Exclusivo)", "3"))
	duplicate := fixtureDir + "/01 copy.m4a"
	scanTaggedTrack(t, s, scan, duplicate, 1, albumTags("Chan Chan", "1"))
	linkScannedTracks(t, s, scan)

	assertLinked(t, trackLinkFor(t, s, original), "t1", false, 1)
	// A copy of the same file is the same recording and shares the track.
	assertLinked(t, trackLinkFor(t, s, duplicate), "t1", false, 1)
	row := trackLinkFor(t, s, bonus)
	if row.spotifyID.Valid || row.status.String != musicStatusUnmatched || row.reason.String != spotifyapi.MatchReasonTrackClaimed {
		t.Fatalf("bonus track = %+v, want an unmatched track_claimed outcome", row)
	}
	if row.candidate.String != "Chan Chan" || !row.score.Valid || row.score.Int64 < 70 {
		t.Fatalf("bonus track = %+v, want the track it lost recorded with its score", row)
	}
}

// albumMatchRow is what an album match stores on the album.
type albumMatchRow struct {
	spotifyID, cover, albumType, spotifyDate, releaseDate, status, reason sql.NullString
	totalTracks                                                           sql.NullInt64
}

func albumMatchFor(t *testing.T, s *Scanner, title string) albumMatchRow {
	t.Helper()
	var row albumMatchRow
	err := s.tx.DB.QueryRow(`
		SELECT a.spotify_id, a.cover, a.album_type, a.total_tracks, md.spotify_date, a.release_date, m.status, m.reason
		FROM albums a
		LEFT JOIN music_album_metadata md ON md.album_id = a.id
		LEFT JOIN music_provider_matches m ON m.entity_type = 'album' AND m.entity_id = a.id
		WHERE a.title = ?`, title).Scan(&row.spotifyID, &row.cover, &row.albumType, &row.totalTracks, &row.spotifyDate, &row.releaseDate, &row.status, &row.reason)
	if err != nil {
		t.Fatalf("album match for %s: %v", title, err)
	}
	return row
}

func TestTrackLinkingDropsAnAlbumMatchItsTracksContradict(t *testing.T) {
	fixtureDir := t.TempDir()
	s := setupMusicScanner(t)
	// A same-titled album whose tracks are none of the files'.
	album := testAlbum(
		spotifyTrack("x1", "Malena", 1, 1, 240000, false),
		spotifyTrack("x2", "Negracha", 1, 2, 250000, true),
	)
	album.Images = []spotifyapi.Image{{URL: "https://i.scdn.co/wrong.jpg"}}
	album.TotalTracks = 2
	album.ReleaseDate = "2011-09-16"
	album.ReleaseDatePrecision = "day"
	stub := &musicScannerSpotifyStub{artist: testArtist(), album: album}
	s.spotify = stub
	scan := newMusicScanContext(nil)
	var paths []string
	for i, title := range []string{"Forget You", "Nothin' On You", "California Gurls"} {
		path := fmt.Sprintf("%s/%02d.m4a", fixtureDir, i+1)
		scanTaggedTrack(t, s, scan, path, 1, albumTags(title, strconv.Itoa(i+1)))
		paths = append(paths, path)
	}
	linkScannedTracks(t, s, scan)

	row := albumMatchFor(t, s, "Test Album")
	cleared := !row.spotifyID.Valid && !row.cover.Valid && !row.albumType.Valid && !row.totalTracks.Valid && !row.spotifyDate.Valid && !row.releaseDate.Valid
	if !cleared || row.status.String != musicStatusUnmatched || row.reason.String != musicSpotifyReasonTracksDisagree {
		t.Fatalf("album = %+v, want its Spotify fields cleared and an unmatched tracks_disagree outcome", row)
	}
	for _, path := range paths {
		link := trackLinkFor(t, s, path)
		if link.spotifyID.Valid || link.explicit || link.status.Valid {
			t.Fatalf("track %s = %+v, want no pairing and no outcome", path, link)
		}
	}
	if scan.enrichmentCounts[musicStatusMatched] != 1 || scan.enrichmentCounts[musicStatusUnmatched] != 1 {
		t.Fatalf("enrichment counts = %v, want the artist matched and the album moved to unmatched", scan.enrichmentCounts)
	}

	// The outcome is final: a later scan neither searches for the album again
	// nor fetches its tracks.
	err := s.retrySpotify(context.Background(), newMusicScanContext(nil), newScanReport(Status{}))
	if err != nil {
		t.Fatal(err)
	}
	if stub.albumCalls != 1 || stub.albumFetches != 0 {
		t.Fatalf("album searches=%d fetches=%d, want no further requests", stub.albumCalls, stub.albumFetches)
	}
}

func TestTrackLinkingKeepsABiggerLocalEditionMatched(t *testing.T) {
	fixtureDir := t.TempDir()
	s := setupMusicScanner(t)
	// The files are an anniversary edition holding the whole Spotify album
	// plus extras; few of them pair, but every Spotify track does.
	s.spotify = &musicScannerSpotifyStub{artist: testArtist(), album: testAlbum(
		spotifyTrack("t1", "Opening Song", 1, 1, 180250, false),
		spotifyTrack("t2", "Vicenta", 1, 2, 180250, false),
		spotifyTrack("t3", "La Pluma", 1, 3, 180250, false),
	)}
	scan := newMusicScanContext(nil)
	for i, title := range []string{"Opening Song", "Vicenta", "La Pluma", "Mandinga", "Candela", "Orgullecida", "Chan Chan"} {
		scanTaggedTrack(t, s, scan, fmt.Sprintf("%s/%02d.m4a", fixtureDir, i+1), 1, albumTags(title, strconv.Itoa(i+1)))
	}
	linkScannedTracks(t, s, scan)

	row := albumMatchFor(t, s, "Test Album")
	if row.spotifyID.String != "album" || row.status.String != musicStatusMatched {
		t.Fatalf("album = %+v, want the match kept", row)
	}
	assertLinked(t, trackLinkFor(t, s, fixtureDir+"/03.m4a"), "t3", false, 3)
}

func TestTrackLinkingDropsASingleMatchedForTheAlbum(t *testing.T) {
	fixtureDir := t.TempDir()
	s := setupMusicScanner(t)
	// The album's lead single: its one track pairs, but a one-track release
	// is not an edition of a four-track album.
	single := testAlbum(spotifyTrack("t1", "Opening Song", 1, 1, 180250, false))
	single.AlbumType = "single"
	s.spotify = &musicScannerSpotifyStub{artist: testArtist(), album: single}
	scan := newMusicScanContext(nil)
	for i, title := range []string{"Opening Song", "Ghosts On The Stereo", "Low", "Backsliding"} {
		scanTaggedTrack(t, s, scan, fmt.Sprintf("%s/%02d.m4a", fixtureDir, i+1), 1, albumTags(title, strconv.Itoa(i+1)))
	}
	linkScannedTracks(t, s, scan)

	row := albumMatchFor(t, s, "Test Album")
	if row.spotifyID.Valid || row.status.String != musicStatusUnmatched || row.reason.String != musicSpotifyReasonTracksDisagree {
		t.Fatalf("album = %+v, want the single's match dropped as tracks_disagree", row)
	}
	link := trackLinkFor(t, s, fixtureDir+"/01.m4a")
	if link.spotifyID.Valid || link.status.Valid {
		t.Fatalf("track = %+v, want no pairing and no outcome", link)
	}
}

func TestTrackLinkingPairsFromListsAtHandAfterTheBreakerTrips(t *testing.T) {
	fixtureDir := t.TempDir()
	s := setupMusicScanner(t)
	stub := &musicScannerSpotifyStub{artist: testArtist(), album: testAlbum(spotifyTrack("t1", "Opening Song", 1, 1, 180250, false))}
	s.spotify = stub
	scan := newMusicScanContext(nil)
	path := fixtureDir + "/01.m4a"
	scanTaggedTrack(t, s, scan, path, 1, albumTags("Opening Song", "1"))

	// The album matched before Spotify started failing, so its track list is
	// already here and pairing needs no request.
	scan.breaker.Stopped = true
	err := s.retrySpotify(context.Background(), scan, newScanReport(Status{}))
	if err != nil {
		t.Fatal(err)
	}
	assertLinked(t, trackLinkFor(t, s, path), "t1", false, 1)
	if stub.albumFetches != 0 {
		t.Fatalf("fetches=%d, want none", stub.albumFetches)
	}
}

func TestTrackLinkRetryPassPairsEarlierTracksAndNewFiles(t *testing.T) {
	dir := t.TempDir()
	s := setupMusicScanner(t)
	first := scannertest.WriteFile(t, filepath.Join(dir, "01.m4a"), "one")
	second := scannertest.WriteFile(t, filepath.Join(dir, "02.m4a"), "two")
	probe := newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{
		first:  testMusicMetadataWithTags(albumTags("Opening Song", "1/3")),
		second: testMusicMetadataWithTags(albumTags("Second Song", "2/3")),
	})
	s.ffprobe = probe
	stub := &musicScannerSpotifyStub{artist: testArtist(), albumErr: entityLookupFailure}
	s.spotify = stub

	// The album fails, so its tracks are imported without a pairing.
	s.scan(dir)
	pending := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_provider_matches WHERE entity_type='track'")
	if pending != 0 {
		t.Fatalf("track outcomes after a failed album = %d, want none", pending)
	}

	// The retry pass matches the album and pairs both tracks from that match.
	stub.albumErr = nil
	stub.album = testAlbum(
		spotifyTrack("t1", "Opening Song", 1, 1, 180250, false),
		spotifyTrack("t2", "Second Song", 1, 2, 180250, true),
		spotifyTrack("t3", "Third Song", 1, 3, 180250, false),
	)
	s.scan(dir)
	assertLinked(t, trackLinkFor(t, s, first), "t1", false, 1)
	assertLinked(t, trackLinkFor(t, s, second), "t2", true, 2)
	status := s.Status()
	if stub.albumCalls != 2 || stub.albumFetches != 0 || status.EnrichmentTotal != 2 || status.EnrichmentProcessed != 2 {
		t.Fatalf("searches=%d fetches=%d status=%+v, want the album retried once and its tracks paired without a fetch", stub.albumCalls, stub.albumFetches, status)
	}

	// A file added to the matched album is paired after one fetch of the list.
	third := scannertest.WriteFile(t, filepath.Join(dir, "03.m4a"), "three")
	probe.results[third] = testMusicMetadataWithTags(albumTags("Third Song", "3/3"))
	stub.albumsByID = map[spotifyapi.ID]*spotifyapi.FullAlbum{"album": stub.album}
	s.scan(dir)
	assertLinked(t, trackLinkFor(t, s, third), "t3", false, 3)
	if stub.albumCalls != 2 || stub.albumFetches != 1 {
		t.Fatalf("searches=%d fetches=%d, want one fetch and no new search", stub.albumCalls, stub.albumFetches)
	}
}

func TestTrackLinkRetryPassSettlesAlbumsGoneFromSpotify(t *testing.T) {
	dir := t.TempDir()
	s := setupMusicScanner(t)
	first := scannertest.WriteFile(t, filepath.Join(dir, "01.m4a"), "one")
	probe := newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{first: testMusicMetadataWithTags(albumTags("Opening Song", "1/2"))})
	s.ffprobe = probe
	stub := &musicScannerSpotifyStub{artist: testArtist(), album: testAlbum(spotifyTrack("t1", "Opening Song", 1, 1, 180250, false))}
	s.spotify = stub
	s.scan(dir)
	assertLinked(t, trackLinkFor(t, s, first), "t1", false, 1)

	// The album has since left Spotify: the fetch is a 404, which settles the
	// new file as unmatched instead of leaving it to be fetched every scan.
	second := scannertest.WriteFile(t, filepath.Join(dir, "02.m4a"), "two")
	probe.results[second] = testMusicMetadataWithTags(albumTags("Second Song", "2/2"))
	s.scan(dir)
	row := trackLinkFor(t, s, second)
	if row.spotifyID.Valid || row.status.String != musicStatusUnmatched || row.reason.String != spotifyapi.MatchReasonNoResults {
		t.Fatalf("new file = %+v, want unmatched/no_results", row)
	}
	s.scan(dir)
	if stub.albumFetches != 1 {
		t.Fatalf("fetches=%d, want the gone album fetched once", stub.albumFetches)
	}
}

func TestSpotifyBreakerStopsLookupsForTheScan(t *testing.T) {
	setup := func(t *testing.T) (*Scanner, string, *musicScannerSpotifyStub) {
		t.Helper()
		dir := t.TempDir()
		s := setupMusicScanner(t)
		results := make(map[string]*ffprobe.FfprobeResult)
		for _, name := range []string{"A", "B", "C"} {
			path := scannertest.WriteFile(t, filepath.Join(dir, name+".m4a"), name)
			results[path] = testMusicMetadataWithTags(ffprobe.FormatTags{Title: "Track " + name, Artist: "Artist " + name, Album: "Album " + name})
		}
		s.ffprobe = newMusicScannerFfprobeByPath(results)
		stub := &musicScannerSpotifyStub{}
		s.spotify = stub
		return s, dir, stub
	}
	assertStopped := func(t *testing.T, s *Scanner, artistCalls, albumCalls, rows int) {
		t.Helper()
		status := s.Status()
		stub := s.spotify.(*musicScannerSpotifyStub)
		if stub.artistCalls != artistCalls || stub.albumCalls != albumCalls {
			t.Fatalf("requests artist=%d album=%d, want %d/%d", stub.artistCalls, stub.albumCalls, artistCalls, albumCalls)
		}
		got := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_provider_matches")
		if got != rows {
			t.Fatalf("outcome rows = %d, want %d (entities after the stop stay pending)", got, rows)
		}
		if status.State != scanner.StateCompletedWithIssues || status.IssueCount != 1 || status.Issues[0].Reason != reasonSpotifyStopped {
			t.Fatalf("status = %+v, want one scan-wide Spotify issue", status)
		}
	}

	t.Run("after a run of transient failures", func(t *testing.T) {
		s, dir, stub := setup(t)
		stub.artistErr = errors.New("connection reset")
		stub.albumErr = errors.New("connection reset")
		s.scan(dir)
		// Artist A, album A and artist B fail; album B and every lookup after
		// it are skipped.
		assertStopped(t, s, 2, 1, 3)

		// The next scan starts closed-circuit and finishes what was pending.
		stub.artistErr = nil
		stub.albumErr = nil
		stub.artistsByName = make(map[string]*spotifyapi.FullArtist)
		stub.albumsByTitle = make(map[string]*spotifyapi.FullAlbum)
		for _, name := range []string{"A", "B", "C"} {
			artist := &spotifyapi.FullArtist{SimpleArtist: spotifyapi.SimpleArtist{ID: spotifyapi.ID("artist-" + name), Name: "Artist " + name}}
			stub.artistsByName["artist "+strings.ToLower(name)] = artist
			album := testAlbum()
			album.ID = spotifyapi.ID("album-" + name)
			album.Name = "Album " + name
			album.Artists = []spotifyapi.SimpleArtist{artist.SimpleArtist}
			stub.albumsByTitle["album "+strings.ToLower(name)] = album
		}
		s.scan(dir)
		matched := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_provider_matches WHERE status='matched' AND entity_type IN ('musician','album')")
		if matched != 6 || s.Status().IssueCount != 0 {
			t.Fatalf("matched entities after recovery = %d, issues = %d", matched, s.Status().IssueCount)
		}
	})

	t.Run("at once after rejected credentials", func(t *testing.T) {
		s, dir, stub := setup(t)
		stub.artistErr = &spotifyapi.TokenError{StatusCode: http.StatusBadRequest, Reason: "invalid_client"}
		s.scan(dir)
		assertStopped(t, s, 1, 0, 1)
	})

	t.Run("not by failures about one entity", func(t *testing.T) {
		s, dir, stub := setup(t)
		stub.artistErr = entityLookupFailure
		stub.albumErr = entityLookupFailure
		s.scan(dir)
		if stub.artistCalls != 3 || stub.albumCalls != 3 || s.Status().IssueCount != 0 {
			t.Fatalf("requests artist=%d album=%d issues=%d, want every entity attempted", stub.artistCalls, stub.albumCalls, s.Status().IssueCount)
		}
	})
}

func TestAlbumQueryCarriesKnownArtistIDAndTagEvidence(t *testing.T) {
	fixtureDir := t.TempDir()
	s := setupMusicScanner(t)
	stub := &musicScannerSpotifyStub{artist: testArtist(), album: testAlbum()}
	s.spotify = stub
	tags := albumTags("Opening Song", "3/12")
	tags.Date = "2001-05-05"
	scanTaggedTrack(t, s, newMusicScanContext(nil), fixtureDir+"/03.m4a", 1, tags)

	want := spotifyapi.AlbumQuery{Title: "Test Album", Artist: "Test Artist", ArtistCredits: []string{"Test Artist"}, ArtistID: "artist", Year: 2001, TotalTracks: 12}
	if len(stub.albumQueries) != 1 || !reflect.DeepEqual(stub.albumQueries[0], want) {
		t.Fatalf("album queries = %+v, want %+v", stub.albumQueries, want)
	}
}

func TestAlbumQueryCarriesCompoundArtistCredits(t *testing.T) {
	fixtureDir := t.TempDir()
	s := setupMusicScanner(t)
	stub := &musicScannerSpotifyStub{artist: testArtist(), album: testAlbum()}
	s.spotify = stub
	tags := albumTags("Opening Song", "1")
	tags.AlbumArtist = "Lin-Manuel Miranda, Leslie Odom, Jr. & Phillipa Soo"
	scanTaggedTrack(t, s, newMusicScanContext(nil), fixtureDir+"/01.m4a", 1, tags)

	want := []string{"Lin-Manuel Miranda", "Leslie Odom, Jr.", "Phillipa Soo"}
	if len(stub.albumQueries) != 1 || !slices.Equal(stub.albumQueries[0].ArtistCredits, want) {
		t.Fatalf("album queries = %+v, want credits %q", stub.albumQueries, want)
	}
}

func beatlesStub() *musicScannerSpotifyStub {
	return &musicScannerSpotifyStub{
		artistErr: &spotifyapi.MatchError{Info: spotifyapi.MatchDebugInfo{Reason: spotifyapi.MatchReasonNoResults}},
		album:     &spotifyapi.FullAlbum{SimpleAlbum: spotifyapi.SimpleAlbum{ID: "abbey", Name: "Abbey Road", Artists: []spotifyapi.SimpleArtist{{ID: "beatles-1", Name: "The Beatles"}}}},
		artistsByID: map[spotifyapi.ID]*spotifyapi.FullArtist{"beatles-1": {
			SimpleArtist: spotifyapi.SimpleArtist{ID: "beatles-1", Name: "The Beatles"},
			Images:       []spotifyapi.Image{{URL: "https://i.scdn.co/beatles.jpg"}},
		}},
	}
}

func musicianMatchFor(t *testing.T, s *Scanner, name string) (spotifyID, thumb, status sql.NullString) {
	t.Helper()
	err := s.tx.DB.QueryRow(`
		SELECT m.spotify_id, m.thumb, o.status FROM musicians m
		LEFT JOIN music_provider_matches o ON o.entity_type = 'musician' AND o.entity_id = m.id
		WHERE m.name = ?`, name).Scan(&spotifyID, &thumb, &status)
	if err != nil {
		t.Fatalf("musician %s: %v", name, err)
	}
	return spotifyID, thumb, status
}

// An artist the name search cannot find is adopted from the album that
// credits it, when the credited name agrees with the local one.
func TestAlbumArtistAdoptedFromMatchedAlbum(t *testing.T) {
	t.Run("during import", func(t *testing.T) {
		fixtureDir := t.TempDir()
		s := setupMusicScanner(t)
		stub := beatlesStub()
		s.spotify = stub
		scan := newMusicScanContext(nil)
		tags := ffprobe.FormatTags{Title: "Come Together", Artist: "Beatles", Album: "Abbey Road", Track: "1"}
		scanTaggedTrack(t, s, scan, fixtureDir+"/01.m4a", 1, tags)

		spotifyID, thumb, status := musicianMatchFor(t, s, "Beatles")
		if spotifyID.String != "beatles-1" || thumb.String != "https://i.scdn.co/beatles.jpg" || status.String != musicStatusMatched {
			t.Fatalf("musician = %#v %#v %#v, want the credited artist adopted", spotifyID, thumb, status)
		}
		if scan.enrichmentCounts[musicStatusMatched] != 2 || scan.enrichmentCounts[musicStatusUnmatched] != 0 {
			t.Fatalf("tallies = %v, want the adoption counted as a match", scan.enrichmentCounts)
		}

		// A second track reuses the adoption instead of fetching again.
		tags.Title = "Something"
		tags.Track = "2"
		scanTaggedTrack(t, s, scan, fixtureDir+"/02.m4a", 1, tags)
		if stub.artistCalls != 1 || stub.artistFetches != 1 {
			t.Fatalf("artist searches=%d fetches=%d, want one of each", stub.artistCalls, stub.artistFetches)
		}
	})

	t.Run("for an album artist the track does not credit", func(t *testing.T) {
		fixtureDir := t.TempDir()
		s := setupMusicScanner(t)
		s.spotify = beatlesStub()
		scan := newMusicScanContext(nil)
		// The first track saves the name with its own search's miss.
		scanTaggedTrack(t, s, scan, fixtureDir+"/intro.m4a", 1, ffprobe.FormatTags{Title: "Intro", Artist: "Beatles"})
		// The album's credit adopts it from a track credited to someone else.
		scanTaggedTrack(t, s, scan, fixtureDir+"/01.m4a", 1, ffprobe.FormatTags{Title: "Come Together", Artist: "Billy Preston", AlbumArtist: "Beatles", Album: "Abbey Road", Track: "1"})

		spotifyID, thumb, status := musicianMatchFor(t, s, "Beatles")
		if spotifyID.String != "beatles-1" || thumb.String != "https://i.scdn.co/beatles.jpg" || status.String != musicStatusMatched {
			t.Fatalf("musician = %#v %#v %#v, want the adoption saved with the track", spotifyID, thumb, status)
		}
	})

	t.Run("during the retry pass", func(t *testing.T) {
		dir := t.TempDir()
		s := setupMusicScanner(t)
		path := scannertest.WriteFile(t, filepath.Join(dir, "01.m4a"), "one")
		s.ffprobe = newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{path: testMusicMetadataWithTags(ffprobe.FormatTags{Title: "Come Together", Artist: "Beatles", Album: "Abbey Road", Track: "1"})})
		stub := beatlesStub()
		matchedAlbum := stub.album
		stub.album = nil
		stub.albumErr = entityLookupFailure
		s.spotify = stub
		s.scan(dir)
		_, _, status := musicianMatchFor(t, s, "Beatles")
		if status.String != musicStatusUnmatched {
			t.Fatalf("musician status = %#v, want unmatched before the album matches", status)
		}

		stub.album = matchedAlbum
		stub.albumErr = nil
		s.scan(dir)
		spotifyID, _, status := musicianMatchFor(t, s, "Beatles")
		if spotifyID.String != "beatles-1" || status.String != musicStatusMatched {
			t.Fatalf("musician = %#v %#v, want adopted from the retried album", spotifyID, status)
		}
		if stub.artistCalls != 1 || stub.artistFetches != 1 {
			t.Fatalf("artist searches=%d fetches=%d, want the final miss left alone and one fetch", stub.artistCalls, stub.artistFetches)
		}
	})

	t.Run("not from a compilation", func(t *testing.T) {
		fixtureDir := t.TempDir()
		s := setupMusicScanner(t)
		stub := beatlesStub()
		stub.album = &spotifyapi.FullAlbum{SimpleAlbum: spotifyapi.SimpleAlbum{ID: "now", Name: "Now That's Music", Artists: []spotifyapi.SimpleArtist{{ID: "va", Name: "Various Artists"}}}}
		s.spotify = stub
		scanTaggedTrack(t, s, newMusicScanContext(nil), fixtureDir+"/01.m4a", 1, ffprobe.FormatTags{Title: "Hit", Artist: "Someone", Album: "Now That's Music", AlbumArtist: "Various Artists"})

		spotifyID, _, status := musicianMatchFor(t, s, "Someone")
		if spotifyID.Valid || status.String != musicStatusUnmatched || stub.artistFetches != 0 {
			t.Fatalf("musician = %#v %#v fetches=%d, want no adoption", spotifyID, status, stub.artistFetches)
		}
	})
}

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
	adopted, err := s.adoptAlbumArtist(context.Background(), scan, "Test Artist", album.spotifyAlbum, false)
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
	if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM musicians WHERE name='Test Artist' AND spotify_id='artist'") != 1 || scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_spotify_adoptions") != 0 {
		t.Fatal("the recovery scan did not save the adopted artist or clear its pending adoption")
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

// The retry pass does not request an artist whose failed lookup is waiting
// out its backoff, even when an album it retries credits that artist; it
// keeps the credit as the pending target, so the adoption resumes by id once
// the wait has passed. The walk requests regardless.
func TestRetryAdoptionHonorsTheArtistBackoff(t *testing.T) {
	s := setupMusicScanner(t)
	dir := t.TempDir()
	scannertest.WriteFile(t, filepath.Join(dir, "track.m4a"), "audio")
	s.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadata()}
	s.scan(dir)
	markArtistsUnmatched(t, s)
	_, err := s.tx.DB.Exec(`UPDATE music_provider_matches SET status='failed', attempts=2, failed_at=0, retry_after=unixepoch()+3600
		WHERE entity_type='musician'`)
	if err != nil {
		t.Fatal(err)
	}
	scan := newMusicScanContext(nil)
	s.spotify = scriptedSpotify(t, spotifyStep{operation: "match", target: "Test Album", album: testAlbum()})
	album, err := s.resolveAlbum(context.Background(), scan, albumQuery{title: "Test Album", artist: "Test Artist"})
	if err != nil {
		t.Fatal(err)
	}

	// The scripted client fails the test on any request it was not given.
	adopted, err := s.adoptAlbumArtist(context.Background(), scan, "Test Artist", album.spotifyAlbum, true)
	if err != nil {
		t.Fatal(err)
	}
	// testAlbum credits the artist as "artist".
	if adopted == nil || adopted.spotifyMatch != nil || adopted.adoptionTarget != "artist" {
		t.Fatalf("a held-back adoption must keep its target and record no outcome: %+v", adopted)
	}
	err = s.persistEnrichment(context.Background(), scan, func(q *database.Queries, txScan *musicScanContext) error {
		_, err := s.persistMusician(context.Background(), q, txScan, *adopted)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_spotify_adoptions WHERE spotify_id = 'artist'") != 1 {
		t.Fatal("the held-back adoption's target was not kept")
	}
	if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_provider_matches WHERE entity_type='musician' AND status='failed' AND retry_after > unixepoch()") != 1 {
		t.Fatal("the held-back artist's failure and its wait were not kept")
	}

	s.spotify = scriptedSpotify(t, spotifyStep{operation: "artist", target: "artist", artist: testArtist()})
	walkScan := newMusicScanContext(nil)
	adopted, err = s.adoptAlbumArtist(context.Background(), walkScan, "Test Artist", album.spotifyAlbum, false)
	if err != nil {
		t.Fatal(err)
	}
	if adopted == nil || adopted.spotifyArtist == nil {
		t.Fatalf("the walk did not request and adopt the album's artist: %+v", adopted)
	}
}

// Another alias's failed request for the same target, cached for the scan,
// gives a held-back artist no outcome: its failure count and wait stay as
// they were.
func TestRetryAdoptionIgnoresACachedFailureForAHeldBackArtist(t *testing.T) {
	s := setupMusicScanner(t)
	dir := t.TempDir()
	scannertest.WriteFile(t, filepath.Join(dir, "track.m4a"), "audio")
	s.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadata()}
	s.scan(dir)
	markArtistsUnmatched(t, s)
	_, err := s.tx.DB.Exec(`UPDATE music_provider_matches SET status='failed', attempts=2, failed_at=0, retry_after=unixepoch()+3600
		WHERE entity_type='musician'`)
	if err != nil {
		t.Fatal(err)
	}
	scan := newMusicScanContext(nil)
	s.spotify = scriptedSpotify(t, spotifyStep{operation: "match", target: "Test Album", album: testAlbum()})
	album, err := s.resolveAlbum(context.Background(), scan, albumQuery{title: "Test Album", artist: "Test Artist"})
	if err != nil {
		t.Fatal(err)
	}
	var retryAfter int64
	err = s.tx.DB.QueryRow("SELECT retry_after FROM music_provider_matches WHERE entity_type='musician'").Scan(&retryAfter)
	if err != nil {
		t.Fatal(err)
	}
	scan.artistIDAttempts["artist"] = artistIDResult{err: errors.New("transport")}

	adopted, err := s.adoptAlbumArtist(context.Background(), scan, "Test Artist", album.spotifyAlbum, true)
	if err != nil {
		t.Fatal(err)
	}
	if adopted == nil || adopted.spotifyMatch != nil || adopted.adoptionTarget != "artist" {
		t.Fatalf("a held-back adoption must keep its target and take no cached failure: %+v", adopted)
	}
	err = s.persistEnrichment(context.Background(), scan, func(q *database.Queries, txScan *musicScanContext) error {
		_, err := s.persistMusician(context.Background(), q, txScan, *adopted)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_provider_matches WHERE entity_type='musician' AND status='failed' AND attempts=2 AND failed_at=0 AND retry_after=?", retryAfter) != 1 {
		t.Fatal("the cached failure counted against the held-back artist")
	}
}

func TestSpotifyRetriesUnchangedCatalog(t *testing.T) {
	for _, offline := range []bool{false, true} {
		t.Run(fmt.Sprintf("offline_%v", offline), func(t *testing.T) {
			s := setupMusicScanner(t)
			dir := t.TempDir()
			scannertest.WriteFile(t, filepath.Join(dir, "track.m4a"), "audio")
			probe := &scannertest.CountingProbe{Default: testMusicMetadata()}
			s.ffprobe = probe
			stub := &musicScannerSpotifyStub{artistErr: errors.New("temporary"), albumErr: errors.New("temporary")}
			if !offline {
				s.spotify = stub
			}
			s.scan(dir)
			if probe.Calls() != 1 {
				t.Fatalf("probes=%d", probe.Calls())
			}
			if !offline && (stub.artistCalls != 1 || stub.albumCalls != 1) {
				t.Fatalf("repeated initial attempts: %+v", stub)
			}
			s.spotify = stub
			stub.artistErr = nil
			stub.albumErr = nil
			stub.artist = &spotifyapi.FullArtist{SimpleArtist: spotifyapi.SimpleArtist{ID: "artist"}}
			stub.album = &spotifyapi.FullAlbum{SimpleAlbum: spotifyapi.SimpleAlbum{ID: "album", ReleaseDate: "2001", ReleaseDatePrecision: "year"}}
			beforeArtist, beforeAlbum := stub.artistCalls, stub.albumCalls
			s.scan(dir)
			if probe.Calls() != 1 || stub.artistCalls != beforeArtist+1 || stub.albumCalls != beforeAlbum+1 {
				t.Fatalf("recovery probes=%d artist=%d album=%d", probe.Calls(), stub.artistCalls, stub.albumCalls)
			}
			matchedRows := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_provider_matches WHERE status='matched'")
			if matchedRows != 2 {
				t.Fatal("recovery did not persist matches")
			}
			s.scan(dir)
			if probe.Calls() != 1 || stub.artistCalls != beforeArtist+1 || stub.albumCalls != beforeAlbum+1 {
				t.Fatal("final outcomes were retried")
			}
		})
	}
}

func TestSpotifyRetrySplitsPersistedCompoundCredits(t *testing.T) {
	fixtureDir := t.TempDir()
	s := setupMusicScanner(t)
	scanTaggedTrack(t, s, newMusicScanContext(nil), fixtureDir+"/one", 1, ffprobe.FormatTags{Title: "Track", Artist: "One & Two", Album: "Album", Genre: "Rock", SortArtist: "First & Second"})
	probe := s.ffprobe.(*scannertest.CountingProbe)
	s.spotify = &musicScannerSpotifyStub{artistErr: &spotifyapi.MatchError{Info: spotifyapi.MatchDebugInfo{Reason: spotifyapi.MatchReasonNoResults}}, albumErr: errors.New("temporary")}
	err := s.retrySpotify(context.Background(), newMusicScanContext(nil), newScanReport(Status{}))
	if err != nil {
		t.Fatal(err)
	}
	if probe.Calls() != 1 {
		t.Fatal("retry probed a file")
	}
	musicianAlbums := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM musician_albums")
	trackMusicians := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM track_musicians")
	if trackMusicians != 2 || musicianAlbums != 2 {
		t.Fatal("compound credits or album links not split")
	}
	sortedCredits := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM musicians WHERE (name='One' AND sort_name='First') OR (name='Two' AND sort_name='Second')")
	if sortedCredits != 2 {
		t.Fatal("per-credit sort tags lost")
	}
	compoundGenres := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM musician_genres WHERE musician_id=(SELECT id FROM musicians WHERE name='One & Two')")
	if compoundGenres != 0 {
		t.Fatal("old compound genre relationship remains")
	}
}

func TestSpotifyRetryPagesAndCancellation(t *testing.T) {
	fixtureDir := t.TempDir()
	s := setupMusicScanner(t)
	ctx := context.Background()
	scan := newMusicScanContext(nil)
	for i := 0; i < 105; i++ {
		scanTaggedTrack(t, s, scan, filepath.Join(fixtureDir, "page", strconv.Itoa(i)), 1, ffprobe.FormatTags{Title: "Track", Artist: fmt.Sprintf("Artist %d", i), Album: fmt.Sprintf("Album %d", i)})
	}
	// Orphans are deliberately retained, but are not retry candidates.
	_, err := s.queries.UpsertMusician(ctx, database.UpsertMusicianParams{Name: "Orphan", SortName: "Orphan"})
	if err != nil {
		t.Fatal(err)
	}
	candidates, err := s.queries.MusicArtistRetryCandidates(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 100 {
		t.Fatalf("page size=%d", len(candidates))
	}
	probe := s.ffprobe.(*scannertest.CountingProbe)
	// A failure about one entity, like a 404, leaves the entity failed without
	// counting against the breaker, so every page is visited.
	stub := &musicScannerSpotifyStub{artistErr: entityLookupFailure, albumErr: entityLookupFailure}
	s.spotify = stub
	retryScan := newMusicScanContext(nil)
	err = s.retrySpotify(ctx, retryScan, newScanReport(Status{}))
	if err != nil {
		t.Fatal(err)
	}
	if stub.artistCalls != 105 || stub.albumCalls != 105 || probe.Calls() != 1 {
		t.Fatalf("paged attempts artist=%d album=%d probes=%d", stub.artistCalls, stub.albumCalls, probe.Calls())
	}
	// Reusing the same scan context cannot issue another request for these entities.
	err = s.retrySpotify(ctx, retryScan, newScanReport(Status{}))
	if err != nil {
		t.Fatal(err)
	}
	if stub.artistCalls != 105 || stub.albumCalls != 105 {
		t.Fatal("repeated same-scan request")
	}
	canceled, cancel := context.WithCancel(ctx)
	defer cancel()
	s.spotify = &cancelingMusicSpotify{cancel: cancel, phase: "artist"}
	canceledScan := newMusicScanContext(nil)
	err = s.retrySpotify(canceled, canceledScan, newScanReport(Status{}))
	canceledError := errors.Is(err, context.Canceled)
	if !canceledError {
		t.Fatalf("canceled retry=%v", err)
	}
	if len(canceledScan.spotifyArtistMisses) != 0 || len(canceledScan.enrichmentCounts) != 0 {
		t.Fatal("cancellation recorded as failure")
	}
}

// A lookup that keeps failing is retried on the next scan once, then held back
// on the miss backoff instead of costing a request on every scan; a success
// clears the count.
func TestSpotifyRetryBacksOffRepeatedFailures(t *testing.T) {
	s := setupMusicScanner(t)
	dir := t.TempDir()
	scannertest.WriteFile(t, filepath.Join(dir, "track.m4a"), "audio")
	s.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadata()}
	stub := &musicScannerSpotifyStub{artistErr: errors.New("temporary"), albumErr: errors.New("temporary")}
	s.spotify = stub

	attempts := func() (int64, int64) {
		t.Helper()
		var withWait, total int64
		err := s.tx.DB.QueryRow(`SELECT count(retry_after), coalesce(sum(attempts), 0) FROM music_provider_matches
			WHERE entity_type IN ('musician','album') AND status='failed'`).Scan(&withWait, &total)
		if err != nil {
			t.Fatal(err)
		}
		return withWait, total
	}

	s.scan(dir)
	if withWait, total := attempts(); withWait != 0 || total != 2 {
		t.Fatalf("after the first failure: %d held back, %d attempts; want 0 and 2", withWait, total)
	}

	// A scan is named by the millisecond it started in; one started in the
	// same millisecond would count as the first scan again.
	time.Sleep(2 * time.Millisecond)
	s.scan(dir)
	if stub.artistCalls != 2 || stub.albumCalls != 2 {
		t.Fatalf("the first failure was not retried on the next scan: %+v", stub)
	}
	if withWait, total := attempts(); withWait != 2 || total != 4 {
		t.Fatalf("after the second failure: %d held back, %d attempts; want 2 and 4", withWait, total)
	}

	s.scan(dir)
	if stub.artistCalls != 2 || stub.albumCalls != 2 {
		t.Fatalf("a held-back failure was looked up again: %+v", stub)
	}

	_, err := s.tx.DB.Exec(`UPDATE music_provider_matches SET retry_after = unixepoch() - 1`)
	if err != nil {
		t.Fatal(err)
	}
	stub.artistErr = nil
	stub.albumErr = nil
	stub.artist = &spotifyapi.FullArtist{SimpleArtist: spotifyapi.SimpleArtist{ID: "artist"}}
	stub.album = &spotifyapi.FullAlbum{SimpleAlbum: spotifyapi.SimpleAlbum{ID: "album", ReleaseDate: "2001", ReleaseDatePrecision: "year"}}
	s.scan(dir)
	if stub.artistCalls != 3 || stub.albumCalls != 3 {
		t.Fatalf("an elapsed backoff was not retried: %+v", stub)
	}
	cleared := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_provider_matches WHERE status='matched' AND attempts=0 AND retry_after IS NULL")
	if cleared != 2 {
		t.Fatalf("matched rows with a cleared count = %d, want 2", cleared)
	}
}

// A pending adoption retries on the next scan, but a failed one waits out its
// backoff like any other failed artist lookup.
func TestPendingAdoptionHonorsTheFailureBackoff(t *testing.T) {
	s := setupMusicScanner(t)
	ctx := context.Background()
	scan := newMusicScanContext(nil)
	scanTaggedTrack(t, s, scan, t.TempDir()+"/adopt", 1, ffprobe.FormatTags{Title: "Track", Artist: "Adoptee", Album: "Record"})
	var musicianID int64
	err := s.tx.DB.QueryRow(`SELECT id FROM musicians WHERE name='Adoptee'`).Scan(&musicianID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.tx.DB.Exec(`INSERT INTO music_spotify_adoptions (musician_id, spotify_id) VALUES (?, 'target')`, musicianID)
	if err != nil {
		t.Fatal(err)
	}
	isCandidate := func() bool {
		t.Helper()
		candidates, err := s.queries.MusicArtistRetryCandidates(ctx, 0)
		if err != nil {
			t.Fatal(err)
		}
		return slices.ContainsFunc(candidates, func(row database.MusicArtistRetryCandidatesRow) bool { return row.ID == musicianID })
	}

	if !isCandidate() {
		t.Fatal("a pending adoption is not retried")
	}
	_, err = s.tx.DB.Exec(`INSERT OR REPLACE INTO music_provider_matches (entity_type, entity_id, provider, status, attempts, failed_at, retry_after)
		VALUES ('musician', ?, 'spotify', 'failed', 2, unixepoch(), unixepoch() + 3600)`, musicianID)
	if err != nil {
		t.Fatal(err)
	}
	if isCandidate() {
		t.Fatal("a failed adoption was retried inside its backoff")
	}
	_, err = s.tx.DB.Exec(`UPDATE music_provider_matches SET retry_after = unixepoch() - 1 WHERE entity_id = ?`, musicianID)
	if err != nil {
		t.Fatal(err)
	}
	if !isCandidate() {
		t.Fatal("a failed adoption is not retried once its backoff has elapsed")
	}
}
