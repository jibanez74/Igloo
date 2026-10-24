package music

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/itunes"
	"igloo/cmd/internal/itunes/itunestest"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/scannertest"
)

// appleStub answers lookups from albums; an id it does not carry is not
// listed. err, when set, fails every lookup, and onLookup runs before each.
type appleStub struct {
	albums   map[int64]itunes.Album
	err      error
	lookups  [][]int64
	onLookup func()
}

func (a *appleStub) LookupAlbums(ctx context.Context, ids []int64) (map[int64]itunes.Album, error) {
	a.lookups = append(a.lookups, slices.Clone(ids))
	if a.onLookup != nil {
		a.onLookup()
	}
	contextErr := ctx.Err()
	if contextErr != nil {
		return nil, contextErr
	}
	if a.err != nil {
		return nil, a.err
	}
	listed := make(map[int64]itunes.Album)
	for _, id := range ids {
		album, found := a.albums[id]
		if found {
			listed[id] = album
		}
	}
	return listed, nil
}

// cryPretty is the stub collection most tests match: its songs are the track
// ids given, explicit when marked true.
func cryPretty(songs map[int64]bool) itunes.Album {
	tracks := make(map[int64]itunes.Track, len(songs))
	for id, explicit := range songs {
		tracks[id] = itunes.Track{Explicit: explicit}
	}
	return itunes.Album{ID: 11, Name: "Cry Pretty", ArtistName: "Carrie Underwood", TrackCount: 13, ArtworkURL100: "https://is1-ssl.mzstatic.com/image/thumb/cry/100x100bb.jpg", Tracks: tracks}
}

func cryPrettyTags(title, track string) ffprobe.FormatTags {
	return ffprobe.FormatTags{Title: title, Artist: "Carrie Underwood", Album: "Cry Pretty", Track: track}
}

// setupAppleScanner returns a scanner with the Apple stub, a Spotify stub
// that matches every artist and would match any album, and a probe the
// tests register their files with.
func setupAppleScanner(t *testing.T, apple *appleStub) (*Scanner, *musicScannerSpotifyStub, *musicScannerFfprobeByPath) {
	t.Helper()
	s := setupMusicScanner(t)
	spotify := &musicScannerSpotifyStub{artist: testArtist(), album: testAlbum()}
	probe := newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{})
	s.spotify, s.apple, s.ffprobe = spotify, apple, probe
	return s, spotify, probe
}

// writePurchase writes an iTunes purchase carrying ids and registers its
// tags with the probe.
func writePurchase(t *testing.T, probe *musicScannerFfprobeByPath, path string, ids itunes.CatalogIDs, tags ffprobe.FormatTags) string {
	t.Helper()
	scannertest.WriteFile(t, path, string(itunestest.M4A(ids, false)))
	probe.results[path] = testMusicMetadataWithTags(tags)
	return path
}

type appleAlbumRow struct {
	appleID     sql.NullInt64
	spotifyID   sql.NullString
	cover       sql.NullString
	albumType   sql.NullString
	totalTracks sql.NullInt64
	provider    sql.NullString
	status      sql.NullString
	reason      sql.NullString
}

func appleAlbumFor(t *testing.T, s *Scanner, title string) appleAlbumRow {
	t.Helper()
	var row appleAlbumRow
	err := s.tx.DB.QueryRow(`
		SELECT a.apple_id, a.spotify_id, a.cover, a.album_type, a.total_tracks, m.provider, m.status, m.reason
		FROM albums a LEFT JOIN music_provider_matches m ON m.entity_type = 'album' AND m.entity_id = a.id
		WHERE a.title = ?`, title).Scan(&row.appleID, &row.spotifyID, &row.cover, &row.albumType, &row.totalTracks, &row.provider, &row.status, &row.reason)
	if err != nil {
		t.Fatalf("album %s: %v", title, err)
	}
	return row
}

