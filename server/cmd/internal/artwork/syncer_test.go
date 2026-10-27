package artwork

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"igloo/cmd/internal/scanner/scannertest"

	_ "github.com/mattn/go-sqlite3"
)

const pngSignature = "\x89PNG\r\n\x1a\n"

// upstream stands in for the TMDB image CDN: it answers every path with a PNG
// unless a test sets a status or handler for it, and counts requests by path.
type upstream struct {
	mu       sync.Mutex
	hits     map[string]int
	statuses map[string]int
	bodies   map[string]string
	handlers map[string]http.HandlerFunc
}

func (u *upstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u.mu.Lock()
	u.hits[r.URL.Path]++
	status, hasStatus := u.statuses[r.URL.Path]
	body, hasBody := u.bodies[r.URL.Path]
	handler := u.handlers[r.URL.Path]
	u.mu.Unlock()

	if handler != nil {
		handler(w, r)
		return
	}
	if hasStatus {
		w.WriteHeader(status)
		return
	}
	if !hasBody {
		body = pngSignature + r.URL.Path
	}
	w.Write([]byte(body))
}

func (u *upstream) requests(path string) int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.hits[path]
}

func (u *upstream) total() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	total := 0
	for _, n := range u.hits {
		total += n
	}
	return total
}

func (u *upstream) set(path string, status int) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.statuses[path] = status
}

type harness struct {
	t         *testing.T
	db        *sql.DB
	logger    *scannertest.Logger
	upstream  *upstream
	syncer    *Syncer
	static    string
	enabled   bool
	libraries []string
	now       time.Time
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	db, queries := scannertest.OpenDB(t, ":memory:?_foreign_keys=on")
	h := &harness{
		t:      t,
		db:     db,
		logger: &scannertest.Logger{},
		upstream: &upstream{
			hits: map[string]int{}, statuses: map[string]int{},
			bodies: map[string]string{}, handlers: map[string]http.HandlerFunc{},
		},
		static:  t.TempDir(),
		enabled: true,
		now:     time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC),
	}
	server := httptest.NewServer(h.upstream)
	t.Cleanup(server.Close)

	ctx, cancel := context.WithCancel(context.Background())
	wait := &sync.WaitGroup{}
	t.Cleanup(func() {
		cancel()
		wait.Wait()
	})

	h.syncer = New(Dependencies{
		DB:               db,
		Queries:          queries,
		Logger:           h.logger,
		Context:          ctx,
		Wait:             wait,
		Enabled:          func() bool { return h.enabled },
		StaticDir:        func() string { return h.static },
		LibraryDirs:      func() []string { return h.libraries },
		TmdbImageBaseURL: func() string { return server.URL },
		HTTPClient:       server.Client(),
		Now:              func() time.Time { return h.now },
	})

	return h
}

func (h *harness) exec(query string, args ...any) {
	h.t.Helper()
	_, err := h.db.Exec(query, args...)
	if err != nil {
		h.t.Fatalf("%s: %v", query, err)
	}
}

func (h *harness) addMovie(id, tmdbID int64, title, poster, backdrop string) {
	h.t.Helper()
	h.exec(`INSERT INTO movies (id, title, file_path, file_name, size, container, mime_type, adult, tmdb_id, poster_path_source, backdrop_path_source)
		VALUES (?, ?, ?, 'movie.mkv', 1, 'mkv', 'video/x-matroska', false, ?, NULLIF(?, ''), NULLIF(?, ''))`,
		id, title, "/movies/"+title+"-"+string(rune('a'+id))+".mkv", tmdbID, poster, backdrop)
}

func (h *harness) addPerson(id, tmdbID int64, name, profile string) {
	h.t.Helper()
	h.exec(`INSERT INTO artist (id, name, tmdb_id, profile_source) VALUES (?, ?, ?, NULLIF(?, ''))`, id, name, tmdbID, profile)
}

func (h *harness) value(table, column string, id int64) sql.NullString {
	h.t.Helper()
	var value sql.NullString
	err := h.db.QueryRow("SELECT "+column+" FROM "+table+" WHERE id = ?", id).Scan(&value)
	if err != nil {
		h.t.Fatalf("read %s.%s: %v", table, column, err)
	}
	return value
}

func (h *harness) pass() {
	h.t.Helper()
	h.syncer.pass(context.Background())
}

