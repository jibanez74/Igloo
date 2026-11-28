package music

import (
	"context"
	"database/sql"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/itunes"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/scannertest"
)

// scanUnderRoot scans files, keyed by their path under root, as a library
// scan of root would.
func scanUnderRoot(t *testing.T, s *Scanner, root string, size int64, files map[string]ffprobe.FormatTags) {
	t.Helper()
	ctx := context.Background()
	results := make(map[string]*ffprobe.FfprobeResult, len(files))
	scanFiles := make([]scanner.ScanFile, 0, len(files))
	for relative, tags := range files {
		path := filepath.Join(root, relative)
		results[path] = testMusicMetadataWithTags(tags)
		scanFiles = append(scanFiles, scanner.ScanFile{Path: path, Ext: strings.TrimPrefix(filepath.Ext(path), "."), Size: size})
	}
	slices.SortFunc(scanFiles, func(a, b scanner.ScanFile) int { return strings.Compare(a.Path, b.Path) })
	s.ffprobe = newMusicScannerFfprobeByPath(results)
	prepareMusicFixtures(t, scanFiles)
	index, _, err := s.loadMusicScanIndex(ctx, nil, nil)
	if err != nil {
		t.Fatalf("load music scan index: %v", err)
	}
	report := newScanReport(Status{})
	s.processMusicBatch(ctx, newMusicScanContext(index), report, root, scanFiles)
	if report.status.Failed != 0 {
		t.Fatalf("scan failed %d files: %+v", report.status.Failed, s.logger.(*scannertest.Logger).WarnEntries)
	}
}

type structureTrack struct {
	title, album, albumArtist, albumType string
	disc, index                          int64
	year                                 sql.NullInt64
	credits                              string
}

// structureTracks reads each track under root by its path below it.
func structureTracks(t *testing.T, s *Scanner, root string) map[string]structureTrack {
	t.Helper()
	rows, err := s.tx.DB.Query(`SELECT t.file_path, t.title, COALESCE(a.title,''), COALESCE(a.musician,''), COALESCE(a.album_type,''),
		t.disc, t.track_index, a.year,
		COALESCE((SELECT group_concat(m.name, '|') FROM (SELECT m.name FROM track_musicians tm JOIN musicians m ON m.id=tm.musician_id
			WHERE tm.track_id=t.id ORDER BY m.id) m), '')
		FROM tracks t LEFT JOIN albums a ON a.id=t.album_id`)
	if err != nil {
		t.Fatalf("read tracks: %v", err)
	}
	defer rows.Close()
	tracks := map[string]structureTrack{}
	for rows.Next() {
		var path string
		var track structureTrack
		err = rows.Scan(&path, &track.title, &track.album, &track.albumArtist, &track.albumType, &track.disc, &track.index, &track.year, &track.credits)
		if err != nil {
			t.Fatalf("scan track: %v", err)
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatal(err)
		}
		tracks[relative] = track
	}
	err = rows.Err()
	if err != nil {
		t.Fatalf("read tracks: %v", err)
	}
	return tracks
}