type appleTrackRow struct {
	appleID   sql.NullInt64
	spotifyID sql.NullString
	explicit  bool
	provider  sql.NullString
	status    sql.NullString
	reason    sql.NullString
}

func appleTrackFor(t *testing.T, s *Scanner, path string) appleTrackRow {
	t.Helper()
	var row appleTrackRow
	err := s.tx.DB.QueryRow(`
		SELECT t.apple_id, t.spotify_id, t.explicit, m.provider, m.status, m.reason
		FROM tracks t LEFT JOIN music_provider_matches m ON m.entity_type = 'track' AND m.entity_id = t.id
		WHERE t.file_path = ?`, path).Scan(&row.appleID, &row.spotifyID, &row.explicit, &row.provider, &row.status, &row.reason)
	if err != nil {
		t.Fatalf("track %s: %v", path, err)
	}
	return row
}

func assertApplePaired(t *testing.T, row appleTrackRow, appleID int64, explicit bool) {
	t.Helper()
	if row.appleID.Int64 != appleID || row.explicit != explicit || row.provider.String != musicProviderApple || row.status.String != musicStatusMatched {
		t.Fatalf("track = %+v, want Apple track %d explicit=%v", row, appleID, explicit)
	}
}

func assertEnrichmentComplete(t *testing.T, status Status) {
	t.Helper()
	if status.EnrichmentTotal != status.EnrichmentProcessed {
		t.Fatalf("enrichment progress %d of %d", status.EnrichmentProcessed, status.EnrichmentTotal)
	}
}

func TestAppleMatchesAlbumsByCatalogID(t *testing.T) {
	dir := t.TempDir()
	apple := &appleStub{albums: map[int64]itunes.Album{11: cryPretty(map[int64]bool{101: false, 102: true})}}
	s, spotify, probe := setupAppleScanner(t, apple)
	first := writePurchase(t, probe, filepath.Join(dir, "01.m4a"), itunes.CatalogIDs{Track: 101, Album: 11}, cryPrettyTags("Cry Pretty", "1/13"))
	second := writePurchase(t, probe, filepath.Join(dir, "02.m4a"), itunes.CatalogIDs{Track: 102, Album: 11}, cryPrettyTags("Ghosts on the Stereo", "2/13"))

	s.scan(dir)

	album := appleAlbumFor(t, s, "Cry Pretty")
	want := appleAlbumRow{
		appleID:     sql.NullInt64{Int64: 11, Valid: true},
		cover:       sql.NullString{String: "https://is1-ssl.mzstatic.com/image/thumb/cry/640x640bb.jpg", Valid: true},
		albumType:   sql.NullString{String: "album", Valid: true},
		totalTracks: sql.NullInt64{Int64: 13, Valid: true},
		provider:    sql.NullString{String: musicProviderApple, Valid: true},
		status:      sql.NullString{String: musicStatusMatched, Valid: true},
	}
	if album != want {
		t.Fatalf("album = %+v, want %+v", album, want)
	}
	assertApplePaired(t, appleTrackFor(t, s, first), 101, false)
	assertApplePaired(t, appleTrackFor(t, s, second), 102, true)
	if spotify.albumCalls != 0 || spotify.albumFetches != 0 || spotify.artistCalls != 1 {
		t.Fatalf("Spotify album searches=%d fetches=%d artist searches=%d, want only the artist search", spotify.albumCalls, spotify.albumFetches, spotify.artistCalls)
	}
	if !slices.EqualFunc(apple.lookups, [][]int64{{11}}, slices.Equal) {
		t.Fatalf("Apple lookups = %v, want one for collection 11", apple.lookups)
	}
	status := s.Status()
	if status.Enriched != 2 || status.EnrichmentFailed != 0 || status.EnrichmentUnmatched != 0 {
		t.Fatalf("tallies matched=%d failed=%d unmatched=%d, want the artist and the album matched", status.Enriched, status.EnrichmentFailed, status.EnrichmentUnmatched)
	}
	assertEnrichmentComplete(t, status)
}