func (h *harness) exists(rel string) bool {
	h.t.Helper()
	_, err := os.Stat(filepath.Join(h.static, filepath.FromSlash(rel)))
	return err == nil
}

func (h *harness) write(rel, contents string) {
	h.t.Helper()
	path := filepath.Join(h.static, filepath.FromSlash(rel))
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		h.t.Fatal(err)
	}
	err = os.WriteFile(path, []byte(contents), 0o644)
	if err != nil {
		h.t.Fatal(err)
	}
}

func local(rel, size, source string) string {
	return localURL(rel, version(size, source))
}

func TestPass_DownloadsMovieAndPersonArtwork(t *testing.T) {
	h := newHarness(t)
	h.addMovie(1, 603, "The Matrix", "/matrix-poster.jpg", "/matrix-backdrop.jpg")
	h.addPerson(1, 6384, "Keanu Reeves", "/keanu.jpg")
	h.addPerson(2, 7777, "No Photo", "")

	h.pass()

	want := map[[3]any]string{
		{"movies", "poster_path", int64(1)}:   local("movies/603/the_matrix_poster.png", "w500", "/matrix-poster.jpg"),
		{"movies", "backdrop_path", int64(1)}: local("movies/603/the_matrix_backdrop.png", "w1280", "/matrix-backdrop.jpg"),
		{"artist", "profile", int64(1)}:       local("people/6384/keanu_reeves_profile.png", "w185", "/keanu.jpg"),
	}
	for key, url := range want {
		got := h.value(key[0].(string), key[1].(string), key[2].(int64))
		if got.String != url {
			t.Errorf("%s.%s = %q, want %q", key[0], key[1], got.String, url)
		}
	}
	for _, rel := range []string{"movies/603/the_matrix_poster.png", "movies/603/the_matrix_backdrop.png", "people/6384/keanu_reeves_profile.png"} {
		if !h.exists(rel) {
			t.Errorf("%s was not saved", rel)
		}
	}
	if h.upstream.requests("/w500/matrix-poster.jpg") != 1 || h.upstream.requests("/w1280/matrix-backdrop.jpg") != 1 || h.upstream.requests("/w185/keanu.jpg") != 1 {
		t.Fatalf("each image should be requested once at the size the clients show, got %v", h.upstream.hits)
	}
	if h.value("artist", "profile", 2).Valid {
		t.Error("a person without a photo got one")
	}

	h.pass()
	if h.upstream.total() != 3 {
		t.Fatalf("a second pass made %d requests, want none", h.upstream.total()-3)
	}
	if h.value("movies", "poster_path", 1).String != want[[3]any{"movies", "poster_path", int64(1)}] {
		t.Fatal("a second pass changed a current copy")
	}
}

func TestPass_SharesOneCopyBetweenFilesOfTheSameMovie(t *testing.T) {
	h := newHarness(t)
	h.addMovie(1, 603, "The Matrix", "/matrix-poster.jpg", "")
	h.addMovie(2, 603, "The Matrix", "/matrix-poster.jpg", "")

	h.pass()

	if h.upstream.requests("/w500/matrix-poster.jpg") != 1 {
		t.Fatalf("poster requested %d times, want once", h.upstream.requests("/w500/matrix-poster.jpg"))
	}
	first, second := h.value("movies", "poster_path", 1), h.value("movies", "poster_path", 2)
	want := local("movies/603/the_matrix_poster.png", "w500", "/matrix-poster.jpg")
	if first.String != want || second.String != want {
		t.Fatalf("posters = %q and %q, want both %q", first.String, second.String, want)
	}

	// A third copy of the movie, matched later, reuses the file.
	h.addMovie(3, 603, "The Matrix", "/matrix-poster.jpg", "")
	h.pass()
	if h.upstream.requests("/w500/matrix-poster.jpg") != 1 || h.value("movies", "poster_path", 3).String != want {
		t.Fatalf("a later copy of the movie was not pointed at the existing file: %q", h.value("movies", "poster_path", 3).String)
	}
}

