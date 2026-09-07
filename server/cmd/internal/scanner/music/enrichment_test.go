package music

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	spotifylib "github.com/zmb3/spotify/v2"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/scanner"
	spotifyapi "igloo/cmd/internal/spotify"
)

func scanTaggedTrack(t *testing.T, s *Scanner, scan *musicScanContext, path string, size int64, tags ffprobe.FormatTags) {
	t.Helper()
	s.ffprobe = &countingMusicScannerFfprobe{result: testMusicMetadataWithTags(tags)}
	n, _, failures := s.processMusicBatch(context.Background(), scan, []scanner.ScanFile{{Path: path, Ext: "m4a", Size: size}})
	if n != 1 || failures != 0 {
		t.Fatalf("scan %s: scanned=%d failures=%d logs=%+v", path, n, failures, s.logger.(*capturedLogger).warnEntries)
	}
}

func TestSpotifyRetriesUnchangedCatalog(t *testing.T) {
	for _, offline := range []bool{false, true} {
		t.Run(fmt.Sprintf("offline_%v", offline), func(t *testing.T) {
			s := setupMusicScanner(t)
			defer s.db.Close()
			dir := t.TempDir()
			err := os.WriteFile(filepath.Join(dir, "track.m4a"), []byte("audio"), 0600)
			if err != nil {
				t.Fatal(err)
			}
			probe := &countingMusicScannerFfprobe{result: testMusicMetadata()}
			s.ffprobe = probe
			stub := &musicScannerSpotifyStub{artistErr: errors.New("temporary"), albumErr: errors.New("temporary")}
			if !offline {
				s.spotify = stub
			}
			s.runMusicScan(dir)
			if probe.calls != 1 {
				t.Fatalf("probes=%d", probe.calls)
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
			s.runMusicScan(dir)
			if probe.calls != 1 || stub.artistCalls != beforeArtist+1 || stub.albumCalls != beforeAlbum+1 {
				t.Fatalf("recovery probes=%d artist=%d album=%d", probe.calls, stub.artistCalls, stub.albumCalls)
			}
			count1 := countScannerRows(t, s.db, "SELECT count(*) FROM music_spotify_matches WHERE status='matched'")
			if count1 != 2 {
				t.Fatal("recovery did not persist matches")
			}
			s.runMusicScan(dir)
			if probe.calls != 1 || stub.artistCalls != beforeArtist+1 || stub.albumCalls != beforeAlbum+1 {
				t.Fatal("final outcomes were retried")
			}
		})
	}
}

func TestMusicNormalizedIdentityAndMutableMetadata(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(fmt.Sprintf("reverse_%v", reverse), func(t *testing.T) {
			s := setupMusicScanner(t)
			defer s.db.Close()
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
				scanTaggedTrack(t, s, scan, fmt.Sprintf("/%d.m4a", j), 1, tags[j])
			}
			for _, table := range []string{"musicians", "albums", "genres"} {
				count2 := countScannerRows(t, s.db, "SELECT count(*) FROM "+table)
				if count2 != 1 {
					t.Fatalf("duplicate %s", table)
				}
			}
			var sortName, sortTitle, date string
			var year int
			err := s.db.QueryRow("SELECT m.sort_name,a.sort_title,a.release_date,a.year FROM musicians m CROSS JOIN albums a").Scan(&sortName, &sortTitle, &date, &year)
			if err != nil {
				t.Fatal(err)
			}
			if sortName != "Z" || sortTitle != "Z" || date != "2020-01-01" || year != 2020 {
				t.Fatalf("majority=%s/%s/%s/%d", sortName, sortTitle, date, year)
			}
			tags[2].SortArtist = ""
			tags[2].SortAlbum = ""
			tags[2].Date = ""
			scanTaggedTrack(t, s, newMusicScanContext(nil), "/2.m4a", 2, tags[2])
			err = s.db.QueryRow("SELECT m.sort_name,a.sort_title,a.release_date FROM musicians m CROSS JOIN albums a").Scan(&sortName, &sortTitle, &date)
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
				scanTaggedTrack(t, s, newMusicScanContext(nil), fmt.Sprintf("/%d.m4a", i), 2, tags[i])
			}
			count3 := countScannerRows(t, s.db, "SELECT count(*) FROM albums WHERE sort_title=title AND release_date IS NULL AND year IS NULL")
			count4 := countScannerRows(t, s.db, "SELECT count(*) FROM musicians WHERE sort_name=name")
			if count4 != 1 || count3 != 1 {
				t.Fatal("removed tags did not restore fallback")
			}
			for _, tag := range []string{"Movie", "movie"} {
				_, err = s.queries.GetOrCreateGenre(context.Background(), database.GetOrCreateGenreParams{Tag: tag, GenreType: "movie"})
				if err != nil {
					t.Fatal(err)
				}
			}
			count5 := countScannerRows(t, s.db, "SELECT count(*) FROM genres WHERE genre_type='movie'")
			if count5 != 2 {
				t.Fatal("movie identity changed")
			}
		})
	}
}