func TestAppleHandsUnlistedAlbumToSpotify(t *testing.T) {
	dir := t.TempDir()
	apple := &appleStub{}
	s, spotify, probe := setupAppleScanner(t, apple)
	spotify.album = testAlbum(spotifyTrack("t1", "Opening Song", 1, 1, 180250, true))
	path := writePurchase(t, probe, filepath.Join(dir, "01.m4a"), itunes.CatalogIDs{Track: 201, Album: 22}, albumTags("Opening Song", "1"))

	s.scan(dir)

	album := appleAlbumFor(t, s, "Test Album")
	if album.appleID.Valid || album.spotifyID.String != "album" || album.provider.String != musicProviderSpotify || album.status.String != musicStatusMatched {
		t.Fatalf("album = %+v, want the Spotify match", album)
	}
	track := appleTrackFor(t, s, path)
	if track.appleID.Valid || track.spotifyID.String != "t1" || !track.explicit || track.provider.String != musicProviderSpotify {
		t.Fatalf("track = %+v, want the Spotify pairing", track)
	}
	if len(apple.lookups) != 1 || spotify.albumCalls != 1 {
		t.Fatalf("Apple lookups=%v Spotify album searches=%d, want one of each", apple.lookups, spotify.albumCalls)
	}
	status := s.Status()
	if status.Enriched != 2 || status.EnrichmentUnmatched != 0 {
		t.Fatalf("tallies matched=%d unmatched=%d, want the album counted once, as matched", status.Enriched, status.EnrichmentUnmatched)
	}
	assertEnrichmentComplete(t, status)
}

func TestAppleLookupFailureLeavesAlbumForNextScan(t *testing.T) {
	dir := t.TempDir()
	apple := &appleStub{err: errors.New("service unavailable")}
	s, spotify, probe := setupAppleScanner(t, apple)
	path := writePurchase(t, probe, filepath.Join(dir, "01.m4a"), itunes.CatalogIDs{Track: 101, Album: 11}, cryPrettyTags("Cry Pretty", "1/13"))

	s.scan(dir)

	album := appleAlbumFor(t, s, "Cry Pretty")
	if album.appleID.Valid || album.status.Valid {
		t.Fatalf("album = %+v, want it left pending", album)
	}
	if spotify.albumCalls != 0 || s.Status().EnrichmentFailed != 1 {
		t.Fatalf("Spotify album searches=%d failed tally=%d, want no search and one failure", spotify.albumCalls, s.Status().EnrichmentFailed)
	}

	apple.err = nil
	apple.albums = map[int64]itunes.Album{11: cryPretty(map[int64]bool{101: false})}
	s.scan(dir)

	if appleAlbumFor(t, s, "Cry Pretty").appleID.Int64 != 11 {
		t.Fatal("the next scan did not match the album")
	}
	assertApplePaired(t, appleTrackFor(t, s, path), 101, false)
	if probe.audioCallsFor(path) != 1 || spotify.albumCalls != 0 {
		t.Fatalf("probes=%d Spotify album searches=%d, want the stored ids reused", probe.audioCallsFor(path), spotify.albumCalls)
	}
}