func TestPass_FollowsANewSourceOrTitle(t *testing.T) {
	h := newHarness(t)
	h.addMovie(1, 603, "The Matrix", "/matrix-poster.jpg", "")
	h.pass()

	// A new match or an Identify writes a new source, which the trigger
	// returns until the pass downloads it under the same name.
	h.exec(`UPDATE movies SET poster_path_source = '/matrix-poster-2.jpg' WHERE id = 1`)
	if h.value("movies", "poster_path", 1).String != "/matrix-poster-2.jpg" {
		t.Fatal("a new source did not replace the copy")
	}
	h.pass()
	want := local("movies/603/the_matrix_poster.png", "w500", "/matrix-poster-2.jpg")
	if h.value("movies", "poster_path", 1).String != want {
		t.Fatalf("poster = %q, want %q", h.value("movies", "poster_path", 1).String, want)
	}
	saved, err := os.ReadFile(filepath.Join(h.static, "movies", "603", "the_matrix_poster.png"))
	if err != nil || !strings.HasSuffix(string(saved), "/w500/matrix-poster-2.jpg") {
		t.Fatalf("the file holds %q, want the new poster", saved)
	}

	// A new title names a new file and retires the old one.
	h.exec(`UPDATE movies SET title = 'The Matrix Reloaded' WHERE id = 1`)
	h.pass()
	want = local("movies/603/the_matrix_reloaded_poster.png", "w500", "/matrix-poster-2.jpg")
	if h.value("movies", "poster_path", 1).String != want {
		t.Fatalf("poster after a rename = %q, want %q", h.value("movies", "poster_path", 1).String, want)
	}
	if h.exists("movies/603/the_matrix_poster.png") {
		t.Fatal("the file of the old title was not removed")
	}
}

func TestPass_HealsAMissingCopy(t *testing.T) {
	h := newHarness(t)
	h.addMovie(1, 603, "The Matrix", "/matrix-poster.jpg", "/matrix-backdrop.jpg")
	h.pass()

	err := os.Remove(filepath.Join(h.static, "movies", "603", "the_matrix_poster.png"))
	if err != nil {
		t.Fatal(err)
	}
	h.pass()

	if !h.exists("movies/603/the_matrix_poster.png") || h.upstream.requests("/w500/matrix-poster.jpg") != 2 {
		t.Fatal("a deleted copy was not downloaded again")
	}
	if h.value("movies", "poster_path", 1).String != local("movies/603/the_matrix_poster.png", "w500", "/matrix-poster.jpg") {
		t.Fatalf("poster = %q after healing", h.value("movies", "poster_path", 1).String)
	}
}

func TestPass_LeavesAStaticDirectoryThatLostEveryCopyAlone(t *testing.T) {
	h := newHarness(t)
	h.addMovie(1, 603, "The Matrix", "/matrix-poster.jpg", "/matrix-backdrop.jpg")
	h.pass()
	copyURL := h.value("movies", "poster_path", 1).String

	// What an unmounted disk looks like: the same directory, empty.
	err := os.RemoveAll(filepath.Join(h.static, "movies"))
	if err != nil {
		t.Fatal(err)
	}
	h.pass()

	if h.upstream.total() != 2 {
		t.Fatalf("made %d requests into what looks like an unmounted directory", h.upstream.total()-2)
	}
	if h.value("movies", "poster_path", 1).String != copyURL {
		t.Fatal("pointed a column away from its copy while the directory looked unmounted")
	}
	if !h.logger.WarnMentions("skipping artwork pass: no downloaded artwork is left in the static directory; is it mounted?", h.static) {
		t.Fatal("did not warn about the empty static directory")
	}

	// Another static directory is a different matter: its copies are made.
	h.static = t.TempDir()
	h.pass()
	if !h.exists("movies/603/the_matrix_poster.png") || h.value("movies", "poster_path", 1).String != copyURL {
		t.Fatal("a new static directory was not filled")
	}
}

func TestPass_RemembersARefusedImageForADay(t *testing.T) {
	h := newHarness(t)
	h.addMovie(1, 603, "The Matrix", "/missing.jpg", "")
	h.upstream.set("/w500/missing.jpg", http.StatusNotFound)

	h.pass()
	h.pass()
	if h.upstream.requests("/w500/missing.jpg") != 1 {
		t.Fatalf("a refused image was requested %d times within a day", h.upstream.requests("/w500/missing.jpg"))
	}
	if h.value("movies", "poster_path", 1).String != "/missing.jpg" {
		t.Fatal("a refused image did not stay remote")
	}

	h.now = h.now.Add(failureRetryAfter + time.Minute)
	h.pass()
	if h.upstream.requests("/w500/missing.jpg") != 2 {
		t.Fatal("a refused image was not requested again after a day")
	}
}