func TestMusicRelationshipsAndSpotifyDateFallback(t *testing.T) {
	s := setupMusicScanner(t)
	defer s.db.Close()
	s.spotify = &musicScannerSpotifyStub{artist: &spotifylib.FullArtist{SimpleArtist: spotifylib.SimpleArtist{ID: "artist"}, Genres: []string{"Rock"}}, album: &spotifylib.FullAlbum{SimpleAlbum: spotifylib.SimpleAlbum{ID: "album", ReleaseDate: "2000", ReleaseDatePrecision: "year"}, Genres: []string{"Rock"}}}
	scan := newMusicScanContext(nil)
	tags := ffprobe.FormatTags{Title: "Track", Artist: "Artist", Album: "Album", Genre: "Rock", Date: "2022"}
	scanTaggedTrack(t, s, scan, "/one", 1, tags)
	scanTaggedTrack(t, s, scan, "/two", 1, tags)
	count6 := countScannerRows(t, s.db, "SELECT count(*) FROM musician_genres")
	if count6 != 2 {
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
	scanTaggedTrack(t, s, scan, "/one", 2, tags)
	count7 := countScannerRows(t, s.db, "SELECT count(*) FROM musician_albums")
	if count7 != 1 {
		t.Fatal("removed multiply supported link")
	}
	// Remove the last local date and genre while retaining album membership.
	tags.Artist = "Artist"
	tags.Album = "Album"
	scanTaggedTrack(t, s, scan, "/two", 2, tags)
	count8 := countScannerRows(t, s.db, "SELECT count(*) FROM albums WHERE release_date='2000-01-01' AND year=2000")
	if count8 != 1 {
		t.Fatal("missing Spotify date fallback")
	}
	count9 := countScannerRows(t, s.db, "SELECT count(*) FROM album_genres WHERE source='spotify'")
	count10 := countScannerRows(t, s.db, "SELECT count(*) FROM musician_genres WHERE source='local'")
	if count10 != 0 || count9 != 1 {
		t.Fatal("genre provenance removal")
	}
	tags.Genre = "Rock"
	tags.SortArtist = "Sort"
	scanTaggedTrack(t, s, scan, "/two", 3, tags)
	count11 := countScannerRows(t, s.db, "SELECT count(*) FROM musician_genres WHERE source='local'")
	if count11 != 1 {
		t.Fatal("local genre not restored within scan")
	}
	err = s.queries.DeleteAlbum(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	count12 := countScannerRows(t, s.db, "SELECT count(*) FROM musician_genres WHERE source='local'")
	count13 := countScannerRows(t, s.db, "SELECT count(*) FROM musician_albums")
	if count13 != 0 || count12 != 0 {
		t.Fatal("album deletion left local relationships")
	}
	count14 := countScannerRows(t, s.db, "SELECT count(*) FROM musicians WHERE sort_name=name")
	if count14 != 1 {
		t.Fatal("album deletion left sort contribution")
	}
	count15 := countScannerRows(t, s.db, "SELECT count(*) FROM musician_genres WHERE source='spotify'")
	if count15 != 1 {
		t.Fatal("album deletion removed Spotify genre")
	}
}

func TestSpotifyRetrySplitsPersistedCompoundCredits(t *testing.T) {
	s := setupMusicScanner(t)
	defer s.db.Close()
	scanTaggedTrack(t, s, newMusicScanContext(nil), "/one", 1, ffprobe.FormatTags{Title: "Track", Artist: "One & Two", Album: "Album", Genre: "Rock", SortArtist: "First & Second"})
	probe := s.ffprobe.(*countingMusicScannerFfprobe)
	s.spotify = &musicScannerSpotifyStub{artistErr: &spotifyapi.MatchError{Info: spotifyapi.MatchDebugInfo{Reason: musicSpotifyReasonNoResults}}, albumErr: errors.New("temporary")}
	err := s.retrySpotify(context.Background(), newMusicScanContext(nil))
	if err != nil {
		t.Fatal(err)
	}
	if probe.calls != 1 {
		t.Fatal("retry probed a file")
	}
	count16 := countScannerRows(t, s.db, "SELECT count(*) FROM musician_albums")
	count17 := countScannerRows(t, s.db, "SELECT count(*) FROM track_musicians")
	if count17 != 2 || count16 != 2 {
		t.Fatal("compound credits or album links not split")
	}
	count18 := countScannerRows(t, s.db, "SELECT count(*) FROM musicians WHERE (name='One' AND sort_name='First') OR (name='Two' AND sort_name='Second')")
	if count18 != 2 {
		t.Fatal("per-credit sort tags lost")
	}
	count19 := countScannerRows(t, s.db, "SELECT count(*) FROM musician_genres WHERE musician_id=(SELECT id FROM musicians WHERE name='One & Two')")
	if count19 != 0 {
		t.Fatal("old compound genre relationship remains")
	}
}

func TestSpotifyMergesPreserveTracksAndRollback(t *testing.T) {
	s := setupMusicScanner(t)
	defer s.db.Close()
	ctx := context.Background()
	for i := 1; i <= 2; i++ {
		scanTaggedTrack(t, s, newMusicScanContext(nil), fmt.Sprintf("/%d", i), 1, ffprobe.FormatTags{Title: "Track", Artist: fmt.Sprintf("Artist %d", i), Album: fmt.Sprintf("Album %d", i), SortArtist: fmt.Sprintf("Sort %d", i), Genre: "Rock"})
	}
	_, err := s.db.Exec("INSERT INTO users(id,email,password,name) VALUES(1,'test@example.com','hash','Test')")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.db.Exec("INSERT INTO playlists(id,name,user_id) VALUES(1,'Music',1)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.db.Exec("INSERT INTO playlist_tracks(playlist_id,track_id,added_by,position) VALUES(1,2,1,0)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.db.Exec("INSERT INTO user_play_history(user_id,track_id,duration_played) VALUES(1,2,60)")
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
	_, err = s.db.Exec("CREATE TRIGGER reject_merge BEFORE DELETE ON musicians WHEN OLD.id=2 BEGIN SELECT RAISE(ABORT,'merge failure'); END")
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
	count20 := countScannerRows(t, s.db, "SELECT count(*) FROM music_artist_identity WHERE musician_id=2")
	count21 := countScannerRows(t, s.db, "SELECT count(*) FROM tracks WHERE id=2 AND musician_id=2")
	if count21 != 1 || count20 != 1 {
		t.Fatal("merge rollback changed references")
	}
	_, err = s.db.Exec("DROP TRIGGER reject_merge")
	if err != nil {
		t.Fatal(err)
	}
	err = s.retrySpotify(ctx, scan)
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"musicians", "albums"} {
		count22 := countScannerRows(t, s.db, "SELECT count(*) FROM "+table)
		if count22 != 1 {
			t.Fatalf("merge left duplicate %s", table)
		}
	}
	count23 := countScannerRows(t, s.db, "SELECT count(*) FROM playlist_tracks WHERE track_id=2")
	count24 := countScannerRows(t, s.db, "SELECT count(*) FROM tracks WHERE id IN (1,2) AND musician_id=1 AND album_id=1")
	if count24 != 2 || count23 != 1 {
		t.Fatal("merge changed track identity or playlist")
	}
	count25 := countScannerRows(t, s.db, "SELECT count(*) FROM music_album_identity WHERE album_id=1")
	count26 := countScannerRows(t, s.db, "SELECT count(*) FROM music_artist_identity WHERE musician_id=1")
	if count26 != 2 || count25 != 2 {
		t.Fatal("aliases not retained")
	}
	scanTaggedTrack(t, s, scan, "/3", 1, ffprobe.FormatTags{Title: "Later", Artist: "Artist 2", Album: "Album 2"})
	count27 := countScannerRows(t, s.db, "SELECT count(*) FROM tracks WHERE musician_id=1 AND album_id=1")
	if count27 != 3 {
		t.Fatal("stale cached IDs after merge")
	}
	historyCount := countScannerRows(t, s.db, "SELECT count(*) FROM user_play_history WHERE track_id=2 AND duration_played=60")
	if historyCount != 1 {
		t.Fatal("merge lost history")
	}

	var violations int
	rows, err := s.db.Query("PRAGMA foreign_key_check")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		violations++
	}
	rowsErr := rows.Err()
	if rowsErr != nil || violations != 0 {
		t.Fatalf("foreign keys: %d %v", violations, rows.Err())
	}
}

func TestSpotifyRetryPagesAndCancellation(t *testing.T) {
	s := setupMusicScanner(t)
	defer s.db.Close()
	ctx := context.Background()
	scan := newMusicScanContext(nil)
	for i := 0; i < 105; i++ {
		scanTaggedTrack(t, s, scan, fmt.Sprintf("/page/%d", i), 1, ffprobe.FormatTags{Title: "Track", Artist: fmt.Sprintf("Artist %d", i), Album: fmt.Sprintf("Album %d", i)})
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
	probe := s.ffprobe.(*countingMusicScannerFfprobe)
	stub := &musicScannerSpotifyStub{artistErr: errors.New("temporary"), albumErr: errors.New("temporary")}
	s.spotify = stub
	retryScan := newMusicScanContext(nil)
	err = s.retrySpotify(ctx, retryScan)
	if err != nil {
		t.Fatal(err)
	}
	if stub.artistCalls != 105 || stub.albumCalls != 105 || probe.calls != 1 {
		t.Fatalf("paged attempts artist=%d album=%d probes=%d", stub.artistCalls, stub.albumCalls, probe.calls)
	}
	// Reusing the same scan context cannot issue another request for these entities.
	err = s.retrySpotify(ctx, retryScan)
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
	err = s.retrySpotify(canceled, canceledScan)
	canceledError := errors.Is(err, context.Canceled)
	if !canceledError {
		t.Fatalf("canceled retry=%v", err)
	}
	if len(canceledScan.spotifyArtistMisses) != 0 || len(canceledScan.enrichmentCounts) != 0 {
		t.Fatal("cancellation recorded as failure")
	}
}

func TestMusicAlbumMoveReconcilesBothSides(t *testing.T) {
	s := setupMusicScanner(t)
	defer s.db.Close()
	scan := newMusicScanContext(nil)
	tags := ffprobe.FormatTags{Title: "Track", Artist: "Artist", Album: "First", Genre: "Rock", Date: "2020", SortAlbum: "Sort"}
	scanTaggedTrack(t, s, scan, "/one", 1, tags)
	tags.Date = "2019"
	tags.SortAlbum = ""
	scanTaggedTrack(t, s, scan, "/two", 1, tags)
	tags.Album = "Second"
	tags.Artist = "Other"
	tags.Date = "2021"
	tags.Genre = "Jazz"
	scanTaggedTrack(t, s, scan, "/one", 2, tags)
	first := countScannerRows(t, s.db, "SELECT count(*) FROM albums WHERE title='First' AND release_date='2019-01-01' AND sort_title='First'")
	second := countScannerRows(t, s.db, "SELECT count(*) FROM albums WHERE title='Second' AND release_date='2021-01-01'")
	links := countScannerRows(t, s.db, "SELECT count(*) FROM musician_albums")
	localGenres := countScannerRows(t, s.db, "SELECT count(*) FROM album_genres WHERE source='local'")
	if first != 1 || second != 1 || links != 2 || localGenres != 2 {
		t.Fatalf("move: first=%d second=%d links=%d genres=%d", first, second, links, localGenres)
	}
	tags.Artist = ""
	tags.Album = ""
	tags.Genre = ""
	scanTaggedTrack(t, s, scan, "/two", 2, tags)
	links = countScannerRows(t, s.db, "SELECT count(*) FROM musician_albums")
	localGenres = countScannerRows(t, s.db, "SELECT count(*) FROM album_genres WHERE source='local'")
	if links != 1 || localGenres != 1 {
		t.Fatal("last supporting track left stale relationships")
	}
}

func TestSpotifyAlbumMergeRollback(t *testing.T) {
	s := setupMusicScanner(t)
	defer s.db.Close()
	ctx := context.Background()
	scan := newMusicScanContext(nil)
	for i := 1; i <= 2; i++ {
		scanTaggedTrack(t, s, scan, fmt.Sprintf("/%d", i), 1, ffprobe.FormatTags{Title: "Track", Album: fmt.Sprintf("Album %d", i), Date: fmt.Sprintf("200%d", i)})
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
	_, err = s.db.Exec("CREATE TRIGGER reject_album_merge BEFORE DELETE ON albums WHEN OLD.id=2 BEGIN SELECT RAISE(ABORT,'album merge failure'); END")
	if err != nil {
		t.Fatal(err)
	}
	invalidations := 0
	s.invalidateCommittedTrack = func(int64) { invalidations++ }
	err = s.retrySpotify(ctx, scan)
	if err == nil {
		t.Fatal("expected album merge failure")
	}
	oldTrack := countScannerRows(t, s.db, "SELECT count(*) FROM tracks WHERE id=2 AND album_id=2")
	oldAlias := countScannerRows(t, s.db, "SELECT count(*) FROM music_album_identity WHERE album_id=2")
	if oldTrack != 1 || oldAlias != 1 || invalidations != 0 {
		t.Fatal("album rollback changed references or cache")
	}
	_, err = s.db.Exec("DROP TRIGGER reject_album_merge")
	if err != nil {
		t.Fatal(err)
	}
	err = s.retrySpotify(ctx, scan)
	if err != nil {
		t.Fatal(err)
	}
	tracks := countScannerRows(t, s.db, "SELECT count(*) FROM tracks WHERE album_id=1")
	dates := countScannerRows(t, s.db, "SELECT count(*) FROM albums WHERE id=1 AND release_date='2001-01-01'")
	if tracks != 2 || dates != 1 || invalidations != 1 {
		t.Fatalf("merge tracks=%d dates=%d invalidations=%d", tracks, dates, invalidations)
	}
}

func TestMusicUnchangedDerivedMetadataDoesNotWrite(t *testing.T) {
	s := setupMusicScanner(t)
	defer s.db.Close()
	tags := ffprobe.FormatTags{Title: "Track", Artist: "Artist", Album: "Album", SortArtist: "Sort Artist", SortAlbum: "Sort Album", Date: "2020"}
	scan := newMusicScanContext(nil)
	scanTaggedTrack(t, s, scan, "/one", 1, tags)
	_, err := s.db.Exec(`
 CREATE TABLE derived_writes(entity TEXT);
 CREATE TRIGGER count_artist_sort AFTER UPDATE OF sort_name ON musicians BEGIN INSERT INTO derived_writes VALUES('artist'); END;
 CREATE TRIGGER count_album_sort AFTER UPDATE OF sort_title ON albums BEGIN INSERT INTO derived_writes VALUES('album sort'); END;
 CREATE TRIGGER count_album_date AFTER UPDATE OF release_date ON albums BEGIN INSERT INTO derived_writes VALUES('album date'); END;
 CREATE TRIGGER count_album_year AFTER UPDATE OF year ON albums BEGIN INSERT INTO derived_writes VALUES('album year'); END;
 `)
	if err != nil {
		t.Fatal(err)
	}
	scanTaggedTrack(t, s, scan, "/one", 2, tags)
	s.spotify = &musicScannerSpotifyStub{artist: &spotifylib.FullArtist{SimpleArtist: spotifylib.SimpleArtist{ID: "artist"}}, album: &spotifylib.FullAlbum{SimpleAlbum: spotifylib.SimpleAlbum{ID: "album", ReleaseDate: "1999", ReleaseDatePrecision: "year"}}}
	err = s.retrySpotify(context.Background(), newMusicScanContext(nil))
	if err != nil {
		t.Fatal(err)
	}

	count := countScannerRows(t, s.db, "SELECT count(*) FROM derived_writes")
	if count != 0 {
		t.Fatalf("unchanged derived metadata caused %d writes", count)
	}
}