func TestAppleBreakerStopsAfterThreeFailedRequests(t *testing.T) {
	s := setupMusicScanner(t)
	// 160 pending albums, each with its own collection: four requests' worth.
	_, err := s.tx.DB.Exec(`
		WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM n WHERE i < 160)
		INSERT INTO albums(title, sort_title) SELECT 'Album ' || i, 'Album ' || i FROM n;
		INSERT INTO tracks(title, sort_title, file_path, file_name, container, mime_type, codec, size, track_index, duration, disc, channels, channel_layout, bit_rate, profile, album_id)
		SELECT 'Track', 'Track', '/music/' || id || '.m4a', id || '.m4a', 'm4a', 'audio/mp4', 'aac', 1, 1, 1, 1, '2', 'stereo', 1, 'LC', id FROM albums;
		INSERT INTO music_track_metadata(track_id, artist_tag, artist_key, artist_sort, album_sort, apple_album_id)
		SELECT id, '', '', '', '', 1000 + id FROM tracks;`)
	if err != nil {
		t.Fatal(err)
	}
	apple := &appleStub{err: errors.New("throttled")}
	s.apple = apple
	scan := newMusicScanContext(nil)
	scan.appleEnabled = true
	report := newScanReport(Status{})
	report.scan = scan

	err = s.enrichFromApple(context.Background(), scan, report)
	if err != nil {
		t.Fatalf("enrich from Apple: %v", err)
	}
	if len(apple.lookups) != 3 || !scan.appleBreaker.Stopped {
		t.Fatalf("lookups=%d stopped=%v, want the breaker to stop after three", len(apple.lookups), scan.appleBreaker.Stopped)
	}
	issues := s.Status().Issues
	if len(issues) != 1 || issues[0].Reason != reasonAppleStopped {
		t.Fatalf("issues = %+v, want one Apple stop", issues)
	}
	if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_provider_matches") != 0 {
		t.Fatal("failed lookups recorded outcomes")
	}
}

func TestAppleLookupsOffLeaveAlbumsToSpotify(t *testing.T) {
	dir := t.TempDir()
	apple := &appleStub{albums: map[int64]itunes.Album{11: cryPretty(map[int64]bool{101: false})}}
	s, spotify, probe := setupAppleScanner(t, apple)
	s.appleLookupsEnabled = func() bool { return false }
	spotify.album = testAlbum(spotifyTrack("t1", "Opening Song", 1, 1, 180250, false))
	writePurchase(t, probe, filepath.Join(dir, "01.m4a"), itunes.CatalogIDs{Track: 101, Album: 11}, albumTags("Opening Song", "1"))

	s.scan(dir)

	if len(apple.lookups) != 0 || spotify.albumCalls != 1 {
		t.Fatalf("Apple lookups=%v Spotify album searches=%d, want Spotify alone", apple.lookups, spotify.albumCalls)
	}
	if appleAlbumFor(t, s, "Test Album").spotifyID.String != "album" {
		t.Fatal("the album did not match on Spotify")
	}
	stored := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_track_metadata WHERE apple_track_id = 101 AND apple_album_id = 11")
	if stored != 1 {
		t.Fatal("the file's catalog ids were not kept for when lookups are turned on")
	}
}

func TestAppleAlbumKeepsFilesWithoutCatalogIDs(t *testing.T) {
	dir := t.TempDir()
	apple := &appleStub{albums: map[int64]itunes.Album{11: cryPretty(map[int64]bool{101: false, 103: true})}}
	s, spotify, probe := setupAppleScanner(t, apple)
	purchased := writePurchase(t, probe, filepath.Join(dir, "01.m4a"), itunes.CatalogIDs{Track: 101, Album: 11}, cryPrettyTags("Cry Pretty", "1/13"))
	// A rip without ids joins the album: it waits for Apple too, and stays
	// unpaired.
	ripped := filepath.Join(dir, "02.m4a")
	scannertest.WriteFile(t, ripped, "ripped audio")
	probe.results[ripped] = testMusicMetadataWithTags(cryPrettyTags("Bonus Demo", "14/13"))

	s.scan(dir)

	assertApplePaired(t, appleTrackFor(t, s, purchased), 101, false)
	rip := appleTrackFor(t, s, ripped)
	if rip.appleID.Valid || rip.provider.String != musicProviderApple || rip.status.String != musicStatusUnmatched || rip.reason.Valid {
		t.Fatalf("rip = %+v, want unmatched on Apple without a reason", rip)
	}

	// A file added later pairs with one more lookup, and still no Spotify
	// album search.
	added := writePurchase(t, probe, filepath.Join(dir, "03.m4a"), itunes.CatalogIDs{Track: 103, Album: 11}, cryPrettyTags("Low", "3/13"))
	s.scan(dir)

	assertApplePaired(t, appleTrackFor(t, s, added), 103, true)
	if len(apple.lookups) != 2 || spotify.albumCalls != 0 {
		t.Fatalf("Apple lookups=%v Spotify album searches=%d, want one lookup per scan and no search", apple.lookups, spotify.albumCalls)
	}
	assertEnrichmentComplete(t, s.Status())
}

