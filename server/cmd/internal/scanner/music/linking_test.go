package music

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/scannertest"
	spotifyapi "igloo/cmd/internal/spotify"
)

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
		FROM tracks t LEFT JOIN music_spotify_matches m ON m.entity_type = 'track' AND m.entity_id = t.id
		WHERE t.file_path = ?`, path).Scan(&row.spotifyID, &row.explicit, &row.trackIndex, &row.status, &row.reason, &row.candidate, &row.score)
	if err != nil {
		t.Fatalf("track link for %s: %v", path, err)
	}
	return row
}

func assertLinked(t *testing.T, row trackLinkRow, spotifyID string, explicit bool, trackIndex int64) {
	t.Helper()
	if row.spotifyID.String != spotifyID || row.explicit != explicit || row.trackIndex != trackIndex || row.status.String != musicSpotifyStatusMatched {
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
	scanTaggedTrack(t, s, newMusicScanContext(nil), extra, 1, albumTags("Hidden Bonus Jam", "9"))
	row := trackLinkFor(t, s, extra)
	if row.spotifyID.Valid || row.status.String != musicSpotifyStatusUnmatched || row.reason.String != spotifyapi.MatchReasonScoreBelowThreshold {
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
	pending := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_spotify_matches WHERE entity_type='track'")
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
	if row.spotifyID.Valid || row.status.String != musicSpotifyStatusUnmatched || row.reason.String != spotifyapi.MatchReasonNoResults {
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
		got := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_spotify_matches")
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
		matched := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_spotify_matches WHERE status='matched' AND entity_type IN ('musician','album')")
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

	want := spotifyapi.AlbumQuery{Title: "Test Album", Artist: "Test Artist", ArtistID: "artist", Year: 2001, TotalTracks: 12}
	if len(stub.albumQueries) != 1 || stub.albumQueries[0] != want {
		t.Fatalf("album queries = %+v, want %+v", stub.albumQueries, want)
	}
}

func TestUnknownAlbumTypeIsStoredAsUnknown(t *testing.T) {
	fixtureDir := t.TempDir()
	s := setupMusicScanner(t)
	album := testAlbum()
	album.AlbumType = "ep"
	s.spotify = &musicScannerSpotifyStub{artist: testArtist(), album: album}
	scanTaggedTrack(t, s, newMusicScanContext(nil), fixtureDir+"/01.m4a", 1, albumTags("Opening Song", "1"))

	var albumType sql.NullString
	err := s.tx.DB.QueryRow("SELECT album_type FROM albums WHERE spotify_id = 'album'").Scan(&albumType)
	if err != nil {
		t.Fatal(err)
	}
	if albumType.Valid {
		t.Fatalf("album_type = %#v, want NULL for a type the schema does not know", albumType)
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
		LEFT JOIN music_spotify_matches o ON o.entity_type = 'musician' AND o.entity_id = m.id
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
		if spotifyID.String != "beatles-1" || thumb.String != "https://i.scdn.co/beatles.jpg" || status.String != musicSpotifyStatusMatched {
			t.Fatalf("musician = %#v %#v %#v, want the credited artist adopted", spotifyID, thumb, status)
		}
		if scan.enrichmentCounts[musicSpotifyStatusMatched] != 2 || scan.enrichmentCounts[musicSpotifyStatusUnmatched] != 0 {
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
		if status.String != musicSpotifyStatusUnmatched {
			t.Fatalf("musician status = %#v, want unmatched before the album matches", status)
		}

		stub.album = matchedAlbum
		stub.albumErr = nil
		s.scan(dir)
		spotifyID, _, status := musicianMatchFor(t, s, "Beatles")
		if spotifyID.String != "beatles-1" || status.String != musicSpotifyStatusMatched {
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
		if spotifyID.Valid || status.String != musicSpotifyStatusUnmatched || stub.artistFetches != 0 {
			t.Fatalf("musician = %#v %#v fetches=%d, want no adoption", spotifyID, status, stub.artistFetches)
		}
	})
}
