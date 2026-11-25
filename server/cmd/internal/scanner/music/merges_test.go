package music

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
)

func TestMergeMusicArtistMovesMusicBrainzIDVotes(t *testing.T) {
	s := setupMusicScanner(t)
	dir := t.TempDir()
	const kept = "3a000000-0000-4000-8000-000000000000"
	const moved = "3b000000-0000-4000-8000-000000000000"
	scanTaggedTrack(t, s, newMusicScanContext(nil), filepath.Join(dir, "one"), 1, ffprobe.FormatTags{Title: "One", Artist: "Owner", MusicBrainzArtistID: kept})
	scanTaggedTrack(t, s, newMusicScanContext(nil), filepath.Join(dir, "two"), 1, ffprobe.FormatTags{Title: "Two", Artist: "Alias", MusicBrainzArtistID: moved})
	scanTaggedTrack(t, s, newMusicScanContext(nil), filepath.Join(dir, "three"), 1, ffprobe.FormatTags{Title: "Three", Artist: "Alias", MusicBrainzArtistID: moved})
	var owner, redundant int64
	err := s.tx.DB.QueryRow("SELECT id FROM musicians WHERE name = 'Owner'").Scan(&owner)
	if err != nil {
		t.Fatal(err)
	}
	err = s.tx.DB.QueryRow("SELECT id FROM musicians WHERE name = 'Alias'").Scan(&redundant)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	err = s.tx.Run(ctx, func(qtx *database.Queries) error {
		return s.mergeMusicArtist(ctx, qtx, newMusicScanContext(nil), redundant, owner)
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	var id sql.NullString
	err = s.tx.DB.QueryRow("SELECT musicbrainz_id FROM musicians WHERE id = ?", owner).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	if id.String != moved {
		t.Fatalf("owner musicbrainz_id = %q, want %q from the two moved votes", id.String, moved)
	}
}