func TestAppleTrackNotListedIsUnmatched(t *testing.T) {
	dir := t.TempDir()
	apple := &appleStub{albums: map[int64]itunes.Album{11: cryPretty(map[int64]bool{101: false})}}
	s, _, probe := setupAppleScanner(t, apple)
	writePurchase(t, probe, filepath.Join(dir, "01.m4a"), itunes.CatalogIDs{Track: 101, Album: 11}, cryPrettyTags("Cry Pretty", "1/13"))
	gone := writePurchase(t, probe, filepath.Join(dir, "02.m4a"), itunes.CatalogIDs{Track: 199, Album: 11}, cryPrettyTags("Album Only", "2/13"))

	s.scan(dir)

	track := appleTrackFor(t, s, gone)
	if track.appleID.Valid || track.status.String != musicStatusUnmatched || track.reason.String != musicAppleReasonNotListed {
		t.Fatalf("track = %+v, want unmatched as not listed", track)
	}
}

func TestAppleCollectionHeldByAnotherAlbumMergesThem(t *testing.T) {
	dir := t.TempDir()
	apple := &appleStub{albums: map[int64]itunes.Album{11: cryPretty(map[int64]bool{101: false, 102: false})}}
	s, _, probe := setupAppleScanner(t, apple)
	writePurchase(t, probe, filepath.Join(dir, "01.m4a"), itunes.CatalogIDs{Track: 101, Album: 11}, cryPrettyTags("Cry Pretty", "1/13"))
	other := cryPrettyTags("Ghosts on the Stereo", "2/13")
	other.AlbumArtist = "Carrie Underwood & Friends"
	writePurchase(t, probe, filepath.Join(dir, "02.m4a"), itunes.CatalogIDs{Track: 102, Album: 11}, other)

	s.scan(dir)

	albums := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM albums")
	aliases := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_album_identity i JOIN albums a ON a.id = i.album_id WHERE a.apple_id = 11")
	tracks := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM tracks t JOIN albums a ON a.id = t.album_id WHERE a.apple_id = 11 AND t.apple_id IS NOT NULL")
	if albums != 1 || aliases != 2 || tracks != 2 {
		t.Fatalf("albums=%d aliases=%d paired tracks=%d, want one album holding both", albums, aliases, tracks)
	}
	if s.Status().Enriched != 2 {
		t.Fatalf("matched tally = %d, want the artist and the merged album once", s.Status().Enriched)
	}
}