func TestScanResolvesAlbumStructureFromFlagsFoldersAndNames(t *testing.T) {
	s := setupMusicScanner(t)
	root := t.TempDir()
	scanUnderRoot(t, s, root, 5, map[string]ffprobe.FormatTags{
		"Compilations/Pure Dance/01 Singer Alpha - Hit One.mp3":  {Title: "Hit One", Artist: "Singer Alpha", Album: "Pure Dance", Track: "1/3", Compilation: "1"},
		"Compilations/Pure Dance/02 Singer Beta - Hit Two.mp3":   {Title: "Hit Two", Artist: "Singer Beta", Album: "Pure Dance", Track: "2/3", Compilation: "1"},
		"Bruce Springsteen/The River/Disc 1/01 Song 1-1.mp3":     {Title: "Song 1-1", Artist: "Bruce Springsteen", Album: "The River", Track: "1"},
		"Bruce Springsteen/The River/Disc 2/01 Song 2-1.mp3":     {Title: "Song 2-1", Artist: "Bruce Springsteen", Album: "The River", Track: "1"},
		"Dido/No Angel (1999)/01 - Here With Me.mp3":             {},
		"Dido/No Angel (1999)/02 - Hunter.mp3":                   {},
		"Norah Jones - Don't Know Why.mp3":                       {},
		"Various Artists/Summer/03 Singer Gamma - Hit Three.mp3": {},
		"Juanes/Mi Sangre/01 La Camisa Negra.flac":               {Title: "La Camisa Negra", Artist: "Juanes", Album: "Mi Sangre"},
		"Juanes/Mi Sangre/04 No Siento Penas.flac":               {Title: "No Siento Penas", Artist: "Juanes feat. Nelly Furtado", Album: "Mi Sangre"},
		"Juanes/Mi Sangre/05 Twice.flac":                         {Title: "Twice", Artist: "Juanes (feat. Juanes)", Album: "Mi Sangre", Track: "5"},
	})

	got := structureTracks(t, s, root)
	want := map[string]structureTrack{
		"Compilations/Pure Dance/01 Singer Alpha - Hit One.mp3":  {title: "Hit One", album: "Pure Dance", albumArtist: variousArtists, albumType: "compilation", index: 1, credits: "Singer Alpha"},
		"Compilations/Pure Dance/02 Singer Beta - Hit Two.mp3":   {title: "Hit Two", album: "Pure Dance", albumArtist: variousArtists, albumType: "compilation", index: 2, credits: "Singer Beta"},
		"Bruce Springsteen/The River/Disc 1/01 Song 1-1.mp3":     {title: "Song 1-1", album: "The River", albumArtist: "Bruce Springsteen", disc: 1, index: 1, credits: "Bruce Springsteen"},
		"Bruce Springsteen/The River/Disc 2/01 Song 2-1.mp3":     {title: "Song 2-1", album: "The River", albumArtist: "Bruce Springsteen", disc: 2, index: 1, credits: "Bruce Springsteen"},
		"Dido/No Angel (1999)/01 - Here With Me.mp3":             {title: "Here With Me", album: "No Angel", albumArtist: "Dido", index: 1, year: sql.NullInt64{Int64: 1999, Valid: true}, credits: "Dido"},
		"Dido/No Angel (1999)/02 - Hunter.mp3":                   {title: "Hunter", album: "No Angel", albumArtist: "Dido", index: 2, year: sql.NullInt64{Int64: 1999, Valid: true}, credits: "Dido"},
		"Norah Jones - Don't Know Why.mp3":                       {title: "Don't Know Why", credits: "Norah Jones"},
		"Various Artists/Summer/03 Singer Gamma - Hit Three.mp3": {title: "Hit Three", album: "Summer", albumArtist: variousArtists, albumType: "compilation", index: 3, credits: "Singer Gamma"},
		"Juanes/Mi Sangre/01 La Camisa Negra.flac":               {title: "La Camisa Negra", album: "Mi Sangre", albumArtist: "Juanes", index: 1, credits: "Juanes"},
		"Juanes/Mi Sangre/04 No Siento Penas.flac":               {title: "No Siento Penas", album: "Mi Sangre", albumArtist: "Juanes", index: 4, credits: "Juanes|Nelly Furtado"},
		"Juanes/Mi Sangre/05 Twice.flac":                         {title: "Twice", album: "Mi Sangre", albumArtist: "Juanes", index: 5, credits: "Juanes"},
	}
	for path, wantTrack := range want {
		gotTrack, ok := got[path]
		if !ok {
			t.Errorf("%s: not imported", path)
			continue
		}
		if gotTrack != wantTrack {
			t.Errorf("%s:\n got %+v\nwant %+v", path, gotTrack, wantTrack)
		}
	}
	albums := scannertest.CountRows(t, s.tx.DB, "SELECT COUNT(*) FROM albums")
	if albums != 5 {
		t.Fatalf("albums = %d, want 5: Pure Dance, The River, No Angel, Summer, Mi Sangre", albums)
	}
	var primary string
	err := s.tx.DB.QueryRow("SELECT m.name FROM tracks t JOIN musicians m ON m.id=t.musician_id WHERE t.title='No Siento Penas'").Scan(&primary)
	if err != nil {
		t.Fatalf("read primary artist: %v", err)
	}
	if primary != "Juanes" {
		t.Fatalf("primary artist = %q, want the main credit Juanes", primary)
	}
}

// Every fallback reads a blank tag only, so a fully tagged file resolves the
// same wherever it sits.
func TestResolveTrackFileKeepsFullyTaggedValuesWhereverTheFileSits(t *testing.T) {
	tags := ffprobe.FormatTags{
		Title: "Tagged Title", Artist: "Tagged Artist", AlbumArtist: "Tagged Album Artist", Album: "Tagged Album",
		Track: "7/12", Disc: "2/2", Date: "2001-04-05",
	}
	root := t.TempDir()
	plain := filepath.Join(t.TempDir(), "track.mp3")
	nested := filepath.Join(root, "Compilations", "Other Album (1999)", "Disc 3", "09 - Other Artist - Other Title.mp3")
	s := setupMusicScanner(t)
	s.ffprobe = newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{
		plain:  testMusicMetadataWithTags(tags),
		nested: testMusicMetadataWithTags(tags),
	})
	resolve := func(root, path string) *resolvedTrack {
		resolved, err := s.resolveTrackFile(context.Background(), newMusicScanContext(nil), root, scanner.ScanFile{Path: path, Ext: "mp3", Size: 1}, itunes.CatalogIDs{})
		if err != nil {
			t.Fatalf("resolve %s: %v", path, err)
		}
		return resolved
	}
	want := resolve("", plain)
	got := resolve(root, nested)

	type values struct {
		title, sortTitle, artistTag string
		index, disc                 int64
		year                        sql.NullInt64
		album, albumArtist          string
		credits                     int
		compilation                 bool
	}
	read := func(r *resolvedTrack) values {
		return values{r.params.Title, r.params.SortTitle, r.artistTag, r.params.TrackIndex, r.params.Disc, r.params.Year,
			r.album.title, r.album.albumArtist, len(r.musicians), r.tags.compilation}
	}
	if read(got) != read(want) {
		t.Fatalf("nested file resolved %+v, want %+v as at a plain path", read(got), read(want))
	}
}

