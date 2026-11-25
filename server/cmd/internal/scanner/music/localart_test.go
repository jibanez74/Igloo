package music

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner/scannertest"
)

// Minimal files that sniff as their image type.
var (
	jpegBytes = append([]byte{0xff, 0xd8, 0xff, 0xe0}, []byte("jpeg body")...)
	pngBytes  = append([]byte("\x89PNG\r\n\x1a\n"), []byte("png body")...)
)

// fakePictures stands in for FFmpeg: it writes picture as the extracted
// image and records each request.
type fakePictures struct {
	mu      sync.Mutex
	picture []byte
	calls   []string
	err     error
	hook    func()
}

func (f *fakePictures) ExtractAttachedPicture(_ context.Context, source string, index int, dest string) error {
	f.mu.Lock()
	f.calls = append(f.calls, filepath.Base(source)+"#"+string(rune('0'+index)))
	hook, err := f.hook, f.err
	f.mu.Unlock()
	if hook != nil {
		hook()
	}
	if err != nil {
		return err
	}
	return os.WriteFile(dest, f.picture, 0o644)
}

func (f *fakePictures) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// artFixture is a scanner with a library and a static directory of their
// own, and the tracks it probes.
type artFixture struct {
	s         *Scanner
	musicDir  string
	staticDir string
	probes    map[string]*ffprobe.FfprobeResult
	pictures  *fakePictures
}

func newArtFixture(t *testing.T) *artFixture {
	t.Helper()
	f := &artFixture{
		s:         setupMusicScanner(t),
		musicDir:  t.TempDir(),
		staticDir: t.TempDir(),
		probes:    make(map[string]*ffprobe.FfprobeResult),
		pictures:  &fakePictures{picture: pngBytes},
	}
	f.s.ffprobe = newMusicScannerFfprobeByPath(f.probes)
	f.s.pictures = f.pictures
	f.s.staticDir = func() string { return f.staticDir }
	f.s.libraryDirs = func() []string { return []string{f.musicDir} }
	f.s.currentMusicDirectory = func() sql.NullString { return sql.NullString{String: f.musicDir, Valid: true} }
	return f
}

// track writes an audio file at rel, tagged with album, embedding pictures
// as the given attached-picture streams.
func (f *artFixture) track(t *testing.T, rel, album string, pictures ...ffprobe.Stream) string {
	t.Helper()
	path := filepath.Join(f.musicDir, filepath.FromSlash(rel))
	scannertest.WriteFile(t, path, "audio "+rel)
	metadata := testMusicMetadataWithTags(ffprobe.FormatTags{Title: filepath.Base(rel), Artist: "Art Band", Album: album, Track: "1"})
	metadata.Streams = append(metadata.Streams, pictures...)
	f.probes[path] = metadata
	return path
}