func TestAppleAlbumPairsAcrossItsCollections(t *testing.T) {
	dir := t.TempDir()
	single := itunes.Album{ID: 12, Name: "Cry Pretty - Single", ArtistName: "Carrie Underwood", TrackCount: 1, Tracks: map[int64]itunes.Track{201: {Explicit: true}}}
	apple := &appleStub{albums: map[int64]itunes.Album{11: cryPretty(map[int64]bool{101: false, 102: false}), 12: single}}
	s, _, probe := setupAppleScanner(t, apple)
	writePurchase(t, probe, filepath.Join(dir, "01.m4a"), itunes.CatalogIDs{Track: 101, Album: 11}, cryPrettyTags("Cry Pretty", "1/13"))
	writePurchase(t, probe, filepath.Join(dir, "02.m4a"), itunes.CatalogIDs{Track: 102, Album: 11}, cryPrettyTags("Ghosts on the Stereo", "2/13"))
	fromSingle := writePurchase(t, probe, filepath.Join(dir, "03.m4a"), itunes.CatalogIDs{Track: 201, Album: 12}, cryPrettyTags("Cry Pretty (Single)", "1/1"))

	s.scan(dir)

	album := appleAlbumFor(t, s, "Cry Pretty")
	if album.appleID.Int64 != 11 || album.albumType.String != "album" || album.totalTracks.Int64 != 13 {
		t.Fatalf("album = %+v, want the collection most files carry", album)
	}
	assertApplePaired(t, appleTrackFor(t, s, fromSingle), 201, true)
	if len(apple.lookups) != 1 || len(apple.lookups[0]) != 2 {
		t.Fatalf("Apple lookups = %v, want both collections in one request", apple.lookups)
	}
}

func TestAppleMergePairsPendingTracksAcrossOwnerCollections(t *testing.T) {
	dir := t.TempDir()
	single := itunes.Album{ID: 12, Tracks: map[int64]itunes.Track{201: {Explicit: true}}}
	apple := &appleStub{albums: map[int64]itunes.Album{11: cryPretty(map[int64]bool{101: false, 102: true}), 12: single}}
	s, _, probe := setupAppleScanner(t, apple)
	s.spotify = nil
	olderTags := cryPrettyTags("Ghosts on the Stereo", "2/13")
	olderTags.Album = "Old album tags"
	older := writePurchase(t, probe, filepath.Join(dir, "01.m4a"), itunes.CatalogIDs{}, olderTags)
	original := writePurchase(t, probe, filepath.Join(dir, "02.m4a"), itunes.CatalogIDs{Track: 101, Album: 11}, cryPrettyTags("Cry Pretty", "1/13"))
	s.scan(dir)

	var olderID, ownerID int64
	err := s.tx.DB.QueryRow("SELECT id FROM albums WHERE title = ?", olderTags.Album).Scan(&olderID)
	if err != nil {
		t.Fatal(err)
	}
	err = s.tx.DB.QueryRow("SELECT id FROM albums WHERE apple_id = 11").Scan(&ownerID)
	if err != nil {
		t.Fatal(err)
	}
	if olderID >= ownerID {
		t.Fatalf("older album %d must precede owner %d", olderID, ownerID)
	}

	// Retagging makes the older album an Apple candidate before the owner,
	// which now also has an unpaired purchase from a different collection.
	writePurchase(t, probe, older, itunes.CatalogIDs{Track: 102, Album: 11}, olderTags)
	added := writePurchase(t, probe, filepath.Join(dir, "03.m4a"), itunes.CatalogIDs{Track: 201, Album: 12}, cryPrettyTags("Cry Pretty (Single)", "1/1"))
	s.scan(dir)

	albums := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM albums")
	owners := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM albums WHERE id = ? AND apple_id = 11", ownerID)
	if albums != 1 || owners != 1 {
		t.Fatal("merge did not retain the existing Apple owner")
	}
	tracks := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM tracks WHERE album_id = ?", ownerID)
	if tracks != 3 {
		t.Fatal("surviving album does not own all three tracks")
	}
	assertApplePaired(t, appleTrackFor(t, s, original), 101, false)
	assertApplePaired(t, appleTrackFor(t, s, older), 102, true)
	assertApplePaired(t, appleTrackFor(t, s, added), 201, true)
	lookupsMatch := slices.EqualFunc(apple.lookups, [][]int64{{11}, {11, 12}}, slices.Equal)
	if !lookupsMatch {
		t.Fatalf("Apple lookups = %v, want each collection once per scan", apple.lookups)
	}

	s.scan(dir)
	assertApplePaired(t, appleTrackFor(t, s, original), 101, false)
	assertApplePaired(t, appleTrackFor(t, s, older), 102, true)
	assertApplePaired(t, appleTrackFor(t, s, added), 201, true)
	status := s.Status()
	if status.Unchanged != 3 || len(apple.lookups) != 2 {
		t.Fatalf("unchanged=%d lookups=%v, want preserved pairings without another lookup", status.Unchanged, apple.lookups)
	}
}