// The tracks' compilation flag decides the album type over any provider's,
// and the provider's type returns once the flag goes.
func TestAlbumTypeTakesTheCompilationFlagOverTheProviders(t *testing.T) {
	s := setupMusicScanner(t)
	ctx := context.Background()
	root := t.TempDir()
	flagged := map[string]ffprobe.FormatTags{
		"Pure Dance/01.mp3": {Title: "One", Artist: "Singer Alpha", AlbumArtist: variousArtists, Album: "Pure Dance", Compilation: "1"},
		"Pure Dance/02.mp3": {Title: "Two", Artist: "Singer Beta", AlbumArtist: variousArtists, Album: "Pure Dance", Compilation: "1"},
	}
	scanUnderRoot(t, s, root, 5, flagged)
	var albumID int64
	err := s.tx.DB.QueryRow("SELECT id FROM albums WHERE title='Pure Dance'").Scan(&albumID)
	if err != nil {
		t.Fatalf("read album: %v", err)
	}
	albumType := func() string {
		t.Helper()
		var value sql.NullString
		err := s.tx.DB.QueryRow("SELECT album_type FROM albums WHERE id=?", albumID).Scan(&value)
		if err != nil {
			t.Fatalf("read album type: %v", err)
		}
		return value.String
	}
	if albumType() != "compilation" {
		t.Fatalf("album type = %q, want compilation from the flag", albumType())
	}

	err = saveProviderAlbumType(ctx, s.queries, albumID, "album")
	if err != nil {
		t.Fatalf("save provider type: %v", err)
	}
	if albumType() != "compilation" {
		t.Fatalf("album type after a provider match = %q, want the flag's compilation", albumType())
	}
	err = saveProviderAlbumType(ctx, s.queries, albumID, "ep")
	if err != nil {
		t.Fatalf("save unknown provider type: %v", err)
	}

	// One flagged track of two is no majority.
	scanUnderRoot(t, s, root, 6, map[string]ffprobe.FormatTags{
		"Pure Dance/01.mp3": {Title: "One", Artist: "Singer Alpha", AlbumArtist: variousArtists, Album: "Pure Dance", Compilation: "1"},
		"Pure Dance/02.mp3": {Title: "Two", Artist: "Singer Beta", AlbumArtist: variousArtists, Album: "Pure Dance"},
	})
	if albumType() != "album" {
		t.Fatalf("album type without a flag majority = %q, want the provider's album, kept past an unknown type", albumType())
	}

	err = s.queries.ClearMusicAlbumSpotifyFallback(ctx, albumID)
	if err != nil {
		t.Fatalf("clear fallback: %v", err)
	}
	err = reconcileAlbum(ctx, s.queries, albumID)
	if err != nil {
		t.Fatalf("reconcile album: %v", err)
	}
	if albumType() != "" {
		t.Fatalf("album type after the match is dropped = %q, want none", albumType())
	}
}

// A merge keeps the owner's provider type, or takes the redundant album's
// when the owner has none.
func TestMergeMusicAlbumCarriesTheProviderAlbumType(t *testing.T) {
	s := setupMusicScanner(t)
	ctx := context.Background()
	root := t.TempDir()
	scanUnderRoot(t, s, root, 5, map[string]ffprobe.FormatTags{
		"A/01.mp3": {Title: "One", Artist: "Artist", Album: "Owner"},
		"B/01.mp3": {Title: "Two", Artist: "Artist", Album: "Redundant"},
	})
	ids := map[string]int64{}
	for _, title := range []string{"Owner", "Redundant"} {
		var id int64
		err := s.tx.DB.QueryRow("SELECT id FROM albums WHERE title=?", title).Scan(&id)
		if err != nil {
			t.Fatalf("read %s: %v", title, err)
		}
		ids[title] = id
	}
	err := saveProviderAlbumType(ctx, s.queries, ids["Redundant"], "single")
	if err != nil {
		t.Fatalf("save provider type: %v", err)
	}
	err = s.mergeMusicAlbum(ctx, s.queries, newMusicScanContext(nil), ids["Redundant"], ids["Owner"])
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	var albumType sql.NullString
	err = s.tx.DB.QueryRow("SELECT album_type FROM albums WHERE id=?", ids["Owner"]).Scan(&albumType)
	if err != nil {
		t.Fatalf("read owner: %v", err)
	}
	if albumType.String != "single" {
		t.Fatalf("owner album type = %#v, want the redundant album's single", albumType)
	}
}