func TestPass_KeepsImagesItWillNotStoreRemote(t *testing.T) {
	h := newHarness(t)
	h.addMovie(1, 603, "The Matrix", "/poster.svg", "/backdrop.jpg")
	h.upstream.bodies["/w1280/backdrop.jpg"] = "<!DOCTYPE html><html>not an image</html>"

	h.pass()
	h.pass()

	if h.upstream.requests("/w500/poster.svg") != 0 {
		t.Fatal("an SVG source was requested")
	}
	if h.upstream.requests("/w1280/backdrop.jpg") != 1 {
		t.Fatalf("a non-image body was requested %d times, want once a day", h.upstream.requests("/w1280/backdrop.jpg"))
	}
	if h.value("movies", "poster_path", 1).String != "/poster.svg" || h.value("movies", "backdrop_path", 1).String != "/backdrop.jpg" {
		t.Fatal("images that were not stored did not stay remote")
	}
	if h.exists("movies/603") {
		t.Fatal("left a folder behind for images that were not stored")
	}
}

func TestPass_StopsAfterRepeatedProviderFailuresAndRetriesLater(t *testing.T) {
	h := newHarness(t)
	for id := int64(1); id <= 20; id++ {
		poster := "/poster-" + string(rune('a'+id)) + ".jpg"
		h.addMovie(id, 600+id, "Movie", poster, "")
		h.upstream.set("/w500"+poster, http.StatusBadGateway)
	}

	h.pass()

	failed := h.upstream.total()
	if failed >= 20 || failed < 3 {
		t.Fatalf("made %d requests against a failing provider, want the pass to stop after a few", failed)
	}
	if !h.logger.WarnMentions("stopped downloading artwork after repeated failures; the rest is retried on the next pass", "") {
		t.Fatal("did not log that downloads stopped")
	}

	// Server errors are not remembered: the next pass tries again.
	h.pass()
	if h.upstream.total() <= failed {
		t.Fatal("the next pass did not retry after server errors")
	}
}

func TestPass_ANewSourceDuringTheDownloadWins(t *testing.T) {
	h := newHarness(t)
	h.addMovie(1, 603, "The Matrix", "/old.jpg", "")
	h.upstream.handlers["/w500/old.jpg"] = func(w http.ResponseWriter, r *http.Request) {
		// A scan matches the movie again while the old poster downloads.
		_, err := h.db.Exec(`UPDATE movies SET poster_path_source = '/new.jpg' WHERE id = 1`)
		if err != nil {
			t.Error(err)
		}
		w.Write([]byte(pngSignature + "old"))
	}

	h.pass()

	if h.value("movies", "poster_path", 1).String != "/new.jpg" {
		t.Fatalf("poster = %q, want the source the scan wrote", h.value("movies", "poster_path", 1).String)
	}
	if h.exists("movies/603/the_matrix_poster.png") {
		t.Fatal("kept the copy of a source the row no longer has")
	}

	h.pass()
	if h.value("movies", "poster_path", 1).String != local("movies/603/the_matrix_poster.png", "w500", "/new.jpg") {
		t.Fatal("the next pass did not download the new source")
	}
}

func TestPass_PrunesOnlyItsOwnFiles(t *testing.T) {
	h := newHarness(t)
	h.addMovie(1, 603, "The Matrix", "/matrix-poster.jpg", "")
	foreign := []string{
		"movies/603/notes.txt",
		"movies/603/Poster.JPG",
		"movies/not a key/old_title_poster.jpg",
		"movies/readme.jpg",
		"avatars/1-1700000000.jpg",
		"albums/123/daylight_cover.jpg",
	}
	for _, rel := range foreign {
		h.write(rel, "keep")
	}
	orphans := []string{"movies/999/old_title_poster.jpg", "movies/603/.tmp-123", "people/42/someone_profile.png"}
	for _, rel := range orphans {
		h.write(rel, "orphan")
	}

	h.pass()

	for _, rel := range foreign {
		if !h.exists(rel) {
			t.Errorf("removed %s, which the syncer did not write", rel)
		}
	}
	for _, rel := range orphans {
		if h.exists(rel) {
			t.Errorf("kept %s, which nothing points at", rel)
		}
	}
	if h.exists("movies/999") || h.exists("people/42") {
		t.Error("kept a folder emptied by pruning")
	}
	if !h.exists("movies/603/the_matrix_poster.png") {
		t.Error("removed a copy a column points at")
	}
}

