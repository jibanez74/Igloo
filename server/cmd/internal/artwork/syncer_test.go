package artwork

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func (h *harness) addShow(id int64, tmdbID any, name, poster, backdrop string) {
	h.t.Helper()
	h.exec(`INSERT INTO shows (id, directory_path, local_name, name, tmdb_id, poster_path_source, backdrop_path_source)
		VALUES (?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''))`, id, fmt.Sprintf("/shows/%s-%d", name, id), name, name, tmdbID, poster, backdrop)
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