func (f *artFixture) image(t *testing.T, rel string, content []byte) string {
	t.Helper()
	path := filepath.Join(f.musicDir, filepath.FromSlash(rel))
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(path, content, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

// scan runs a library scan and waits for the background pass it resumes.
func (f *artFixture) scan(t *testing.T) {
	t.Helper()
	f.s.scan(f.musicDir)
	f.wait(t)
}

func (f *artFixture) wait(t *testing.T) {
	t.Helper()
	scannertest.WaitForGroup(t, f.s.launcher.Wait, 5*time.Second, "music lookups")
}

type albumArt struct {
	id          int64
	cover       sql.NullString
	coverLocal  sql.NullString
	fingerprint sql.NullString
}

func (f *artFixture) album(t *testing.T, title string) albumArt {
	t.Helper()
	var art albumArt
	err := f.s.tx.DB.QueryRow(`SELECT a.id, a.cover, a.cover_local, m.local_cover_fingerprint FROM albums a
		LEFT JOIN music_album_metadata m ON m.album_id = a.id WHERE a.title = ?`, title).
		Scan(&art.id, &art.cover, &art.coverLocal, &art.fingerprint)
	if err != nil {
		t.Fatalf("read album %q: %v", title, err)
	}
	return art
}

// copied reads the copy a cover_local URL names.
func (f *artFixture) copied(t *testing.T, art albumArt) []byte {
	t.Helper()
	if !art.coverLocal.Valid {
		t.Fatal("album has no local cover")
	}
	rest := strings.TrimPrefix(art.coverLocal.String, helpers.STATIC_URL_PREFIX)
	rel, _, _ := strings.Cut(rest, "?v=")
	content, err := os.ReadFile(filepath.Join(f.staticDir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read copy: %v", err)
	}
	return content
}

func picture(index int, comment string) ffprobe.Stream {
	return ffprobe.Stream{Index: index, CodecType: "video", CodecName: "mjpeg", Disposition: ffprobe.StreamDisposition{AttachedPic: 1}, Tags: ffprobe.StreamTags{Comment: comment}}
}

func TestEmbeddedPicturePrefersTheFrontCover(t *testing.T) {
	audio := ffprobe.Stream{Index: 0, CodecType: "audio"}
	tests := []struct {
		name    string
		streams []ffprobe.Stream
		want    sql.NullInt64
	}{
		{"none", []ffprobe.Stream{audio}, sql.NullInt64{}},
		{"first without a role", []ffprobe.Stream{audio, picture(1, ""), picture(2, "Other")}, sql.NullInt64{Int64: 1, Valid: true}},
		{"any cover over no role", []ffprobe.Stream{audio, picture(1, "Artist"), picture(2, "Cover (back)")}, sql.NullInt64{Int64: 2, Valid: true}},
		{"front over back", []ffprobe.Stream{audio, picture(1, "Cover (back)"), picture(2, "Cover (front)")}, sql.NullInt64{Int64: 2, Valid: true}},
		{"a video that is not a picture", []ffprobe.Stream{audio, {Index: 1, CodecType: "video"}}, sql.NullInt64{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := embeddedPicture(tt.streams)
			if got != tt.want {
				t.Fatalf("embeddedPicture = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsDiscFolder(t *testing.T) {
	for name, want := range map[string]bool{
		"CD1": true, "cd 2": true, "Disc 1": true, "Disk_3": true, "Vol. 2": true, "Volume 10": true, "Part 1": true,
		"Disc": false, "CDs": false, "The River": false, "Disc One": false, "1": false,
	} {
		got := isDiscFolder(name)
		if got != want {
			t.Errorf("isDiscFolder(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestLocalArtCopiesTheBestNamedFolderImage(t *testing.T) {
	f := newArtFixture(t)
	f.track(t, "Band/Album/01.flac", "Album", picture(1, "Cover (front)"))
	f.image(t, "Band/Album/folder.png", pngBytes)
	f.image(t, "Band/Album/Cover.JPG", jpegBytes)
	f.image(t, "Band/Album/back.jpg", pngBytes)
	f.scan(t)

	art := f.album(t, "Album")
	wantPrefix := helpers.STATIC_URL_PREFIX + "local/albums/" + itoa(art.id) + "/cover.jpg?v="
	if !strings.HasPrefix(art.coverLocal.String, wantPrefix) {
		t.Fatalf("cover_local = %q, want prefix %q", art.coverLocal.String, wantPrefix)
	}
	if art.cover != art.coverLocal {
		t.Fatalf("cover = %v, want the local cover %v", art.cover, art.coverLocal)
	}
	if !bytes.Equal(f.copied(t, art), jpegBytes) {
		t.Fatal("the copy is not Cover.JPG")
	}
	if f.pictures.callCount() != 0 {
		t.Fatal("a folder image was found, yet a picture was extracted")
	}
	status := f.s.Status().Lookups
	if status.State != LookupIdle || status.Stage != nil || status.Total != 1 || status.Processed != 1 || status.Matched != 1 {
		t.Fatalf("lookups = %+v, want one matched album and idle", status)
	}
}

func TestLocalArtLooksInTheAlbumFolderThenItsDiscFolders(t *testing.T) {
	f := newArtFixture(t)
	f.track(t, "Floyd/The Wall/CD1/01.flac", "The Wall")
	f.track(t, "Floyd/The Wall/CD2/01.flac", "The Wall")
	f.image(t, "Floyd/The Wall/CD2/cover.jpg", pngBytes)
	f.image(t, "Floyd/The Wall/folder.jpg", jpegBytes)
	f.track(t, "Bruce/The River/Disc 1/01.flac", "The River")
	f.track(t, "Bruce/The River/Disc 2/01.flac", "The River")
	f.image(t, "Bruce/The River/Disc 1/cover.jpg", jpegBytes)
	f.scan(t)

	wall := f.album(t, "The Wall")
	if !bytes.Equal(f.copied(t, wall), jpegBytes) {
		t.Fatal("The Wall did not take the album folder's image over a disc folder's")
	}
	river := f.album(t, "The River")
	if !bytes.Equal(f.copied(t, river), jpegBytes) {
		t.Fatal("The River did not take its disc folder's image")
	}
}

func TestLocalArtSkipsTheImageOfAFolderSharedByAlbums(t *testing.T) {
	f := newArtFixture(t)
	f.track(t, "Singles/one.flac", "First Single")
	f.track(t, "Singles/two.flac", "Second Single", picture(1, ""))
	f.image(t, "Singles/cover.jpg", jpegBytes)
	f.scan(t)

	first := f.album(t, "First Single")
	if first.coverLocal.Valid {
		t.Fatalf("an album sharing its folder took its image: %v", first.coverLocal)
	}
	second := f.album(t, "Second Single")
	if !bytes.Equal(f.copied(t, second), pngBytes) {
		t.Fatal("the album sharing its folder did not fall back to its embedded picture")
	}
	if got := f.pictures.calls; len(got) != 1 || got[0] != "two.flac#1" {
		t.Fatalf("extracted %v, want two.flac#1", got)
	}
	status := f.s.Status().Lookups
	if status.Matched != 1 || status.Unmatched != 1 {
		t.Fatalf("lookups = %+v, want one matched and one unmatched", status)
	}
}

func TestLocalArtKeepsAnUnchangedCopyAndFollowsAChangedImage(t *testing.T) {
	f := newArtFixture(t)
	f.track(t, "Band/Album/01.flac", "Album")
	image := f.image(t, "Band/Album/cover.jpg", jpegBytes)
	f.scan(t)
	first := f.album(t, "Album")

	f.s.runLookups(context.Background(), lookupRequest{full: true})
	again := f.album(t, "Album")
	if again != first {
		t.Fatalf("an unchanged image changed the album: %+v, then %+v", first, again)
	}

	// A copy deleted by hand is made again.
	err := os.RemoveAll(filepath.Join(f.staticDir, "local"))
	if err != nil {
		t.Fatal(err)
	}
	f.s.runLookups(context.Background(), lookupRequest{full: true})
	if !bytes.Equal(f.copied(t, f.album(t, "Album")), jpegBytes) {
		t.Fatal("a deleted copy was not made again")
	}

	err = os.Remove(image)
	if err != nil {
		t.Fatal(err)
	}
	f.image(t, "Band/Album/cover.png", pngBytes)
	f.s.runLookups(context.Background(), lookupRequest{full: true})
	changed := f.album(t, "Album")
	if changed.coverLocal == first.coverLocal || !strings.Contains(changed.coverLocal.String, "/cover.png?v=") {
		t.Fatalf("cover_local = %v after the image changed, want a new png copy", changed.coverLocal)
	}
	entries, err := os.ReadDir(filepath.Join(f.staticDir, "local", "albums", itoa(changed.id)))
	if err != nil || len(entries) != 1 {
		t.Fatalf("the album folder holds %v (%v), want only the new copy", entries, err)
	}
}

func TestLocalArtGoesAwayWithItsSource(t *testing.T) {
	f := newArtFixture(t)
	f.track(t, "Band/Album/01.flac", "Album")
	image := f.image(t, "Band/Album/cover.jpg", jpegBytes)
	f.scan(t)
	art := f.album(t, "Album")
	_, err := f.s.tx.DB.Exec(`UPDATE albums SET cover_source = 'https://covers.example/a.jpg' WHERE id = ?`, art.id)
	if err != nil {
		t.Fatal(err)
	}
	if f.album(t, "Album").cover != art.coverLocal {
		t.Fatal("a provider cover replaced the album's own")
	}

	err = os.Remove(image)
	if err != nil {
		t.Fatal(err)
	}
	f.s.runLookups(context.Background(), lookupRequest{albums: map[int64]bool{art.id: true}})
	gone := f.album(t, "Album")
	if gone.coverLocal.Valid || gone.fingerprint.Valid || gone.cover.String != "https://covers.example/a.jpg" {
		t.Fatalf("album = %+v, want the provider cover back", gone)
	}
	_, err = os.Stat(filepath.Join(f.staticDir, "local", "albums", itoa(art.id)))
	if !os.IsNotExist(err) {
		t.Fatalf("the album's folder is still there: %v", err)
	}
}

func TestLocalArtFullPassPrunesFoldersNoAlbumPointsAt(t *testing.T) {
	f := newArtFixture(t)
	f.track(t, "Band/Album/01.flac", "Album")
	f.image(t, "Band/Album/cover.jpg", jpegBytes)
	f.scan(t)
	art := f.album(t, "Album")
	stray := scannertest.WriteFile(t, filepath.Join(f.staticDir, "local", "albums", "999", "cover.jpg"), "old")
	kept := scannertest.WriteFile(t, filepath.Join(f.staticDir, "local", "albums", "notes", "readme.txt"), "mine")

	f.s.runLookups(context.Background(), lookupRequest{albums: map[int64]bool{art.id: true}})
	_, err := os.Stat(stray)
	if err != nil {
		t.Fatalf("a pass of some albums pruned: %v", err)
	}

	f.s.runLookups(context.Background(), lookupRequest{full: true})
	_, err = os.Stat(stray)
	if !os.IsNotExist(err) {
		t.Fatalf("the folder of a deleted album survived a full pass: %v", err)
	}
	_, err = os.Stat(kept)
	if err != nil {
		t.Fatalf("a full pass removed a folder that is not an album's: %v", err)
	}
	f.copied(t, f.album(t, "Album"))
}

func TestLocalArtNeverWritesIntoALibrary(t *testing.T) {
	f := newArtFixture(t)
	f.staticDir = filepath.Join(f.musicDir, "static")
	f.track(t, "Band/Album/01.flac", "Album")
	f.image(t, "Band/Album/cover.jpg", jpegBytes)
	f.scan(t)

	if f.album(t, "Album").coverLocal.Valid {
		t.Fatal("album art was copied into the library")
	}
	_, err := os.Stat(filepath.Join(f.staticDir, "local"))
	if !os.IsNotExist(err) {
		t.Fatalf("the stage wrote into the library: %v", err)
	}
}

func TestLocalArtRefusesALinkedFolder(t *testing.T) {
	f := newArtFixture(t)
	f.track(t, "Band/Album/01.flac", "Album")
	f.image(t, "Band/Album/cover.jpg", jpegBytes)
	elsewhere := t.TempDir()
	err := os.Symlink(elsewhere, filepath.Join(f.staticDir, "local"))
	if err != nil {
		t.Fatal(err)
	}
	f.scan(t)

	if f.album(t, "Album").coverLocal.Valid {
		t.Fatal("album art was copied through a symlink")
	}
	if f.s.Status().Lookups.Failed != 1 {
		t.Fatalf("lookups = %+v, want the album failed", f.s.Status().Lookups)
	}
}

func TestLocalArtTriesTheNextSourceWhenOneFails(t *testing.T) {
	f := newArtFixture(t)
	f.track(t, "Band/Album/01.flac", "Album", picture(1, "Cover (front)"))
	f.image(t, "Band/Album/cover.jpg", []byte("not an image"))
	f.scan(t)

	if !bytes.Equal(f.copied(t, f.album(t, "Album")), pngBytes) {
		t.Fatal("an unreadable folder image did not fall back to the embedded picture")
	}
}

func TestLocalArtIsLeftAloneByTheArtworkSyncer(t *testing.T) {
	f := newArtFixture(t)
	f.track(t, "Band/Album/01.flac", "Album")
	f.image(t, "Band/Album/cover.jpg", jpegBytes)
	f.scan(t)
	art := f.album(t, "Album")
	q := f.s.queries
	ctx := context.Background()

	_, err := f.s.tx.DB.Exec(`UPDATE albums SET cover_source = 'https://covers.example/a.jpg', spotify_id = 'abc' WHERE id = ?`, art.id)
	if err != nil {
		t.Fatal(err)
	}
	n, err := q.SetAlbumCoverArtwork(ctx, database.SetAlbumCoverArtworkParams{
		ID: art.id, Value: helpers.NullString("/api/static/albums/abc/album_cover.jpg?v=1"), Source: helpers.NullString("https://covers.example/a.jpg"), Key: "abc",
	})
	if err != nil || n != 0 {
		t.Fatalf("the syncer pointed an album with art of its own at a download: %d rows, %v", n, err)
	}
	_, err = q.RevertAlbumCoverArtwork(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, list := range []func() (int, error){
		func() (int, error) {
			rows, err := q.ListAlbumArtwork(ctx, database.ListAlbumArtworkParams{PageSize: 10})
			return len(rows), err
		},
		func() (int, error) {
			rows, err := q.ListPendingAlbumArtwork(ctx, database.ListPendingAlbumArtworkParams{PageSize: 10})
			return len(rows), err
		},
		func() (int, error) {
			rows, err := q.ListAlbumArtworkInFolders(ctx, `["abc"]`)
			return len(rows), err
		},
	} {
		n, err := list()
		if err != nil || n != 0 {
			t.Fatalf("the syncer lists an album with art of its own: %d rows, %v", n, err)
		}
	}
	if f.album(t, "Album").cover != art.coverLocal {
		t.Fatal("the album's own cover did not survive the syncer's writes")
	}
}

func TestLocalArtStageStoppedByCancelRecordsNothing(t *testing.T) {
	f := newArtFixture(t)
	f.track(t, "Band/Album/01.flac", "Album", picture(1, ""))
	f.s.staticDir = func() string { return "" }
	f.scan(t)
	art := f.album(t, "Album")
	f.s.staticDir = func() string { return f.staticDir }

	ctx, cancel := context.WithCancel(context.Background())
	f.pictures.hook = cancel
	rest := f.s.runLookups(ctx, lookupRequest{albums: map[int64]bool{art.id: true}})
	if f.album(t, "Album").coverLocal.Valid {
		t.Fatal("a canceled pass recorded a cover")
	}
	if !rest.albums[art.id] {
		t.Fatalf("a canceled pass handed back %+v, want the album it stopped on", rest)
	}
}

func TestLocalArtCopiesIntoANewStaticDirectory(t *testing.T) {
	f := newArtFixture(t)
	f.track(t, "Band/Album/01.flac", "Album")
	f.image(t, "Band/Album/cover.jpg", jpegBytes)
	f.scan(t)

	f.staticDir = t.TempDir()
	f.s.RequestLookups()
	f.wait(t)
	if !bytes.Equal(f.copied(t, f.album(t, "Album")), jpegBytes) {
		t.Fatal("the copy was not made in the new static directory")
	}
}

func itoa(id int64) string {
	return strconv.FormatInt(id, 10)
}

func TestLocalArtKeepsTheCopyWhileTheFolderCannotBeRead(t *testing.T) {
	for _, hide := range []string{"unreadable", "missing"} {
		t.Run(hide, func(t *testing.T) {
			f := newArtFixture(t)
			f.track(t, "Band/Album/01.flac", "Album")
			f.image(t, "Band/Album/cover.jpg", jpegBytes)
			f.scan(t)
			before := f.album(t, "Album")

			folder := filepath.Join(f.musicDir, "Band", "Album")
			if hide == "unreadable" {
				err := os.Chmod(folder, 0)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(folder, 0o755) })
			} else {
				err := os.Rename(filepath.Join(f.musicDir, "Band"), filepath.Join(f.musicDir, "Elsewhere"))
				if err != nil {
					t.Fatal(err)
				}
			}
			f.s.runLookups(context.Background(), lookupRequest{full: true})
			after := f.album(t, "Album")
			if after != before {
				t.Fatalf("album = %+v with its folder %s, want the copy kept: %+v", after, hide, before)
			}
			if f.s.Status().Lookups.Failed != 1 {
				t.Fatalf("lookups = %+v, want the album failed", f.s.Status().Lookups)
			}
		})
	}
}

func TestLocalArtCopiesAgainInAStaticDirectoryUsedBefore(t *testing.T) {
	f := newArtFixture(t)
	f.track(t, "Band/Album/01.flac", "Album")
	image := f.image(t, "Band/Album/cover.jpg", jpegBytes)
	f.scan(t)
	first := f.staticDir

	f.staticDir = t.TempDir()
	f.s.runLookups(context.Background(), lookupRequest{full: true})
	changed := append(append([]byte{}, jpegBytes...), []byte(" changed")...)
	err := os.WriteFile(image, changed, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Minute)
	err = os.Chtimes(image, future, future)
	if err != nil {
		t.Fatal(err)
	}
	f.s.runLookups(context.Background(), lookupRequest{full: true})

	f.staticDir = first
	f.s.runLookups(context.Background(), lookupRequest{full: true})
	if !bytes.Equal(f.copied(t, f.album(t, "Album")), changed) {
		t.Fatal("the old directory's stale copy was taken for current")
	}
}

func TestLocalArtDoesNotCopyAFallbackAgainEachPass(t *testing.T) {
	f := newArtFixture(t)
	f.track(t, "Band/Album/01.flac", "Album", picture(1, "Cover (front)"))
	f.image(t, "Band/Album/cover.jpg", []byte("not an image"))
	f.scan(t)
	f.s.runLookups(context.Background(), lookupRequest{full: true})
	if f.pictures.callCount() != 1 {
		t.Fatalf("extracted %d times, want once: an unchanged fallback is kept", f.pictures.callCount())
	}
}

func TestLocalArtStopsWhenTheLibraryRootsChange(t *testing.T) {
	f := newArtFixture(t)
	f.track(t, "One/First/01.flac", "First", picture(1, ""))
	f.track(t, "Two/Second/01.flac", "Second", picture(1, ""))
	f.s.staticDir = func() string { return "" }
	f.scan(t)
	f.s.staticDir = func() string { return f.staticDir }

	var mu sync.Mutex
	roots := []string{f.musicDir}
	f.s.libraryDirs = func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(roots)
	}
	f.pictures.hook = func() {
		mu.Lock()
		roots = append(roots, filepath.Join(f.staticDir, "local"))
		mu.Unlock()
	}
	rest := f.s.runLookups(context.Background(), lookupRequest{full: true})
	if !rest.full {
		t.Fatalf("a pass stopped by new roots handed back %+v, want a full pass", rest)
	}
	if f.album(t, "First").coverLocal.Valid || f.album(t, "Second").coverLocal.Valid {
		t.Fatal("a pass wrote after the library roots changed")
	}
}

func TestLocalArtTreatsALoneDiscLikeFolderAsTheAlbum(t *testing.T) {
	f := newArtFixture(t)
	f.track(t, "Sabbath/Vol. 4/01.flac", "Vol. 4")
	f.image(t, "Sabbath/Vol. 4/cover.jpg", jpegBytes)
	f.image(t, "Sabbath/folder.jpg", pngBytes)
	f.scan(t)
	if !bytes.Equal(f.copied(t, f.album(t, "Vol. 4")), jpegBytes) {
		t.Fatal("an album named like a disc took its artist folder's image")
	}
}

func TestLocalArtSkippedStageReadsNoAlbum(t *testing.T) {
	f := newArtFixture(t)
	f.track(t, "Band/Album/01.flac", "Album")
	f.s.staticDir = func() string { return "" }
	f.scan(t)
	status := f.s.Status().Lookups
	if status.Total != 0 || status.Processed != 0 {
		t.Fatalf("lookups = %+v, want no album read by a stage that cannot run", status)
	}
}

func TestLookupsOfSomeAlbumsAddTheirFolderNeighbors(t *testing.T) {
	f := newArtFixture(t)
	f.track(t, "Band/Album/01.flac", "Album")
	f.track(t, "Band/Album/02.flac", "Stray")
	f.track(t, "Band/Other/01.flac", "Other")
	f.track(t, "Multi/Set/CD1/01.flac", "Set")
	f.track(t, "Multi/Set/CD2/01.flac", "Set Bonus")
	f.scan(t)

	ids, err := f.s.withNeighborAlbums(context.Background(), []int64{f.album(t, "Stray").id, f.album(t, "Set Bonus").id})
	if err != nil {
		t.Fatal(err)
	}
	want := []int64{f.album(t, "Album").id, f.album(t, "Stray").id, f.album(t, "Set").id, f.album(t, "Set Bonus").id}
	slices.Sort(want)
	if !slices.Equal(ids, want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
}