func TestPass_TurningOffPointsBackAtTheProviderAndDeletesTheCopies(t *testing.T) {
	h := newHarness(t)
	h.addMovie(1, 603, "The Matrix", "/matrix-poster.jpg", "/matrix-backdrop.jpg")
	h.addPerson(1, 6384, "Keanu Reeves", "/keanu.jpg")
	h.write("avatars/1-1700000000.jpg", "avatar")
	h.write("movies/603/notes.txt", "keep")
	h.pass()

	h.enabled = false
	h.pass()

	if h.value("movies", "poster_path", 1).String != "/matrix-poster.jpg" ||
		h.value("movies", "backdrop_path", 1).String != "/matrix-backdrop.jpg" ||
		h.value("artist", "profile", 1).String != "/keanu.jpg" {
		t.Fatal("columns were not pointed back at their sources")
	}
	for _, rel := range []string{"movies/603/the_matrix_poster.png", "movies/603/the_matrix_backdrop.png", "people"} {
		if h.exists(rel) {
			t.Errorf("%s is still there", rel)
		}
	}
	if !h.exists("avatars/1-1700000000.jpg") || !h.exists("movies/603/notes.txt") {
		t.Fatal("removed files the syncer did not write")
	}

	// Turning it on again downloads everything again.
	h.enabled = true
	h.pass()
	if !h.exists("movies/603/the_matrix_poster.png") || !h.exists("people/6384/keanu_reeves_profile.png") {
		t.Fatal("turning the setting back on did not download the artwork")
	}
}

func TestPass_NeverManagesAFolderThatOverlapsALibrary(t *testing.T) {
	h := newHarness(t)
	h.addMovie(1, 603, "The Matrix", "/matrix-poster.jpg", "")
	h.addPerson(1, 6384, "Keanu Reeves", "/keanu.jpg")
	// The static directory's movies folder is the movie library, holding a
	// file that looks like a copy.
	h.libraries = []string{filepath.Join(h.static, "movies")}
	h.write("movies/1999/a_poster.jpg", "the library's own")

	h.pass()

	if h.upstream.requests("/w500/matrix-poster.jpg") != 0 || h.value("movies", "poster_path", 1).String != "/matrix-poster.jpg" {
		t.Fatal("downloaded into a library folder")
	}
	if !h.exists("people/6384/keanu_reeves_profile.png") {
		t.Fatal("a folder that does not overlap was skipped too")
	}

	h.enabled = false
	h.pass()
	if !h.exists("movies/1999/a_poster.jpg") {
		t.Fatal("deleted a file from a library folder")
	}
}

func TestPass_CancellationLeavesFilesForTheNextPass(t *testing.T) {
	h := newHarness(t)
	h.addMovie(1, 603, "The Matrix", "/matrix-poster.jpg", "")
	h.write("movies/999/old_title_poster.jpg", "orphan")
	ctx, cancel := context.WithCancel(context.Background())
	h.upstream.handlers["/w500/matrix-poster.jpg"] = func(w http.ResponseWriter, r *http.Request) {
		cancel()
		<-r.Context().Done()
	}

	h.syncer.pass(ctx)

	if h.value("movies", "poster_path", 1).String != "/matrix-poster.jpg" {
		t.Fatal("a canceled download changed the column")
	}
	if !h.exists("movies/999/old_title_poster.jpg") {
		t.Fatal("pruned during a canceled pass")
	}
}