// seedAppleMerge puts pending collection-11 albums before an existing owner
// with a pending collection-12 track. ownerID=101 puts it on the next page.
func seedAppleMerge(t *testing.T, s *Scanner, ownerID int64) {
	t.Helper()
	_, err := s.tx.DB.Exec(`
		WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM n WHERE i < ?)
		INSERT INTO albums(id, title, sort_title) SELECT i, 'Album ' || i, 'Album ' || i FROM n;
		UPDATE albums SET apple_id = 11 WHERE id = ?;
		INSERT INTO music_provider_matches(entity_type, entity_id, provider, status)
		VALUES ('album', ?, 'apple', 'matched');
		INSERT INTO tracks(title, sort_title, file_path, file_name, container, mime_type, codec, size, track_index, duration, disc, channels, channel_layout, bit_rate, profile, album_id)
		SELECT 'Track', 'Track', '/music/' || id || '.m4a', id || '.m4a', 'm4a', 'audio/mp4', 'aac', 1, 1, 1, 1, '2', 'stereo', 1, 'LC', id FROM albums;
		INSERT INTO music_track_metadata(track_id, artist_tag, artist_key, artist_sort, album_sort, apple_track_id, apple_album_id)
		SELECT id, '', '', '', '', CASE WHEN album_id = ? THEN 201 ELSE 101 END,
		CASE WHEN album_id = ? THEN 12 ELSE 11 END FROM tracks;`, ownerID, ownerID, ownerID, ownerID, ownerID)
	if err != nil {
		t.Fatal(err)
	}
}

func TestAppleMergeDistinguishesUnansweredAndUnlistedOwnerCollection(t *testing.T) {
	for _, answered := range []bool{false, true} {
		name := "unanswered"
		if answered {
			name = "confirmed unlisted"
		}
		t.Run(name, func(t *testing.T) {
			s := setupMusicScanner(t)
			seedAppleMerge(t, s, 2)
			collection := cryPretty(map[int64]bool{101: false})
			answers := map[int64]*itunes.Album{11: &collection}
			if answered {
				answers[12] = nil
			}
			scan := newMusicScanContext(nil)
			err := s.settleAppleAlbum(context.Background(), scan, appleAlbum{id: 1, collections: []int64{11}}, answers)
			if err != nil {
				t.Fatal(err)
			}
			albums := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM albums")
			tracks := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM tracks WHERE album_id = 2")
			if albums != 1 || tracks != 2 {
				t.Fatal("valid merge was not committed")
			}
			if !answered {
				outcomes := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_provider_matches WHERE entity_type = 'track'")
				if outcomes != 0 {
					t.Fatal("unanswered owner collection persisted premature track outcomes")
				}
				answers[12] = &itunes.Album{ID: 12, Tracks: map[int64]itunes.Track{201: {Explicit: true}}}
				err = s.settleAppleAlbum(context.Background(), scan, appleAlbum{id: 2, matched: 11, collections: []int64{11, 12}}, answers)
				if err != nil {
					t.Fatal(err)
				}
				assertApplePaired(t, appleTrackFor(t, s, "/music/2.m4a"), 201, true)
			} else {
				track := appleTrackFor(t, s, "/music/2.m4a")
				if track.appleID.Valid || track.provider.String != musicProviderApple || track.status.String != musicStatusUnmatched || track.reason.String != musicAppleReasonNotListed {
					t.Fatalf("track = %+v, want confirmed not_listed", track)
				}
			}
			assertApplePaired(t, appleTrackFor(t, s, "/music/1.m4a"), 101, false)
		})
	}
}

