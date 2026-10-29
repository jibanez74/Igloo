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
	"sync/atomic"
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
	t        *testing.T
	db       *sql.DB
	logger   *scannertest.Logger
	upstream *upstream
	// upstreamURL serves the music sources too: they are full URLs.
	upstreamURL string
	syncer      *Syncer
	// deps builds the syncer, again on restart.
	deps      Dependencies
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
	h.upstreamURL = server.URL

	ctx, cancel := context.WithCancel(context.Background())
	wait := &sync.WaitGroup{}
	t.Cleanup(func() {
		cancel()
		wait.Wait()
	})

	h.deps = Dependencies{
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
	}
	h.syncer = New(h.deps)

	return h
}

// restart replaces the syncer with a new one over the same database and
// settings, as a server restart does.
func (h *harness) restart() {
	h.syncer = New(h.deps)
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
	h.syncer.pass(context.Background(), true)
}

// passChanges runs the pass a run of watcher-detected changes asks for.
func (h *harness) passChanges() {
	h.t.Helper()
	h.syncer.pass(context.Background(), false)
}

// passScopes counts the finished passes by the scope they logged.
func (h *harness) passScopes() map[string]int {
	scopes := map[string]int{}
	for _, entries := range [][]scannertest.LogEntry{h.logger.InfoEntries, h.logger.DebugEntries} {
		for _, entry := range entries {
			isSummary := entry.Msg == "artwork pass finished" && len(entry.Args) > 1 && entry.Args[0] == "scope"
			if isSummary {
				scopes[fmt.Sprint(entry.Args[1])]++
			}
		}
	}
	return scopes
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

// Files deleted from a folder that is still there are not what an unmounted
// disk looks like, even when they were every copy there was.
func TestPass_HealsEveryCopyDeletedFromAFolderThatStillExists(t *testing.T) {
	h := newHarness(t)
	h.addMovie(1, 603, "The Matrix", "/matrix-poster.jpg", "/matrix-backdrop.jpg")
	h.pass()

	for _, name := range []string{"the_matrix_poster.png", "the_matrix_backdrop.png"} {
		err := os.Remove(filepath.Join(h.static, "movies", "603", name))
		if err != nil {
			t.Fatal(err)
		}
	}
	h.pass()

	if !h.exists("movies/603/the_matrix_poster.png") || !h.exists("movies/603/the_matrix_backdrop.png") {
		t.Fatal("copies deleted from a folder that still exists were not downloaded again")
	}
	if len(h.logger.WarnEntries) != 0 {
		t.Fatalf("warned %q about a static directory that is mounted", h.logger.WarnEntries[0].Msg)
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
	if !h.logger.WarnMentions("skipping artwork pass: no downloaded artwork is left in the static directory; is it mounted? Turn Download images off and on to download it again", h.static) {
		t.Fatal("did not warn about the empty static directory")
	}

	// Another static directory is a different matter: its copies are made.
	h.static = t.TempDir()
	h.pass()
	if !h.exists("movies/603/the_matrix_poster.png") || h.value("movies", "poster_path", 1).String != copyURL {
		t.Fatal("a new static directory was not filled")
	}
}

// The database records the directory the copies were made in, so a disk that
// has not mounted by the first pass after a restart is left alone too.
func TestPass_LeavesAnEmptyStaticDirectoryAloneAfterARestart(t *testing.T) {
	h := newHarness(t)
	h.addMovie(1, 603, "The Matrix", "/matrix-poster.jpg", "/matrix-backdrop.jpg")
	h.pass()
	copyURL := h.value("movies", "poster_path", 1).String

	err := os.RemoveAll(filepath.Join(h.static, "movies"))
	if err != nil {
		t.Fatal(err)
	}
	h.restart()
	h.pass()

	if h.upstream.total() != 2 {
		t.Fatalf("made %d requests into what looks like an unmounted directory after a restart", h.upstream.total()-2)
	}
	if h.value("movies", "poster_path", 1).String != copyURL {
		t.Fatal("pointed a column away from its copy while the directory looked unmounted")
	}
	if !h.logger.WarnMentions("skipping artwork pass: no downloaded artwork is left in the static directory; is it mounted? Turn Download images off and on to download it again", h.static) {
		t.Fatal("did not warn about the empty static directory")
	}

	// Turning the setting off and on again downloads the copies again.
	h.enabled = false
	h.pass()
	h.enabled = true
	h.pass()
	if !h.exists("movies/603/the_matrix_poster.png") || h.value("movies", "poster_path", 1).String != copyURL {
		t.Fatal("turning the setting off and on again did not download the copies")
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

	// A refusal is forgotten once it has expired, even when its source
	// changed and nothing asks for it again.
	h.exec(`UPDATE movies SET poster_path_source = '/found.jpg' WHERE id = 1`)
	h.now = h.now.Add(failureRetryAfter + time.Minute)
	h.pass()
	if len(h.syncer.failures) != 0 {
		t.Fatalf("remembered %d expired refusals", len(h.syncer.failures))
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
		"playlists/12/summer_cover.jpg",
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

	// Passes run after every scan, so the overlap is reported once, until
	// it changes.
	const overlapMessage = "not managing artwork in a folder that overlaps a media library; choose another static directory"
	h.pass()
	if n := h.logger.ErrorCount(overlapMessage); n != 1 {
		t.Fatalf("reported the overlap %d times over two passes, want once", n)
	}
	h.libraries = []string{filepath.Join(h.static, "movies", "1999")}
	h.pass()
	if n := h.logger.ErrorCount(overlapMessage); n != 2 {
		t.Fatalf("reported the overlap %d times after it changed, want twice in all", n)
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

	h.syncer.pass(ctx, true)

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

	passes := h.passScopes()["full"]
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

// A key folder is the syncer's own; one that is a symlink, here into the movie
// library, is never written through, and a copy reached through one is not
// taken for one the syncer wrote.
func TestPass_NeverWritesThroughALinkedKeyFolder(t *testing.T) {
	h := newHarness(t)
	// Movie 1 is still remote, so a pass would download into its folder;
	// movie 2 already points at a file its linked folder leads to.
	h.addMovie(1, 603, "The Matrix", "/matrix-poster.jpg", "")
	h.addMovie(2, 604, "The Matrix Reloaded", "/reloaded-poster.jpg", "")
	h.exec(`UPDATE movies SET poster_path = ? WHERE id = 2`, local("movies/604/the_matrix_reloaded_poster.png", "w500", "/reloaded-poster.jpg"))
	err := os.MkdirAll(filepath.Join(h.static, "movies"), 0o755)
	if err != nil {
		t.Fatal(err)
	}
	libraryFiles := map[string]string{}
	for key, name := range map[string]string{"603": "the_matrix_poster.png", "604": "the_matrix_reloaded_poster.png"} {
		library := t.TempDir()
		h.libraries = append(h.libraries, library)
		libraryFiles[key] = filepath.Join(library, name)
		err = os.WriteFile(libraryFiles[key], []byte("the library's own"), 0o644)
		if err != nil {
			t.Fatal(err)
		}
		err = os.Symlink(library, filepath.Join(h.static, "movies", key))
		if err != nil {
			t.Fatal(err)
		}
	}

	h.pass()

	for key, path := range libraryFiles {
		saved, err := os.ReadFile(path)
		if err != nil || string(saved) != "the library's own" {
			t.Errorf("folder %s: the library's file holds %q (%v), want it untouched", key, saved, err)
		}
	}
	if h.upstream.total() != 0 {
		t.Errorf("made %d requests for linked key folders", h.upstream.total())
	}
	for id, source := range map[int64]string{1: "/matrix-poster.jpg", 2: "/reloaded-poster.jpg"} {
		got := h.value("movies", "poster_path", id).String
		if got != source {
			t.Errorf("movie %d poster = %q, want its source", id, got)
		}
	}
}

// Copies are trusted only in the directory they were made in. Back in a
// directory used before, a file left there from an older source is replaced,
// even though its name and the column's version agree.
func TestPass_DownloadsAgainInADirectoryTheCopiesWereNotMadeIn(t *testing.T) {
	h := newHarness(t)
	h.addMovie(1, 603, "The Matrix", "/old-poster.jpg", "")
	first := h.static
	h.pass()

	h.static = t.TempDir()
	h.pass()
	h.exec(`UPDATE movies SET poster_path_source = '/new-poster.jpg' WHERE id = 1`)
	h.pass()

	h.static = first
	h.pass()

	saved, err := os.ReadFile(filepath.Join(first, "movies", "603", "the_matrix_poster.png"))
	if err != nil || !strings.HasSuffix(string(saved), "/w500/new-poster.jpg") {
		t.Fatalf("the first directory's copy holds %q (%v), want the new poster", saved, err)
	}
	if h.value("movies", "poster_path", 1).String != local("movies/603/the_matrix_poster.png", "w500", "/new-poster.jpg") {
		t.Fatalf("poster = %q", h.value("movies", "poster_path", 1).String)
	}
}

// A pass that is downloading when downloads are turned off writes nothing
// after that: the save's Revert and the pass's writes share the scanner
// mutex, and each write first checks the setting.
func TestPass_WritesNothingOnceDownloadsAreTurnedOff(t *testing.T) {
	h := newHarness(t)
	h.addMovie(1, 603, "The Matrix", "/matrix-poster.jpg", "")
	var on atomic.Bool
	on.Store(true)
	h.syncer.enabled = on.Load
	h.upstream.handlers["/w500/matrix-poster.jpg"] = func(w http.ResponseWriter, r *http.Request) {
		on.Store(false)
		w.Write([]byte(pngSignature))
	}

	h.pass()

	if h.value("movies", "poster_path", 1).String != "/matrix-poster.jpg" {
		t.Fatalf("poster = %q, want no copy written after downloads were turned off", h.value("movies", "poster_path", 1).String)
	}
	for _, entry := range h.logger.ErrorEntries {
		t.Errorf("logged the stop as an error: %s %v", entry.Msg, entry.Args)
	}
}

func TestRevert_PointsEveryColumnBackAtOnce(t *testing.T) {
	h := newHarness(t)
	h.addMovie(1, 603, "The Matrix", "/matrix-poster.jpg", "/matrix-backdrop.jpg")
	h.addPerson(1, 6384, "Keanu Reeves", "/keanu.jpg")
	h.addShow(1, 1438, "The Wire", "/wire-poster.jpg", "")
	h.pass()

	err := h.syncer.Revert(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	for _, check := range []struct{ table, column, want string }{
		{"movies", "poster_path", "/matrix-poster.jpg"},
		{"movies", "backdrop_path", "/matrix-backdrop.jpg"},
		{"artist", "profile", "/keanu.jpg"},
		{"shows", "poster_path", "/wire-poster.jpg"},
	} {
		got := h.value(check.table, check.column, 1).String
		if got != check.want {
			t.Errorf("%s.%s = %q, want %q", check.table, check.column, got, check.want)
		}
	}
	// The copies stay for the pass that follows; with downloads still on, it
	// makes them again in the directory, since Revert forgot it.
	h.pass()
	if h.value("movies", "poster_path", 1).String != local("movies/603/the_matrix_poster.png", "w500", "/matrix-poster.jpg") {
		t.Fatal("the pass after a Revert did not point the column at its copy again")
	}
}

func TestPass_DownloadsAlbumCoversAndMusicianPhotos(t *testing.T) {
	h := newHarness(t)
	spotifyCover := h.upstreamURL + "/image/ab67616d0000b273"
	appleCover := h.upstreamURL + "/image/thumb/Music/daylight/640x640bb.jpg"
	h.exec(`INSERT INTO albums (id, title, sort_title, spotify_id, cover_source) VALUES (1, 'Daylight', 'daylight', '4aawyAB9vmqN3uQ7FjRGTy', ?)`, spotifyCover)
	h.exec(`INSERT INTO albums (id, title, sort_title, apple_id, cover_source) VALUES (2, 'Night Songs', 'night songs', 1440833098, ?)`, appleCover)
	// A cover without a provider match has no folder.
	h.exec(`INSERT INTO albums (id, title, sort_title, cover_source) VALUES (3, 'Untagged', 'untagged', ?)`, h.upstreamURL+"/image/untagged")
	h.exec(`INSERT INTO musicians (id, name, sort_name, spotify_id, thumb_source) VALUES (1, 'Aurora', 'aurora', '1WgXqy2Dd70QQOU7Ay074N', ?)`, h.upstreamURL+"/image/aurora")

	h.pass()

	want := []struct {
		table, column string
		id            int64
		rel, source   string
	}{
		{"albums", "cover", 1, "albums/4aawyAB9vmqN3uQ7FjRGTy/daylight_cover.png", spotifyCover},
		{"albums", "cover", 2, "albums/1440833098/night_songs_cover.png", appleCover},
		{"musicians", "thumb", 1, "musicians/1WgXqy2Dd70QQOU7Ay074N/aurora_thumb.png", h.upstreamURL + "/image/aurora"},
	}
	for _, w := range want {
		got := h.value(w.table, w.column, w.id).String
		if got != local(w.rel, "", w.source) {
			t.Errorf("%s.%s %d = %q, want the copy at %s", w.table, w.column, w.id, got, w.rel)
		}
		if !h.exists(w.rel) {
			t.Errorf("%s was not saved", w.rel)
		}
	}
	if h.upstream.requests("/image/ab67616d0000b273") != 1 || h.upstream.requests("/image/thumb/Music/daylight/640x640bb.jpg") != 1 {
		t.Errorf("music URLs were not fetched as stored, got %v", h.upstream.hits)
	}
	if h.value("albums", "cover", 3).String != h.upstreamURL+"/image/untagged" || h.upstream.requests("/image/untagged") != 0 {
		t.Error("a cover without a provider match did not stay remote")
	}

	h.enabled = false
	h.pass()
	for _, w := range want {
		if h.value(w.table, w.column, w.id).String != w.source {
			t.Errorf("%s.%s %d was not pointed back at its source", w.table, w.column, w.id)
		}
	}
	if h.exists("albums") || h.exists("musicians") {
		t.Error("turning downloads off left the music folders")
	}
}

// Each provider has its own breaker: TMDB failing over and over stops only
// the TMDB downloads, and the music covers from another host still come.
func TestPass_AFailingProviderStopsOnlyItsOwnDownloads(t *testing.T) {
	h := newHarness(t)
	for id := int64(1); id <= 10; id++ {
		poster := fmt.Sprintf("/poster-%d.jpg", id)
		h.addMovie(id, 600+id, "Movie", poster, "")
		h.upstream.set("/w500"+poster, http.StatusBadGateway)
	}
	music := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(pngSignature + r.URL.Path))
	}))
	t.Cleanup(music.Close)
	for id := int64(1); id <= 5; id++ {
		h.exec(`INSERT INTO albums (id, title, sort_title, spotify_id, cover_source) VALUES (?, ?, ?, ?, ?)`,
			id, fmt.Sprintf("Album %d", id), fmt.Sprintf("album %d", id), fmt.Sprintf("spotifyAlbum%d", id), fmt.Sprintf("%s/image/%d", music.URL, id))
	}

	h.pass()

	if h.upstream.total() >= 10 {
		t.Fatalf("made %d of 10 requests to a failing provider, want it stopped", h.upstream.total())
	}
	downloaded := scannertest.CountRows(t, h.db, `SELECT count(*) FROM albums WHERE cover LIKE '/api/static/%'`)
	if downloaded != 5 {
		t.Fatalf("downloaded %d of 5 album covers from a working provider", downloaded)
	}
}

// A library at the filesystem root contains every managed folder.
func TestPass_NeverManagesFoldersUnderALibraryAtTheRoot(t *testing.T) {
	h := newHarness(t)
	h.addMovie(1, 603, "The Matrix", "/matrix-poster.jpg", "")
	h.write("movies/1999/a_poster.jpg", "the library's own")
	h.libraries = []string{string(filepath.Separator)}

	h.pass()
	h.enabled = false
	h.pass()

	if h.upstream.total() != 0 {
		t.Fatalf("made %d requests with every folder inside a library", h.upstream.total())
	}
	if !h.exists("movies/1999/a_poster.jpg") {
		t.Fatal("deleted a file inside a library at the root")
	}
}

// The overlap check holds for the roots a pass started with. When a library
// is moved onto a managed folder mid-pass, the pass dispatches no more
// downloads and deletes nothing there; the pass the save starts points the
// folder's columns back at their sources and leaves its files alone.
func TestPass_StopsWritingAndDeletingWhenTheLibrariesChange(t *testing.T) {
	h := newHarness(t)
	for id := int64(1); id <= 12; id++ {
		h.addMovie(id, 600+id, fmt.Sprintf("Movie %d", id), fmt.Sprintf("/poster-%d.jpg", id), "")
	}
	h.write("movies/1999/valuable_poster.jpg", "the library's own")
	var libraries atomic.Pointer[[]string]
	libraries.Store(&[]string{})
	h.syncer.libraryDirs = func() []string { return *libraries.Load() }
	for id := 1; id <= 12; id++ {
		h.upstream.handlers[fmt.Sprintf("/w500/poster-%d.jpg", id)] = func(w http.ResponseWriter, r *http.Request) {
			libraries.Store(&[]string{filepath.Join(h.static, "movies")})
			w.Write([]byte(pngSignature + r.URL.Path))
		}
	}

	h.pass()

	if !h.exists("movies/1999/valuable_poster.jpg") {
		t.Fatal("deleted a file from a folder that became a library during the pass")
	}
	if h.upstream.total() > downloadWorkers {
		t.Fatalf("made %d requests, want none dispatched after the libraries changed", h.upstream.total())
	}

	h.pass()

	local := scannertest.CountRows(t, h.db, `SELECT count(*) FROM movies WHERE poster_path LIKE '/api/static/%'`)
	if local != 0 {
		t.Fatalf("%d posters still point into the library's folder", local)
	}
	if !h.exists("movies/1999/valuable_poster.jpg") {
		t.Fatal("the next pass deleted a file from the library's folder")
	}
}

// Pruning stops before its next file or folder once the library roots change,
// whether downloads are on or being turned off: the folder it would delete
// from may be a library's now. The pass the save starts leaves the library's
// files alone.
func TestPass_StopsPruningWhenTheLibrariesChange(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		t.Run(fmt.Sprintf("enabled=%v", enabled), func(t *testing.T) {
			h := newHarness(t)
			h.enabled = enabled
			first := "movies/101/a_backdrop.jpg"
			// Later in the same key folder, then in a later key folder.
			later := []string{"movies/101/b_poster.jpg", "movies/1999/valuable_poster.jpg"}
			h.write(first, "orphan")
			for _, rel := range later {
				h.write(rel, "the library's own")
			}
			// The movies folder becomes a library once the first file is gone.
			h.syncer.libraryDirs = func() []string {
				if h.exists(first) {
					return nil
				}
				return []string{filepath.Join(h.static, "movies")}
			}

			h.pass()

			if h.exists(first) {
				t.Fatal("did not prune before the libraries changed")
			}
			for _, rel := range later {
				if !h.exists(rel) {
					t.Fatalf("deleted %s after its folder became a library", rel)
				}
			}
			if h.syncer.cleanedDir != "" {
				t.Fatal("recorded a stopped removal as complete")
			}

			h.pass()

			for _, rel := range later {
				if !h.exists(rel) {
					t.Fatalf("the next pass deleted %s from the library's folder", rel)
				}
			}
		})
	}
}

// A pass of changes reads only the folders of the images still to download:
// a copy deleted by hand and a file nothing points at elsewhere wait for the
// next full pass.
func TestPassChanges_LeavesFoldersWithoutNewImagesAlone(t *testing.T) {
	h := newHarness(t)
	h.addMovie(1, 603, "The Matrix", "/matrix-poster.jpg", "/matrix-backdrop.jpg")
	h.addPerson(1, 6384, "Keanu Reeves", "/keanu.jpg")
	h.pass()

	err := os.Remove(filepath.Join(h.static, "movies", "603", "the_matrix_poster.png"))
	if err != nil {
		t.Fatal(err)
	}
	h.write("people/42/old_name_profile.jpg", "orphan")
	h.addMovie(2, 604, "Inception", "/inception-poster.jpg", "")
	h.passChanges()

	if h.value("movies", "poster_path", 2).String != local("movies/604/inception_poster.png", "w500", "/inception-poster.jpg") {
		t.Fatalf("new poster = %q, want its copy", h.value("movies", "poster_path", 2).String)
	}
	if h.exists("movies/603/the_matrix_poster.png") || h.upstream.requests("/w500/matrix-poster.jpg") != 1 {
		t.Fatal("a pass of changes healed a copy in a folder with nothing to download")
	}
	if !h.exists("people/42/old_name_profile.jpg") {
		t.Fatal("a pass of changes pruned a folder with nothing to download")
	}
	if h.passScopes()["changes"] != 1 {
		t.Fatalf("passes by scope = %v, want one pass of changes", h.passScopes())
	}

	h.pass()
	if !h.exists("movies/603/the_matrix_poster.png") || h.exists("people/42/old_name_profile.jpg") {
		t.Fatal("the next full pass did not heal the copy and remove the leftover file")
	}
}

// A pass of changes lists every row of a folder it works in, so the images
// already there are reused and kept, as a full pass keeps them.
func TestPassChanges_SeesEveryImageOfAFolderItTouches(t *testing.T) {
	h := newHarness(t)
	h.addMovie(1, 603, "The Matrix", "/matrix-poster.jpg", "")
	h.addShow(1, 1438, "The Wire", "/wire-poster.jpg", "")
	h.exec(`INSERT INTO show_seasons (id, show_id, season_number, name, poster_path_source) VALUES (1, 1, 1, 'Season 1', '/season-1.jpg')`)
	h.pass()
	showCopy := h.value("shows", "poster_path", 1).String
	seasonCopy := h.value("show_seasons", "poster_path", 1).String

	// Another file of the same movie, and a new episode of the show.
	h.addMovie(2, 603, "The Matrix", "/matrix-poster.jpg", "")
	h.exec(`INSERT INTO show_episodes (id, season_id, episode_number, name, still_path_source) VALUES (1, 1, 1, 'The Target', '/pilot.jpg')`)
	h.write("movies/603/old_title_poster.jpg", "orphan")
	h.passChanges()

	if h.upstream.requests("/w500/matrix-poster.jpg") != 1 || h.value("movies", "poster_path", 2).String != h.value("movies", "poster_path", 1).String {
		t.Fatal("another file of the movie did not reuse its copy")
	}
	if h.value("show_episodes", "still_path", 1).String != local("shows/1438/s01e01_still.png", "w500", "/pilot.jpg") {
		t.Fatalf("episode still = %q, want its copy", h.value("show_episodes", "still_path", 1).String)
	}
	if h.value("shows", "poster_path", 1).String != showCopy || h.value("show_seasons", "poster_path", 1).String != seasonCopy {
		t.Fatal("the show's other images were pointed away from their copies")
	}
	if !h.exists("shows/1438/the_wire_poster.png") || !h.exists("shows/1438/season_01_poster.png") {
		t.Fatal("removed copies the show's other images point at")
	}
	if h.upstream.requests("/w500/wire-poster.jpg") != 1 || h.upstream.requests("/w500/season-1.jpg") != 1 {
		t.Fatal("downloaded the show's other images again")
	}
	if h.exists("movies/603/old_title_poster.jpg") {
		t.Fatal("left a file nothing points at in a folder the pass worked in")
	}
}

// A season named like its show's poster stays remote in a pass of changes too,
// rather than overwrite the show's copy.
func TestPassChanges_KeepsTwoImagesOfOneNameApart(t *testing.T) {
	h := newHarness(t)
	h.addShow(1, 1438, "Season 01", "/show-poster.jpg", "")
	h.pass()

	h.exec(`INSERT INTO show_seasons (id, show_id, season_number, name, poster_path_source) VALUES (1, 1, 1, 'Season 1', '/season-poster.jpg')`)
	h.passChanges()

	saved, err := os.ReadFile(filepath.Join(h.static, "shows", "1438", "season_01_poster.png"))
	if err != nil || !strings.HasSuffix(string(saved), "/w500/show-poster.jpg") {
		t.Fatalf("the show's copy holds %q, want the show poster", saved)
	}
	if h.value("show_seasons", "poster_path", 1).String != "/season-poster.jpg" || h.upstream.requests("/w500/season-poster.jpg") != 0 {
		t.Fatal("downloaded an image whose file name another image holds")
	}
}

func TestPassChanges_DownloadsEveryKindOfImage(t *testing.T) {
	h := newHarness(t)
	h.pass()

	spotifyCover := h.upstreamURL + "/image/daylight"
	appleCover := h.upstreamURL + "/image/night-songs"
	musicianPhoto := h.upstreamURL + "/image/aurora"
	h.addMovie(1, 603, "The Matrix", "/matrix-poster.jpg", "/matrix-backdrop.jpg")
	h.addPerson(1, 6384, "Keanu Reeves", "/keanu.jpg")
	h.addShow(1, 1438, "The Wire", "/wire-poster.jpg", "/wire-backdrop.jpg")
	h.exec(`INSERT INTO show_seasons (id, show_id, season_number, name, poster_path_source) VALUES (1, 1, 1, 'Season 1', '/season-1.jpg')`)
	h.exec(`INSERT INTO show_episodes (id, season_id, episode_number, name, still_path_source) VALUES (1, 1, 1, 'The Target', '/pilot.jpg')`)
	h.exec(`INSERT INTO networks (id, tmdb_id, name, logo_source) VALUES (1, 49, 'HBO', '/hbo.png')`)
	h.exec(`INSERT INTO albums (id, title, sort_title, spotify_id, cover_source) VALUES (1, 'Daylight', 'daylight', '4aawyAB9vmqN3uQ7FjRGTy', ?)`, spotifyCover)
	h.exec(`INSERT INTO albums (id, title, sort_title, apple_id, cover_source) VALUES (2, 'Night Songs', 'night songs', 1440833098, ?)`, appleCover)
	h.exec(`INSERT INTO musicians (id, name, sort_name, spotify_id, thumb_source) VALUES (1, 'Aurora', 'aurora', '1WgXqy2Dd70QQOU7Ay074N', ?)`, musicianPhoto)
	h.passChanges()

	want := []struct {
		table, column string
		id            int64
		rel, size     string
		source        string
	}{
		{"movies", "poster_path", 1, "movies/603/the_matrix_poster.png", "w500", "/matrix-poster.jpg"},
		{"movies", "backdrop_path", 1, "movies/603/the_matrix_backdrop.png", "w1280", "/matrix-backdrop.jpg"},
		{"artist", "profile", 1, "people/6384/keanu_reeves_profile.png", "w185", "/keanu.jpg"},
		{"shows", "poster_path", 1, "shows/1438/the_wire_poster.png", "w500", "/wire-poster.jpg"},
		{"shows", "backdrop_path", 1, "shows/1438/the_wire_backdrop.png", "w1280", "/wire-backdrop.jpg"},
		{"show_seasons", "poster_path", 1, "shows/1438/season_01_poster.png", "w500", "/season-1.jpg"},
		{"show_episodes", "still_path", 1, "shows/1438/s01e01_still.png", "w500", "/pilot.jpg"},
		{"networks", "logo", 1, "networks/49/hbo_logo.png", "w92", "/hbo.png"},
		{"albums", "cover", 1, "albums/4aawyAB9vmqN3uQ7FjRGTy/daylight_cover.png", "", spotifyCover},
		{"albums", "cover", 2, "albums/1440833098/night_songs_cover.png", "", appleCover},
		{"musicians", "thumb", 1, "musicians/1WgXqy2Dd70QQOU7Ay074N/aurora_thumb.png", "", musicianPhoto},
	}
	for _, w := range want {
		got := h.value(w.table, w.column, w.id).String
		if got != local(w.rel, w.size, w.source) {
			t.Errorf("%s.%s %d = %q, want the copy at %s", w.table, w.column, w.id, got, w.rel)
		}
		if !h.exists(w.rel) {
			t.Errorf("%s was not saved", w.rel)
		}
	}
	if h.passScopes()["changes"] != 1 {
		t.Fatalf("passes by scope = %v, want one pass of changes", h.passScopes())
	}
}

// Images the provider refused stay remembered, and failures worth retrying
// are retried on the next pass, as in a full pass.
func TestPassChanges_RetriesFailuresLikeAFullPass(t *testing.T) {
	h := newHarness(t)
	h.pass()
	h.addMovie(1, 603, "The Matrix", "/missing.jpg", "/flaky.jpg")
	h.upstream.set("/w500/missing.jpg", http.StatusNotFound)
	h.upstream.set("/w1280/flaky.jpg", http.StatusBadGateway)

	h.passChanges()
	h.passChanges()
	if h.upstream.requests("/w500/missing.jpg") != 1 {
		t.Fatalf("a refused image was requested %d times within a day", h.upstream.requests("/w500/missing.jpg"))
	}
	if h.upstream.requests("/w1280/flaky.jpg") != 2 {
		t.Fatalf("a failing image was requested %d times in two passes, want each time", h.upstream.requests("/w1280/flaky.jpg"))
	}

	h.upstream.mu.Lock()
	delete(h.upstream.statuses, "/w1280/flaky.jpg")
	h.upstream.mu.Unlock()
	h.passChanges()
	if h.value("movies", "backdrop_path", 1).String != local("movies/603/the_matrix_backdrop.png", "w1280", "/flaky.jpg") {
		t.Fatal("an image that failed was not downloaded once its provider recovered")
	}
	if h.value("movies", "poster_path", 1).String != "/missing.jpg" {
		t.Fatal("a refused image did not stay remote")
	}
}

func TestPassChanges_RunsAFullPassOnceADayHasPassed(t *testing.T) {
	h := newHarness(t)
	h.addMovie(1, 603, "The Matrix", "/matrix-poster.jpg", "")
	h.pass()
	h.write("people/42/old_name_profile.jpg", "orphan")

	h.now = h.now.Add(fullPassInterval - time.Second)
	h.passChanges()
	if !h.exists("people/42/old_name_profile.jpg") {
		t.Fatal("ran a full pass before a day had passed")
	}

	h.now = h.now.Add(time.Second)
	h.passChanges()
	if h.exists("people/42/old_name_profile.jpg") {
		t.Fatal("did not run a full pass once a day had passed")
	}
	if h.passScopes()["full"] != 2 || h.passScopes()["changes"] != 1 {
		t.Fatalf("passes by scope = %v, want two full and one of changes", h.passScopes())
	}

	// The first pass after a restart is a full one.
	h.write("people/42/old_name_profile.jpg", "orphan")
	h.restart()
	h.passChanges()
	if h.exists("people/42/old_name_profile.jpg") {
		t.Fatal("the first pass after a restart was not a full one")
	}
}

func TestPassChanges_RunsAFullPassInANewStaticDirectory(t *testing.T) {
	h := newHarness(t)
	h.addMovie(1, 603, "The Matrix", "/matrix-poster.jpg", "")
	h.pass()

	h.static = t.TempDir()
	h.passChanges()

	if !h.exists("movies/603/the_matrix_poster.png") {
		t.Fatal("a new static directory was not filled")
	}
	if h.passScopes()["full"] != 2 {
		t.Fatalf("passes by scope = %v, want two full", h.passScopes())
	}
}

// A pass of changes lists no copy outside its folders, so it checks the
// folders that held copies before it downloads into what may be an empty
// mount point.
func TestPassChanges_LeavesAStaticDirectoryThatLostEveryCopyAlone(t *testing.T) {
	h := newHarness(t)
	h.addMovie(1, 603, "The Matrix", "/matrix-poster.jpg", "")
	// The first pass fills an empty directory, so its folders are known only
	// from the copies it made.
	h.pass()

	err := os.RemoveAll(filepath.Join(h.static, "movies"))
	if err != nil {
		t.Fatal(err)
	}
	h.addMovie(2, 604, "Inception", "/inception-poster.jpg", "")
	h.passChanges()
	if h.upstream.requests("/w500/inception-poster.jpg") != 0 || h.value("movies", "poster_path", 2).String != "/inception-poster.jpg" {
		t.Fatal("downloaded into what looks like an unmounted directory")
	}
	if !h.logger.WarnMentions(unmountedWarning, h.static) {
		t.Fatal("did not warn about the empty static directory")
	}

	// After a restart, neither the full pass nor the pass of changes after
	// it downloads there.
	h.restart()
	h.pass()
	h.passChanges()
	if h.upstream.requests("/w500/inception-poster.jpg") != 0 {
		t.Fatal("downloaded into what looks like an unmounted directory after a restart")
	}

	err = os.MkdirAll(filepath.Join(h.static, "movies"), 0o755)
	if err != nil {
		t.Fatal(err)
	}
	h.passChanges()
	if h.value("movies", "poster_path", 2).String != local("movies/604/inception_poster.png", "w500", "/inception-poster.jpg") {
		t.Fatal("did not download once the folder was back")
	}
}

// A managed folder may link to another disk outside every library; a full
// pass writes through it, and a pass of changes does not take it for missing.
func TestPassChanges_WritesThroughAManagedFolderThatLinksElsewhere(t *testing.T) {
	h := newHarness(t)
	err := os.Symlink(t.TempDir(), filepath.Join(h.static, "movies"))
	if err != nil {
		t.Fatal(err)
	}
	h.addMovie(1, 603, "The Matrix", "/matrix-poster.jpg", "")
	h.pass()
	if !h.exists("movies/603/the_matrix_poster.png") {
		t.Fatal("a full pass did not write through the linked folder")
	}

	h.addMovie(2, 604, "Inception", "/inception-poster.jpg", "")
	h.passChanges()
	if h.value("movies", "poster_path", 2).String != local("movies/604/inception_poster.png", "w500", "/inception-poster.jpg") {
		t.Fatal("a pass of changes did not write through the linked folder")
	}
	if len(h.logger.WarnEntries) != 0 {
		t.Fatalf("warned %v", h.logger.WarnEntries)
	}
}

func TestPassChanges_TurningOffRemovesEveryCopy(t *testing.T) {
	h := newHarness(t)
	h.addMovie(1, 603, "The Matrix", "/matrix-poster.jpg", "")
	h.addPerson(1, 6384, "Keanu Reeves", "/keanu.jpg")
	h.pass()

	h.enabled = false
	h.passChanges()

	if h.value("movies", "poster_path", 1).String != "/matrix-poster.jpg" || h.value("artist", "profile", 1).String != "/keanu.jpg" {
		t.Fatal("did not point the columns back at their sources")
	}
	if h.exists("movies") || h.exists("people") {
		t.Fatal("did not remove the copies")
	}
}

// Requests made during a pass share one pass after it, a full one if any of
// them asked for one. The first pass is a full one whatever asked for it.
func TestRequest_AFullRequestDuringAPassWinsOverChanges(t *testing.T) {
	tests := []struct {
		name   string
		during []func(*Syncer)
		want   map[string]int
	}{
		{"changes only", []func(*Syncer){(*Syncer).RequestChanges, (*Syncer).RequestChanges}, map[string]int{"full": 1, "changes": 1}},
		{"a full request among them", []func(*Syncer){(*Syncer).RequestChanges, (*Syncer).Request, (*Syncer).RequestChanges}, map[string]int{"full": 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			h.addMovie(1, 603, "The Matrix", "/matrix-poster.jpg", "")
			entered := make(chan struct{})
			release := make(chan struct{})
			h.upstream.handlers["/w500/matrix-poster.jpg"] = func(w http.ResponseWriter, r *http.Request) {
				close(entered)
				<-release
				w.Write([]byte(pngSignature))
			}

			h.syncer.RequestChanges()
			scannertest.WaitForSignal(t, entered, 5*time.Second, "the first pass to download")
			for _, request := range tt.during {
				request(h.syncer)
			}
			close(release)
			scannertest.WaitForGroup(t, h.syncer.wait, 5*time.Second, "the passes to finish")

			got := h.passScopes()
			if len(got) != len(tt.want) || got["full"] != tt.want["full"] || got["changes"] != tt.want["changes"] {
				t.Fatalf("passes by scope = %v, want %v", got, tt.want)
			}
		})
	}
}