func TestRequest_RunsOnePassForTheRequestsMadeDuringAPass(t *testing.T) {
	h := newHarness(t)
	h.addMovie(1, 603, "The Matrix", "/matrix-poster.jpg", "")
	entered := make(chan struct{})
	release := make(chan struct{})
	h.upstream.handlers["/w500/matrix-poster.jpg"] = func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		w.Write([]byte(pngSignature))
	}

	h.syncer.Request()
	scannertest.WaitForSignal(t, entered, 5*time.Second, "the first pass to download")
	h.syncer.Request()
	h.syncer.Request()
	h.syncer.Request()
	close(release)
	scannertest.WaitForGroup(t, h.syncer.wait, 5*time.Second, "the passes to finish")

	passes := 0
	for _, entries := range [][]scannertest.LogEntry{h.logger.InfoEntries, h.logger.DebugEntries} {
		for _, entry := range entries {
			if entry.Msg == "artwork pass finished" {
				passes++
			}
		}
	}
	if passes != 2 {
		t.Fatalf("ran %d passes, want the first and one for the requests made during it", passes)
	}
}

// A managed folder that leads into a library through a symlink is that
// library: the syncer neither downloads into it nor prunes it, with the
// setting on or off, whether the folder or the static directory is the link.
func TestPass_NeverManagesAFolderThatLinksIntoALibrary(t *testing.T) {
	for _, link := range []string{"managed folder", "static directory"} {
		t.Run(link, func(t *testing.T) {
			h := newHarness(t)
			h.addMovie(1, 603, "The Matrix", "/matrix-poster.jpg", "")
			parent := t.TempDir()
			library := filepath.Join(parent, "movies")
			libraryPoster := filepath.Join(library, "603", "my_movie_poster.jpg")
			err := os.MkdirAll(filepath.Dir(libraryPoster), 0o755)
			if err != nil {
				t.Fatal(err)
			}
			err = os.WriteFile(libraryPoster, []byte("the library's own"), 0o644)
			if err != nil {
				t.Fatal(err)
			}
			h.libraries = []string{library}
			if link == "managed folder" {
				err = os.Symlink(library, filepath.Join(h.static, "movies"))
			} else {
				h.static = filepath.Join(t.TempDir(), "static")
				err = os.Symlink(parent, h.static)
			}
			if err != nil {
				t.Fatal(err)
			}

			h.enabled = false
			h.pass()
			h.enabled = true
			h.pass()

			_, err = os.Stat(libraryPoster)
			if err != nil {
				t.Fatalf("the library's file is gone: %v", err)
			}
			if h.upstream.requests("/w500/matrix-poster.jpg") != 0 || h.value("movies", "poster_path", 1).String != "/matrix-poster.jpg" {
				t.Fatal("downloaded into a folder that links into a library")
			}
		})
	}
}

// The pass stops after three failures in a row, not three in all: a success
// in between starts the count again.
func TestPass_ASuccessResetsTheProviderFailureCount(t *testing.T) {
	h := newHarness(t)
	for id := int64(1); id <= 40; id++ {
		poster := fmt.Sprintf("/poster-%d.jpg", id)
		h.addMovie(id, 600+id, "Movie", poster, "")
		if id%10 == 0 {
			h.upstream.set("/w500"+poster, http.StatusBadGateway)
		}
	}

	h.pass()

	if h.upstream.total() != 40 {
		t.Fatalf("requested %d of 40 images, want every one", h.upstream.total())
	}
	downloaded := scannertest.CountRows(t, h.db, `SELECT count(*) FROM movies WHERE poster_path LIKE '/api/static/%'`)
	if downloaded != 36 {
		t.Fatalf("downloaded %d posters, want the 36 the provider served", downloaded)
	}
}

// Turning downloads off records the static directory as clean only once every
// copy is gone, so a folder it could not empty is tried again on later passes.
func TestPass_TurningOffRetriesAFolderItCouldNotEmpty(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	h := newHarness(t)
	h.addMovie(1, 603, "The Matrix", "/matrix-poster.jpg", "")
	h.pass()
	folder := filepath.Join(h.static, "movies", "603")
	err := os.Chmod(folder, 0o555)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(folder, 0o755) })

	h.enabled = false
	h.pass()
	if !h.exists("movies/603/the_matrix_poster.png") {
		t.Fatal("removed a copy from a folder it cannot write to")
	}

	err = os.Chmod(folder, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	h.pass()
	if h.exists("movies/603/the_matrix_poster.png") {
		t.Fatal("a later pass did not remove the copy once it could")
	}
}

func (h *harness) addShow(id int64, tmdbID any, name, poster, backdrop string) {
	h.t.Helper()
	h.exec(`INSERT INTO shows (id, directory_path, local_name, name, tmdb_id, poster_path_source, backdrop_path_source)
		VALUES (?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''))`, id, fmt.Sprintf("/shows/%s-%d", name, id), name, name, tmdbID, poster, backdrop)
}