func TestAppleMergeDefersPairingUntilOwnerPageIsAnswered(t *testing.T) {
	for _, failOwnerLookup := range []bool{false, true} {
		name := "later page"
		if failOwnerLookup {
			name = "next scan after failed owner lookup"
		}
		t.Run(name, func(t *testing.T) {
			s := setupMusicScanner(t)
			seedAppleMerge(t, s, 101)
			apple := &appleStub{albums: map[int64]itunes.Album{
				11: cryPretty(map[int64]bool{101: false}),
				12: {ID: 12, Tracks: map[int64]itunes.Track{201: {Explicit: true}}},
			}}
			s.apple = apple
			apple.onLookup = func() {
				if len(apple.lookups) != 2 {
					return
				}
				albums := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM albums")
				tracks := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM tracks WHERE album_id = 101")
				if albums != 1 || tracks != 101 {
					t.Fatal("first page did not commit merges into the owner outside its page")
				}
				outcomes := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_provider_matches WHERE entity_type = 'track'")
				if outcomes != 0 {
					t.Fatal("tracks received outcomes before the owner's collection was answered")
				}
				if failOwnerLookup {
					apple.err = errors.New("service unavailable")
				}
			}
			scan := newMusicScanContext(nil)
			scan.appleEnabled = true
			report := newScanReport(Status{})
			report.scan = scan
			err := s.enrichFromApple(context.Background(), scan, report)
			if err != nil {
				t.Fatal(err)
			}
			lookupsMatch := slices.EqualFunc(apple.lookups, [][]int64{{11}, {12}}, slices.Equal)
			if !lookupsMatch {
				t.Fatalf("Apple lookups = %v, want deduplicated lookups across pages", apple.lookups)
			}
			if failOwnerLookup {
				outcomes := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_provider_matches WHERE entity_type = 'track'")
				if outcomes != 0 {
					t.Fatal("failed owner lookup persisted track outcomes")
				}
				apple.err = nil
				scan = newMusicScanContext(nil)
				scan.appleEnabled = true
				report = newScanReport(Status{})
				report.scan = scan
				err = s.enrichFromApple(context.Background(), scan, report)
				if err != nil {
					t.Fatal(err)
				}
				lookupsMatch = slices.EqualFunc(apple.lookups, [][]int64{{11}, {12}, {11, 12}}, slices.Equal)
				if !lookupsMatch {
					t.Fatalf("Apple lookups = %v, want both collections retried next scan", apple.lookups)
				}
			}
			assertApplePaired(t, appleTrackFor(t, s, "/music/1.m4a"), 101, false)
			assertApplePaired(t, appleTrackFor(t, s, "/music/101.m4a"), 201, true)
			paired := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_provider_matches WHERE entity_type = 'track' AND provider = 'apple' AND status = 'matched'")
			if paired != 101 {
				t.Fatal("not all deferred tracks paired after the owner lookup succeeded")
			}
		})
	}
}

func TestShutdownDuringAppleLookup(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	apple := &appleStub{albums: map[int64]itunes.Album{11: cryPretty(map[int64]bool{101: false})}, onLookup: cancel}
	s, _, probe := setupAppleScanner(t, apple)
	s.scanContext = ctx
	writePurchase(t, probe, filepath.Join(dir, "01.m4a"), itunes.CatalogIDs{Track: 101, Album: 11}, cryPrettyTags("Cry Pretty", "1/13"))

	s.scan(dir)

	if len(apple.lookups) != 1 || s.Status().State != scanner.StateCanceled {
		t.Fatalf("lookups=%d state=%s, want a cancelled run after one lookup", len(apple.lookups), s.Status().State)
	}
	if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_provider_matches WHERE provider = 'apple'") != 0 {
		t.Fatal("a cancelled lookup recorded Apple outcomes")
	}
}
