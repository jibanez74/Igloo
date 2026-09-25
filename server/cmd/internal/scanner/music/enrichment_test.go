package music

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/scannertest"
	spotifyapi "igloo/cmd/internal/spotify"

	spotifylib "github.com/zmb3/spotify/v2"
)

func scanTaggedTrack(t *testing.T, s *Scanner, scan *musicScanContext, path string, size int64, tags ffprobe.FormatTags) {
	t.Helper()
	s.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadataWithTags(tags)}
	n, _, failures := s.processMusicFixtureBatch(t, context.Background(), scan, []scanner.ScanFile{{Path: path, Ext: "m4a", Size: size}})
	if n != 1 || failures != 0 {
		t.Fatalf("scan %s: scanned=%d failures=%d logs=%+v", path, n, failures, s.logger.(*scannertest.Logger).WarnEntries)
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
			stub.artist = &spotifylib.FullArtist{SimpleArtist: spotifylib.SimpleArtist{ID: "artist"}}
			stub.album = &spotifylib.FullAlbum{SimpleAlbum: spotifylib.SimpleAlbum{ID: "album", ReleaseDate: "2001", ReleaseDatePrecision: "year"}}
			beforeArtist, beforeAlbum := stub.artistCalls, stub.albumCalls
			s.scan(dir)
			if probe.Calls() != 1 || stub.artistCalls != beforeArtist+1 || stub.albumCalls != beforeAlbum+1 {
				t.Fatalf("recovery probes=%d artist=%d album=%d", probe.Calls(), stub.artistCalls, stub.albumCalls)
			}
			matchedRows := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_spotify_matches WHERE status='matched'")
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

func TestMusicNormalizedIdentityAndMutableMetadata(t *testing.T) {
	fixtureDir := t.TempDir()
	for _, reverse := range []bool{false, true} {
		t.Run(fmt.Sprintf("reverse_%v", reverse), func(t *testing.T) {
			s := setupMusicScanner(t)
			tags := []ffprobe.FormatTags{
				{Title: "One", Artist: " ÉCHO ", Album: " Record ", AlbumArtist: " ÉCHO ", Genre: " RÖCK ", SortArtist: "Z", SortAlbum: "Z", Date: "2020"},
				{Title: "Two", Artist: "écho", Album: "record", AlbumArtist: "écho", Genre: "röck", SortArtist: "A", SortAlbum: "A", Date: "2019"},
				{Title: "Three", Artist: "ÉCHO", Album: "RECORD", AlbumArtist: "ÉCHO", Genre: "RÖCK", SortArtist: "Z", SortAlbum: "Z", Date: "2020"},
			}
			scan := newMusicScanContext(nil)
			for i := range tags {
				if i == 2 {
					scan = newMusicScanContext(nil)
				}
				j := i
				if reverse {
					j = len(tags) - i - 1
				}
				// Exercise both same-scan caches and persisted Unicode identities.
				scanTaggedTrack(t, s, scan, fmt.Sprintf(fixtureDir+"/%d.m4a", j), 1, tags[j])
			}
			for _, table := range []string{"musicians", "albums", "genres"} {
				tableRows := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM "+table)
				if tableRows != 1 {
					t.Fatalf("duplicate %s", table)
				}
			}
			var sortName, sortTitle, date string
			var year int
			err := s.tx.DB.QueryRow("SELECT m.sort_name,a.sort_title,a.release_date,a.year FROM musicians m CROSS JOIN albums a").Scan(&sortName, &sortTitle, &date, &year)
			if err != nil {
				t.Fatal(err)
			}
			if sortName != "Z" || sortTitle != "Z" || date != "2020-01-01" || year != 2020 {
				t.Fatalf("majority=%s/%s/%s/%d", sortName, sortTitle, date, year)
			}
			tags[2].SortArtist = ""
			tags[2].SortAlbum = ""
			tags[2].Date = ""
			scanTaggedTrack(t, s, newMusicScanContext(nil), fixtureDir+"/2.m4a", 2, tags[2])
			err = s.tx.DB.QueryRow("SELECT m.sort_name,a.sort_title,a.release_date FROM musicians m CROSS JOIN albums a").Scan(&sortName, &sortTitle, &date)
			if err != nil {
				t.Fatal(err)
			}
			if sortName != "A" || sortTitle != "A" || date != "2019-01-01" {
				t.Fatalf("ties=%s/%s/%s", sortName, sortTitle, date)
			}
			for i := 0; i < 2; i++ {
				tags[i].SortArtist = ""
				tags[i].SortAlbum = ""
				tags[i].Date = "invalid"
				scanTaggedTrack(t, s, newMusicScanContext(nil), fmt.Sprintf(fixtureDir+"/%d.m4a", i), 2, tags[i])
			}
			fallbackAlbums := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM albums WHERE sort_title=title AND release_date IS NULL AND year IS NULL")
			fallbackMusicians := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM musicians WHERE sort_name=name")
			if fallbackMusicians != 1 || fallbackAlbums != 1 {
				t.Fatal("removed tags did not restore fallback")
			}
			// Music genres normalise case; movie genres must keep it, since
			// TMDB names are the identity there.
			for _, tag := range []string{"Movie", "movie"} {
				_, err = s.queries.GetOrCreateGenre(context.Background(), database.GetOrCreateGenreParams{Tag: tag, GenreType: "movie"})
				if err != nil {
					t.Fatal(err)
				}
			}
			movieGenres := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM genres WHERE genre_type='movie'")
			if movieGenres != 2 {
				t.Fatal("movie identity changed")
			}
		})
	}
}

func TestMusicRelationshipsAndSpotifyDateFallback(t *testing.T) {
	fixtureDir := t.TempDir()
	s := setupMusicScanner(t)
	s.spotify = &musicScannerSpotifyStub{artist: &spotifylib.FullArtist{SimpleArtist: spotifylib.SimpleArtist{ID: "artist"}, Genres: []string{"Rock"}}, album: &spotifylib.FullAlbum{SimpleAlbum: spotifylib.SimpleAlbum{ID: "album", ReleaseDate: "2000", ReleaseDatePrecision: "year"}, Genres: []string{"Rock"}}}
	scan := newMusicScanContext(nil)
	tags := ffprobe.FormatTags{Title: "Track", Artist: "Artist", Album: "Album", Genre: "Rock", Date: "2022"}
	scanTaggedTrack(t, s, scan, fixtureDir+"/one", 1, tags)
	scanTaggedTrack(t, s, scan, fixtureDir+"/two", 1, tags)
	musicianGenres := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM musician_genres")
	if musicianGenres != 2 {
		t.Fatal("missing provenance overlap")
	}
	genres, err := s.queries.GetGenresByMusicianID(context.Background(), 1)
	if err != nil || len(genres) != 1 {
		t.Fatalf("distinct genres=%v %v", genres, err)
	}
	tags.Artist = ""
	tags.Album = ""
	tags.Genre = ""
	tags.Date = ""
	scanTaggedTrack(t, s, scan, fixtureDir+"/one", 2, tags)
	musicianAlbums := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM musician_albums")
	if musicianAlbums != 1 {
		t.Fatal("removed multiply supported link")
	}
	// Remove the last local date and genre while retaining album membership.
	tags.Artist = "Artist"
	tags.Album = "Album"
	scanTaggedTrack(t, s, scan, fixtureDir+"/two", 2, tags)
	datedAlbums := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM albums WHERE release_date='2000-01-01' AND year=2000")
	if datedAlbums != 1 {
		t.Fatal("missing Spotify date fallback")
	}
	spotifyAlbumGenres := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM album_genres WHERE source='spotify'")
	localMusicianGenres := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM musician_genres WHERE source='local'")
	if localMusicianGenres != 0 || spotifyAlbumGenres != 1 {
		t.Fatal("genre provenance removal")
	}
	tags.Genre = "Rock"
	tags.SortArtist = "Sort"
	scanTaggedTrack(t, s, scan, fixtureDir+"/two", 3, tags)
	restoredLocalGenres := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM musician_genres WHERE source='local'")
	if restoredLocalGenres != 1 {
		t.Fatal("local genre not restored within scan")
	}
	err = s.queries.DeleteAlbum(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	localGenresAfterDelete := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM musician_genres WHERE source='local'")
	albumLinksAfterDelete := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM musician_albums")
	if albumLinksAfterDelete != 0 || localGenresAfterDelete != 0 {
		t.Fatal("album deletion left local relationships")
	}
	fallbackSortMusicians := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM musicians WHERE sort_name=name")
	if fallbackSortMusicians != 1 {
		t.Fatal("album deletion left sort contribution")
	}
	spotifyMusicianGenres := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM musician_genres WHERE source='spotify'")
	if spotifyMusicianGenres != 1 {
		t.Fatal("album deletion removed Spotify genre")
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

func TestSpotifyMergesPreserveTracksAndRollback(t *testing.T) {
	fixtureDir := t.TempDir()
	s := setupMusicScanner(t)
	ctx := context.Background()
	for i := 1; i <= 2; i++ {
		scanTaggedTrack(t, s, newMusicScanContext(nil), fmt.Sprintf(fixtureDir+"/%d", i), 1, ffprobe.FormatTags{Title: "Track", Artist: fmt.Sprintf("Artist %d", i), Album: fmt.Sprintf("Album %d", i), SortArtist: fmt.Sprintf("Sort %d", i), Genre: fmt.Sprintf("Genre %d", i)})
	}
	_, err := s.tx.DB.Exec("INSERT INTO users(id,email,password,name) VALUES(1,'test@example.com','hash','Test')")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.tx.DB.Exec("INSERT INTO playlists(id,name,user_id) VALUES(1,'Music',1)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.tx.DB.Exec("INSERT INTO playlist_tracks(playlist_id,track_id,added_by,position) VALUES(1,2,1,0)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.tx.DB.Exec("INSERT INTO user_play_history(user_id,track_id,duration_played) VALUES(1,2,60)")
	if err != nil {
		t.Fatal(err)
	}

	s.spotify = &musicScannerSpotifyStub{artist: &spotifylib.FullArtist{SimpleArtist: spotifylib.SimpleArtist{ID: "artist"}}, album: &spotifylib.FullAlbum{SimpleAlbum: spotifylib.SimpleAlbum{ID: "album"}}}
	scan := newMusicScanContext(nil)
	// Enrich the first owner before forcing the second merge to roll back.
	artist, err := s.resolveMusician(ctx, scan, "Artist 1", "")
	if err != nil {
		t.Fatal(err)
	}
	err = s.persistEnrichment(ctx, scan, func(q *database.Queries, txScan *musicScanContext) error {
		_, e := s.persistMusician(ctx, q, txScan, *artist)
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.tx.DB.Exec("CREATE TRIGGER reject_merge BEFORE DELETE ON musicians WHEN OLD.id=2 BEGIN SELECT RAISE(ABORT,'merge failure'); END")
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.resolveMusician(ctx, scan, "Artist 2", "")
	if err != nil {
		t.Fatal(err)
	}
	err = s.persistEnrichment(ctx, scan, func(q *database.Queries, txScan *musicScanContext) error {
		_, e := s.persistMusician(ctx, q, txScan, *second)
		return e
	})
	if err == nil {
		t.Fatal("expected merge rollback")
	}
	secondIdentities := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_artist_identity WHERE musician_id=2")
	secondTracks := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM tracks WHERE id=2 AND musician_id=2")
	if secondTracks != 1 || secondIdentities != 1 {
		t.Fatal("merge rollback changed references")
	}
	_, err = s.tx.DB.Exec("DROP TRIGGER reject_merge")
	if err != nil {
		t.Fatal(err)
	}
	err = s.retrySpotify(ctx, scan, newScanReport(Status{}))
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"musicians", "albums"} {
		tableRows := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM "+table)
		if tableRows != 1 {
			t.Fatalf("merge left duplicate %s", table)
		}
	}
	playlistTracks := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM playlist_tracks WHERE track_id=2")
	mergedTracks := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM tracks WHERE id IN (1,2) AND musician_id=1 AND album_id=1")
	if mergedTracks != 2 || playlistTracks != 1 {
		t.Fatal("merge changed track identity or playlist")
	}
	albumAliases := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_album_identity WHERE album_id=1")
	artistAliases := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_artist_identity WHERE musician_id=1")
	if artistAliases != 2 || albumAliases != 2 {
		t.Fatal("aliases not retained")
	}
	// The redundant artist's genres and credits move to the owner and its
	// Spotify match row goes with it, so nothing keeps pointing at the merged
	// row.
	ownerGenres := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM musician_genres WHERE musician_id=1")
	ownerCredits := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM track_musicians WHERE musician_id=1")
	redundantRefs := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM track_musicians WHERE musician_id=2") + scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_spotify_matches WHERE entity_type='musician' AND entity_id=2")
	if ownerGenres != 2 || ownerCredits != 2 || redundantRefs != 0 {
		t.Fatalf("merge moved genres=%d credits=%d, redundant references=%d", ownerGenres, ownerCredits, redundantRefs)
	}
	scanTaggedTrack(t, s, scan, fixtureDir+"/3", 1, ffprobe.FormatTags{Title: "Later", Artist: "Artist 2", Album: "Album 2"})
	laterTracks := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM tracks WHERE musician_id=1 AND album_id=1")
	if laterTracks != 3 {
		t.Fatal("stale cached IDs after merge")
	}
	historyCount := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM user_play_history WHERE track_id=2 AND duration_played=60")
	if historyCount != 1 {
		t.Fatal("merge lost history")
	}

	violations := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM pragma_foreign_key_check")
	if violations != 0 {
		t.Fatalf("foreign keys: %d violations", violations)
	}
}

func TestSpotifyRetryPagesAndCancellation(t *testing.T) {
	fixtureDir := t.TempDir()
	s := setupMusicScanner(t)
	ctx := context.Background()
	scan := newMusicScanContext(nil)
	for i := 0; i < 105; i++ {
		scanTaggedTrack(t, s, scan, fmt.Sprintf(fixtureDir+"/page/%d", i), 1, ffprobe.FormatTags{Title: "Track", Artist: fmt.Sprintf("Artist %d", i), Album: fmt.Sprintf("Album %d", i)})
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
	stub := &musicScannerSpotifyStub{artistErr: errors.New("temporary"), albumErr: errors.New("temporary")}
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

func TestMusicAlbumMoveReconcilesBothSides(t *testing.T) {
	fixtureDir := t.TempDir()
	s := setupMusicScanner(t)
	scan := newMusicScanContext(nil)
	tags := ffprobe.FormatTags{Title: "Track", Artist: "Artist", Album: "First", Genre: "Rock", Date: "2020", SortAlbum: "Sort"}
	scanTaggedTrack(t, s, scan, fixtureDir+"/one", 1, tags)
	tags.Date = "2019"
	tags.SortAlbum = ""
	scanTaggedTrack(t, s, scan, fixtureDir+"/two", 1, tags)
	tags.Album = "Second"
	tags.Artist = "Other"
	tags.Date = "2021"
	tags.Genre = "Jazz"
	scanTaggedTrack(t, s, scan, fixtureDir+"/one", 2, tags)
	first := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM albums WHERE title='First' AND release_date='2019-01-01' AND sort_title='First'")
	second := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM albums WHERE title='Second' AND release_date='2021-01-01'")
	links := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM musician_albums")
	localGenres := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM album_genres WHERE source='local'")
	if first != 1 || second != 1 || links != 2 || localGenres != 2 {
		t.Fatalf("move: first=%d second=%d links=%d genres=%d", first, second, links, localGenres)
	}
	tags.Artist = ""
	tags.Album = ""
	tags.Genre = ""
	scanTaggedTrack(t, s, scan, fixtureDir+"/two", 2, tags)
	links = scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM musician_albums")
	localGenres = scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM album_genres WHERE source='local'")
	if links != 1 || localGenres != 1 {
		t.Fatal("last supporting track left stale relationships")
	}
}

func TestSpotifyAlbumMergeRollback(t *testing.T) {
	fixtureDir := t.TempDir()
	s := setupMusicScanner(t)
	ctx := context.Background()
	scan := newMusicScanContext(nil)
	for i := 1; i <= 2; i++ {
		scanTaggedTrack(t, s, scan, fmt.Sprintf(fixtureDir+"/%d", i), 1, ffprobe.FormatTags{Title: "Track", Album: fmt.Sprintf("Album %d", i), Date: fmt.Sprintf("200%d", i)})
	}
	s.spotify = &musicScannerSpotifyStub{album: &spotifylib.FullAlbum{SimpleAlbum: spotifylib.SimpleAlbum{ID: "album"}}}
	scan = newMusicScanContext(nil)
	owner, err := s.resolveAlbum(ctx, scan, "Album 1", "Album 1", "")
	if err != nil {
		t.Fatal(err)
	}
	err = s.persistEnrichment(ctx, scan, func(q *database.Queries, txScan *musicScanContext) error {
		_, e := s.persistAlbum(ctx, q, txScan, *owner)
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.tx.DB.Exec("CREATE TRIGGER reject_album_merge BEFORE DELETE ON albums WHEN OLD.id=2 BEGIN SELECT RAISE(ABORT,'album merge failure'); END")
	if err != nil {
		t.Fatal(err)
	}
	invalidations := 0
	s.invalidateCommittedTrack = func(int64) { invalidations++ }
	err = s.retrySpotify(ctx, scan, newScanReport(Status{}))
	if err == nil {
		t.Fatal("expected album merge failure")
	}
	oldTrack := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM tracks WHERE id=2 AND album_id=2")
	oldAlias := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM music_album_identity WHERE album_id=2")
	if oldTrack != 1 || oldAlias != 1 || invalidations != 0 {
		t.Fatal("album rollback changed references or cache")
	}
	_, err = s.tx.DB.Exec("DROP TRIGGER reject_album_merge")
	if err != nil {
		t.Fatal(err)
	}
	err = s.retrySpotify(ctx, scan, newScanReport(Status{}))
	if err != nil {
		t.Fatal(err)
	}
	tracks := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM tracks WHERE album_id=1")
	dates := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM albums WHERE id=1 AND release_date='2001-01-01'")
	if tracks != 2 || dates != 1 || invalidations != 1 {
		t.Fatalf("merge tracks=%d dates=%d invalidations=%d", tracks, dates, invalidations)
	}
}

func TestMusicUnchangedDerivedMetadataDoesNotWrite(t *testing.T) {
	fixtureDir := t.TempDir()
	s := setupMusicScanner(t)
	tags := ffprobe.FormatTags{Title: "Track", Artist: "Artist", Album: "Album", SortArtist: "Sort Artist", SortAlbum: "Sort Album", Date: "2020"}
	scan := newMusicScanContext(nil)
	scanTaggedTrack(t, s, scan, fixtureDir+"/one", 1, tags)
	_, err := s.tx.DB.Exec(`
 CREATE TABLE derived_writes(entity TEXT);
 CREATE TRIGGER count_artist_sort AFTER UPDATE OF sort_name ON musicians BEGIN INSERT INTO derived_writes VALUES('artist'); END;
 CREATE TRIGGER count_album_sort AFTER UPDATE OF sort_title ON albums BEGIN INSERT INTO derived_writes VALUES('album sort'); END;
 CREATE TRIGGER count_album_date AFTER UPDATE OF release_date ON albums BEGIN INSERT INTO derived_writes VALUES('album date'); END;
 CREATE TRIGGER count_album_year AFTER UPDATE OF year ON albums BEGIN INSERT INTO derived_writes VALUES('album year'); END;
 `)
	if err != nil {
		t.Fatal(err)
	}
	scanTaggedTrack(t, s, scan, fixtureDir+"/one", 2, tags)
	s.spotify = &musicScannerSpotifyStub{artist: &spotifylib.FullArtist{SimpleArtist: spotifylib.SimpleArtist{ID: "artist"}}, album: &spotifylib.FullAlbum{SimpleAlbum: spotifylib.SimpleAlbum{ID: "album", ReleaseDate: "1999", ReleaseDatePrecision: "year"}}}
	err = s.retrySpotify(context.Background(), newMusicScanContext(nil), newScanReport(Status{}))
	if err != nil {
		t.Fatal(err)
	}

	count := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM derived_writes")
	if count != 0 {
		t.Fatalf("unchanged derived metadata caused %d writes", count)
	}
}

// Persisting one compound credit can merge its local artist into the existing
// row that already owns the Spotify identity, deleting the local row while
// later credits are still being written. persistMusicians re-reads the aliases
// whenever a merge happened, so every credit lands on the surviving owner.
func TestCompoundCreditMergeRepointsEarlierCredits(t *testing.T) {
	s := setupMusicScanner(t)
	ctx := context.Background()

	// One local artist row aliased by both credited names, and a separate row
	// that already owns the Spotify identity the second credit resolves to.
	local, err := s.queries.UpsertMusician(ctx, database.UpsertMusicianParams{Name: "Artist One", SortName: "Artist One"})
	if err != nil {
		t.Fatal(err)
	}
	owner, err := s.queries.UpsertMusician(ctx, database.UpsertMusicianParams{
		Name: "Canonical Two", SortName: "Canonical Two", SpotifyID: sql.NullString{String: "s2", Valid: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	err = s.queries.SaveMusicArtistIdentity(ctx, database.SaveMusicArtistIdentityParams{IdentityKey: "artist one", MusicianID: local.ID})
	if err != nil {
		t.Fatal(err)
	}

	s.spotify = &musicScannerSpotifyStub{
		artist: &spotifylib.FullArtist{SimpleArtist: spotifylib.SimpleArtist{ID: "s2", Name: "Canonical Two"}},
	}
	scanTaggedTrack(t, s, newMusicScanContext(nil), filepath.Join(t.TempDir(), "one"), 1,
		ffprobe.FormatTags{Title: "One", Artist: "Artist One, Artist Two"})

	if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM musicians WHERE id = ?", local.ID) != 0 {
		t.Fatal("the redundant artist was not merged away")
	}
	credited := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM track_musicians WHERE musician_id = ?", owner.ID)
	if credited != 1 {
		t.Fatalf("track credits pointing at the surviving owner = %d, want 1", credited)
	}
	if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM track_musicians") != 1 {
		t.Fatal("track kept a credit for the merged-away artist")
	}
	var primary sql.NullInt64
	err = s.tx.DB.QueryRow("SELECT musician_id FROM tracks").Scan(&primary)
	if err != nil {
		t.Fatal(err)
	}
	if !primary.Valid || primary.Int64 != owner.ID {
		t.Fatalf("primary artist = %#v, want the surviving owner %d", primary, owner.ID)
	}
}
