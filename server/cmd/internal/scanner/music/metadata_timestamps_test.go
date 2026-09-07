package music

import (
	"context"
	"database/sql"
	"testing"

	"igloo/cmd/internal/database"
)

func TestMusicMetadataUpdatesTimestamp(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name, table, setup, predicate string
		derived                       bool
		update                        func(*database.Queries) error
	}{
		{"artist enrichment", "musicians", "", "summary='Summary' AND spotify_popularity=42 AND spotify_followers=100", false, func(q *database.Queries) error {
			return q.UpdateMusicArtistEnrichment(ctx, database.UpdateMusicArtistEnrichmentParams{ID: 1, Summary: sql.NullString{String: "Summary", Valid: true}, SpotifyPopularity: sql.NullFloat64{Float64: 42, Valid: true}, SpotifyFollowers: sql.NullInt64{Int64: 100, Valid: true}})
		}},
		{"album enrichment", "albums", "", "spotify_popularity=42 AND total_tracks=10", false, func(q *database.Queries) error {
			return q.UpdateMusicAlbumEnrichment(ctx, database.UpdateMusicAlbumEnrichmentParams{ID: 1, SpotifyPopularity: sql.NullFloat64{Float64: 42, Valid: true}, TotalTracks: sql.NullInt64{Int64: 10, Valid: true}})
		}},
		{"artist Spotify ID", "musicians", "", "spotify_id='artist'", false, func(q *database.Queries) error {
			return q.SetMusicArtistSpotifyID(ctx, database.SetMusicArtistSpotifyIDParams{ID: 1, SpotifyID: sql.NullString{String: "artist", Valid: true}})
		}},
		{"album Spotify ID", "albums", "", "spotify_id='album'", false, func(q *database.Queries) error {
			return q.SetMusicAlbumSpotifyID(ctx, database.SetMusicAlbumSpotifyIDParams{ID: 1, SpotifyID: sql.NullString{String: "album", Valid: true}})
		}},
		{"artist sort", "musicians", "", "sort_name='Artist'", true, func(q *database.Queries) error { return q.ReconcileMusicArtistSort(ctx, 1) }},
		{"album sort", "albums", "", "sort_title='Album'", true, func(q *database.Queries) error { return q.ReconcileMusicAlbumSort(ctx, 1) }},
		{"album date", "albums", "INSERT INTO music_album_metadata(album_id,spotify_date) VALUES(1,'2020-01-01')", "release_date='2020-01-01'", true, func(q *database.Queries) error { return q.ReconcileMusicAlbumDate(ctx, 1) }},
		{"album year", "albums", "UPDATE albums SET release_date='2020-01-01'", "year=2020", true, func(q *database.Queries) error { return q.ReconcileMusicAlbumYear(ctx, 1) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := setupMusicScanner(t)
			defer s.db.Close()
			_, err := s.db.Exec(`INSERT INTO musicians(id,name,sort_name) VALUES(1,'Artist','Old'); INSERT INTO albums(id,title,sort_title) VALUES(1,'Album','Old');`)
			if err != nil {
				t.Fatal(err)
			}
			if tc.setup != "" {
				_, err = s.db.Exec(tc.setup)
				if err != nil {
					t.Fatal(err)
				}
			}
			_, err = s.db.Exec("UPDATE " + tc.table + " SET updated_at=datetime('now','-1 hour') WHERE id=1")
			if err != nil {
				t.Fatal(err)
			}
			var old string
			err = s.db.QueryRow("SELECT updated_at FROM " + tc.table + " WHERE id=1").Scan(&old)
			if err != nil {
				t.Fatal(err)
			}
			err = tc.update(s.queries)
			if err != nil {
				t.Fatal(err)
			}
			count := countScannerRows(t, s.db, "SELECT count(*) FROM "+tc.table+" WHERE id=1 AND updated_at>? AND ("+tc.predicate+")", old)
			if count != 1 {
				t.Fatal("metadata or timestamp did not update")
			}
			if !tc.derived {
				return
			}
			_, err = s.db.Exec("UPDATE "+tc.table+" SET updated_at=? WHERE id=1", old)
			if err != nil {
				t.Fatal(err)
			}
			_, err = s.db.Exec("CREATE TABLE writes(id INTEGER); CREATE TRIGGER count_writes AFTER UPDATE ON " + tc.table + " BEGIN INSERT INTO writes VALUES(NEW.id); END;")
			if err != nil {
				t.Fatal(err)
			}
			err = tc.update(s.queries)
			if err != nil {
				t.Fatal(err)
			}
			count = countScannerRows(t, s.db, "SELECT count(*) FROM writes")
			unchanged := countScannerRows(t, s.db, "SELECT count(*) FROM "+tc.table+" WHERE id=1 AND updated_at=?", old)
			if count != 0 || unchanged != 1 {
				t.Fatal("unchanged derived metadata wrote or advanced timestamp")
			}
		})
	}
}

func TestMusicImageHelpersTimestamp(t *testing.T) {
	for _, entity := range []struct{ table, column string }{{"musicians", "thumb"}, {"albums", "cover"}} {
		for _, input := range []string{"", "old.jpg", "new.jpg"} {
			t.Run(entity.table+"/"+input, func(t *testing.T) {
				s := setupMusicScanner(t)
				defer s.db.Close()
				_, err := s.db.Exec(`INSERT INTO musicians(id,name,sort_name,thumb,updated_at) VALUES(1,'Artist','Artist','old.jpg',datetime('now','-1 hour'));
 INSERT INTO albums(id,title,sort_title,cover,updated_at) VALUES(1,'Album','Album','old.jpg',datetime('now','-1 hour'));`)
				if err != nil {
					t.Fatal(err)
				}
				var old string
				err = s.db.QueryRow("SELECT updated_at FROM " + entity.table + " WHERE id=1").Scan(&old)
				if err != nil {
					t.Fatal(err)
				}
				_, err = s.db.Exec("CREATE TABLE writes(id INTEGER); CREATE TRIGGER count_writes AFTER UPDATE ON " + entity.table + " BEGIN INSERT INTO writes VALUES(NEW.id); END;")
				if err != nil {
					t.Fatal(err)
				}
				image := sql.NullString{String: "old.jpg", Valid: true}
				var returned sql.NullString
				if entity.table == "musicians" {
					row, updateErr := s.updateMusicianThumbIfChanged(context.Background(), s.queries, database.Musician{ID: 1, Thumb: image}, input)
					err = updateErr
					returned = row.Thumb
				} else {
					row, updateErr := s.updateAlbumCoverIfChanged(context.Background(), s.queries, database.Album{ID: 1, Cover: image}, input)
					err = updateErr
					returned = row.Cover
				}
				if err != nil {
					t.Fatal(err)
				}
				var value, updated string
				err = s.db.QueryRow("SELECT "+entity.column+",updated_at FROM "+entity.table+" WHERE id=1").Scan(&value, &updated)
				if err != nil {
					t.Fatal(err)
				}
				writes := countScannerRows(t, s.db, "SELECT count(*) FROM writes")
				want := "old.jpg"
				if input == "new.jpg" {
					want = input
					if writes != 1 || updated <= old {
						t.Fatal("changed image did not write and advance timestamp")
					}
				} else if writes != 0 || updated != old {
					t.Fatal("unchanged image wrote or advanced timestamp")
				}
				if value != want || !returned.Valid || returned.String != want {
					t.Fatalf("stored=%q returned=%+v, want %q", value, returned, want)
				}
			})
		}
	}
}