func TestPass_DownloadsShowSeasonEpisodeAndNetworkArtwork(t *testing.T) {
	h := newHarness(t)
	h.addShow(1, 1438, "The Wire", "/wire-poster.jpg", "/wire-backdrop.jpg")
	h.exec(`INSERT INTO show_seasons (id, show_id, season_number, name, poster_path_source) VALUES (1, 1, 0, 'Specials', '/specials.jpg'), (2, 1, 1, 'Season 1', '/season-1.jpg')`)
	h.exec(`INSERT INTO show_episodes (id, season_id, episode_number, name, still_path_source) VALUES (1, 2, 1, 'The Target', '/pilot.jpg')`)
	h.exec(`INSERT INTO networks (id, tmdb_id, name, logo_source) VALUES (1, 49, 'HBO', '/hbo.png')`)
	// A show with no TMDB match has no folder, so neither do its seasons.
	h.addShow(2, nil, "Home Movies", "/home-poster.jpg", "")
	h.exec(`INSERT INTO show_seasons (id, show_id, season_number, name, poster_path_source) VALUES (3, 2, 1, 'Season 1', '/home-season.jpg')`)

	h.pass()

	want := []struct {
		table, column string
		id            int64
		rel, size     string
		source        string
	}{
		{"shows", "poster_path", 1, "shows/1438/the_wire_poster.png", "w500", "/wire-poster.jpg"},
		{"shows", "backdrop_path", 1, "shows/1438/the_wire_backdrop.png", "w1280", "/wire-backdrop.jpg"},
		{"show_seasons", "poster_path", 1, "shows/1438/season_00_poster.png", "w500", "/specials.jpg"},
		{"show_seasons", "poster_path", 2, "shows/1438/season_01_poster.png", "w500", "/season-1.jpg"},
		{"show_episodes", "still_path", 1, "shows/1438/s01e01_still.png", "w500", "/pilot.jpg"},
		{"networks", "logo", 1, "networks/49/hbo_logo.png", "w92", "/hbo.png"},
	}
	for _, w := range want {
		got := h.value(w.table, w.column, w.id).String
		if got != local(w.rel, w.size, w.source) {
			t.Errorf("%s.%s %d = %q, want the copy at %s", w.table, w.column, w.id, got, w.rel)
		}
		if h.upstream.requests("/"+w.size+w.source) != 1 {
			t.Errorf("%s was not requested once at %s", w.source, w.size)
		}
	}
	if h.value("shows", "poster_path", 2).String != "/home-poster.jpg" || h.value("show_seasons", "poster_path", 3).String != "/home-season.jpg" {
		t.Error("an unmatched show's images did not stay remote")
	}

	h.enabled = false
	h.pass()
	for _, w := range want {
		if h.value(w.table, w.column, w.id).String != w.source {
			t.Errorf("%s.%s %d was not pointed back at its source", w.table, w.column, w.id)
		}
	}
	if h.exists("shows") || h.exists("networks") {
		t.Error("turning downloads off left the show or network folders")
	}
}

// A show named like one of its seasons would give two images one file. The
// first listed, the show's own poster, keeps the name; the season's stays
// remote rather than overwrite it.
func TestPass_KeepsTwoImagesOfOneNameApart(t *testing.T) {
	h := newHarness(t)
	h.addShow(1, 1438, "Season 01", "/show-poster.jpg", "")
	h.exec(`INSERT INTO show_seasons (id, show_id, season_number, name, poster_path_source) VALUES (1, 1, 1, 'Season 1', '/season-poster.jpg')`)

	h.pass()
	h.pass()

	if h.value("shows", "poster_path", 1).String != local("shows/1438/season_01_poster.png", "w500", "/show-poster.jpg") {
		t.Fatalf("show poster = %q", h.value("shows", "poster_path", 1).String)
	}
	if h.value("show_seasons", "poster_path", 1).String != "/season-poster.jpg" {
		t.Fatalf("season poster = %q, want it left remote", h.value("show_seasons", "poster_path", 1).String)
	}
	if h.upstream.requests("/w500/season-poster.jpg") != 0 {
		t.Fatal("downloaded an image whose file name another image holds")
	}
}
