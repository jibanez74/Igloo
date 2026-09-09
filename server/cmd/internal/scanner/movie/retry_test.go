package movie

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/tmdb"
)

func nextMovieScan(t *testing.T, s *Scanner) *movieScanContext {
	t.Helper()
	index, _, err := s.loadMovieScanIndex(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return newMovieScanContext(index)
}

func retryMovieFixture(t *testing.T) tmdb.TmdbMovie {
	t.Helper()
	return tmdbMovieFromJSON(t, `{"id":42,"title":"Enriched","overview":"Overview","tagline":"Tagline","release_date":"2001-02-03","imdb_id":"tt42","poster_path":"/poster","backdrop_path":"/backdrop","adult":true,"original_language":"es","vote_average":8,"budget":100,"revenue":200,"runtime":999,"production_companies":[{"id":1,"name":"Studio"}],"genres":[{"id":1,"name":"Drama"}],"credits":{"cast":[{"id":1,"name":"Actor","character":"Lead","order":0}],"crew":[{"id":2,"name":"Director","job":"Director","department":"Directing"}]},"videos":{"results":[{"id":"trailer","key":"key","site":"YouTube","type":"Trailer"}]}}`)
}

func TestMovieEnrichmentRecovery(t *testing.T) {
	for _, failure := range []string{"offline", "empty", "search", "details", "timeout"} {
		for _, fingerprintOnly := range []bool{false, true} {
			t.Run(failure+map[bool]string{false: "/unchanged", true: "/fingerprint"}[fingerprintOnly], func(t *testing.T) {
				fixture := setupMovieScanner(t)
				defer fixture.db.Close()
				s := fixture.scanner
				ctx := context.Background()
				probe := &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("7200")}
				s.ffprobe = probe
				path := filepath.Join(t.TempDir(), "Local (2001).mkv")
				err := os.WriteFile(path, []byte("movie"), 0600)
				if err != nil {
					t.Fatal(err)
				}
				file := scanner.ScanFile{Path: path, Ext: "mkv"}
				details := retryMovieFixture(t)
				client := &stubMovieScannerTmdb{searchResults: []tmdb.TmdbMovie{{TmdbID: 42, Title: "Local"}}, detailMovies: map[int]tmdb.TmdbMovie{42: details}}
				switch failure {
				case "empty":
					client.searchErr = tmdb.ErrNoMoviesFound
				case "search":
					client.searchErr = errors.New("offline")
				case "details":
					client.detailErr = errors.New("offline")
				case "timeout":
					client.detailErr = context.DeadlineExceeded
				}
				if failure != "offline" {
					s.tmdb = client
				}
				scan := nextMovieScan(t, s)
				_, err = s.processFile(ctx, scan, file)
				if err != nil {
					t.Fatal(err)
				}
				movie, err := readTestMovieByPath(ctx, s.queries, path)
				if err != nil {
					t.Fatal(err)
				}
				if movie.Title != "Local" || movie.TmdbID.Valid || movie.RunTime.Int64 != 120 || movie.Duration.Float64 != 7200 {
					t.Fatalf("offline defaults: %+v", movie)
				}
				if countScannerRows(t, s.db, "SELECT count(*) FROM movie_tmdb_retries") != 1 {
					t.Fatal("missing retry")
				}
				searchCalls, detailCalls := len(client.searchCalls), len(client.detailCalls)
				_, err = s.processFile(ctx, scan, file)
				if err != nil {
					t.Fatal(err)
				}
				if len(client.searchCalls) != searchCalls || len(client.detailCalls) != detailCalls {
					t.Fatal("failed enrichment retried twice in one scan")
				}
				// Rows imported before retry bookkeeping also recover by missing identity.
				if failure == "offline" && !fingerprintOnly {
					err = s.queries.ClearMovieTmdbRetry(ctx, movie.ID)
					if err != nil {
						t.Fatal(err)
					}
				}
				_, err = s.db.Exec("UPDATE movies SET audience_rating=9 WHERE id=?", movie.ID)
				if err != nil {
					t.Fatal(err)
				}
				seedMoviePlaybackWork(t, s, movie.ID)
				streams, err := s.queries.GetVideoStreamsByMovieID(ctx, movie.ID)
				if err != nil {
					t.Fatal(err)
				}
				chapters, err := s.queries.GetChaptersByMovieID(ctx, movie.ID)
				if err != nil {
					t.Fatal(err)
				}
				invalidations := 0
				s.invalidateCommittedMovie = func(int64) { invalidations++ }
				client.searchErr = nil
				client.detailErr = nil
				s.tmdb = client
				if fingerprintOnly {
					err = os.Chmod(path, 0640)
					if err != nil {
						t.Fatal(err)
					}
				}
				scan = nextMovieScan(t, s)
				outcome, err := s.processFile(ctx, scan, file)
				if err != nil {
					t.Fatal(err)
				}
				want := scanner.FileUnchanged
				if fingerprintOnly {
					want = scanner.FileFingerprintOnly
				}
				if outcome != want {
					t.Fatalf("outcome=%v want=%v", outcome, want)
				}
				recovered, err := s.queries.GetMovieByID(ctx, movie.ID)
				if err != nil {
					t.Fatal(err)
				}
				if recovered.Title != details.Title || recovered.TmdbID.Int64 != 42 || recovered.AudienceRating.Float64 != 9 || recovered.RunTime != movie.RunTime || recovered.Duration != movie.Duration {
					t.Fatalf("recovery: %+v", recovered)
				}
				afterStreams, err := s.queries.GetVideoStreamsByMovieID(ctx, movie.ID)
				if err != nil {
					t.Fatal(err)
				}
				afterChapters, err := s.queries.GetChaptersByMovieID(ctx, movie.ID)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(chapters, afterChapters) {
					t.Fatal("metadata retry rebuilt chapters")
				}
				if !reflect.DeepEqual(streams, afterStreams) || probe.calls != 1 || invalidations != 0 {
					t.Fatal("metadata retry rebuilt technical state")
				}
				assertMoviePlaybackWork(t, s)
				if countScannerRows(t, s.db, "SELECT count(*) FROM movie_tmdb_retries") != 0 || scan.enriched != 1 {
					t.Fatal("success did not clear retry/count enrichment")
				}
				calls := len(client.detailCalls)
				_, err = s.processFile(ctx, scan, file)
				if err != nil {
					t.Fatal(err)
				}
				if len(client.detailCalls) != calls {
					t.Fatal("multiple enrichment attempts in one scan")
				}
			})
		}
	}
}

func seedMoviePlaybackWork(t *testing.T, s *Scanner, id int64) {
	t.Helper()
	for _, query := range []string{
		"INSERT INTO keyframe_indexes(movie_id,stream_index,fingerprint,duration_sec,keyframes) VALUES (?,0,'old',120,'[0]')",
		"INSERT INTO remux_safety_verdicts(movie_id,stream_index,fingerprint,safe) VALUES (?,0,'old',1)",
	} {
		_, err := s.db.Exec(query, id)
		if err != nil {
			t.Fatal(err)
		}
	}
}
func assertMoviePlaybackWork(t *testing.T, s *Scanner) {
	t.Helper()
	if countScannerRows(t, s.db, "SELECT count(*) FROM keyframe_indexes") != 1 || countScannerRows(t, s.db, "SELECT count(*) FROM remux_safety_verdicts") != 1 {
		t.Fatal("metadata retry invalidated playback work")
	}
}

func TestFailedChangedMoviePreservesMetadataAndRecoversByIdentity(t *testing.T) {
	fixture := setupMovieScanner(t)
	defer fixture.db.Close()
	s := fixture.scanner
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "Unrelated Filename.mkv")
	err := os.WriteFile(path, []byte("movie"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	file := scanner.ScanFile{Path: path, Ext: "mkv"}
	details := retryMovieFixture(t)
	client := &stubMovieScannerTmdb{searchResults: []tmdb.TmdbMovie{{TmdbID: 42, Title: "Enriched"}}, detailMovies: map[int]tmdb.TmdbMovie{42: details}}
	s.tmdb = client
	probe := &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("7200")}
	s.ffprobe = probe
	_, err = s.processFile(ctx, nextMovieScan(t, s), file)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.db.Exec("UPDATE movies SET audience_rating=9")
	if err != nil {
		t.Fatal(err)
	}
	if len(s.logger.(*capturedLogger).warnEntries) != 0 {
		t.Fatal("low-confidence import produced a warning")
	}
	before, err := readTestMovieByPath(ctx, s.queries, path)
	if err != nil {
		t.Fatal(err)
	}
	var relationIDs string
	err = s.db.QueryRow("SELECT group_concat(id) FROM cast").Scan(&relationIDs)
	if err != nil {
		t.Fatal(err)
	}
	client.detailErr = errors.New("TMDB down")
	probe.result = movieScannerMetadataFixture("7260")
	err = os.WriteFile(path, []byte("changed movie"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.processFile(ctx, nextMovieScan(t, s), file)
	if err != nil {
		t.Fatal(err)
	}
	after, err := readTestMovieByPath(ctx, s.queries, path)
	if err != nil {
		t.Fatal(err)
	}
	if after.Size != 13 || after.Duration.Float64 != 7260 || after.RunTime.Int64 != 121 {
		t.Fatalf("technical refresh: %+v", after)
	}
	preserved := after
	preserved.Size = before.Size
	preserved.Duration = before.Duration
	preserved.RunTime = before.RunTime
	preserved.UpdatedAt = before.UpdatedAt
	if !reflect.DeepEqual(before, preserved) {
		t.Fatalf("metadata lost:\nbefore=%+v\nafter=%+v", before, after)
	}
	var afterIDs string
	err = s.db.QueryRow("SELECT group_concat(id) FROM cast").Scan(&afterIDs)
	if err != nil {
		t.Fatal(err)
	}
	if relationIDs != afterIDs {
		t.Fatal("failed refresh rebuilt relationships")
	}
	for _, table := range []string{"cast", "crew", "movie_genres", "movie_production_companies", "movie_extra_videos"} {
		if countScannerRows(t, s.db, "SELECT count(*) FROM "+table) != 1 {
			t.Fatalf("lost %s", table)
		}
	}
	if len(client.searchCalls) != 1 || len(client.detailCalls) != 2 || client.detailCalls[1] != 42 {
		t.Fatal("identified movie was rematched")
	}
	seedMoviePlaybackWork(t, s, after.ID)
	client.detailErr = nil
	details.Title = "Refreshed"
	client.detailMovies[42] = details
	_, err = s.processFile(ctx, nextMovieScan(t, s), file)
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := s.queries.GetMovieByID(ctx, after.ID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Title != "Refreshed" || recovered.AudienceRating != after.AudienceRating || recovered.RunTime != after.RunTime || recovered.Duration != after.Duration || probe.calls != 2 {
		t.Fatalf("bad recovery: %+v", recovered)
	}
	assertMoviePlaybackWork(t, s)
}

// A hook executes while the scanner is waiting for remote details.
type hookedMovieTmdb struct {
	stubMovieScannerTmdb
	hook func()
}

func (h *hookedMovieTmdb) GetTmdbMovieByID(ctx context.Context, movie *tmdb.TmdbMovie) error {
	h.hook()
	return h.stubMovieScannerTmdb.GetTmdbMovieByID(ctx, movie)
}

func TestMovieRetryAtomicityAndStaleResults(t *testing.T) {
	for _, technical := range []bool{false, true} {
		for _, mutation := range []string{"identify", "delete", "replace", "path", "rollback", "cancel", "file"} {
			t.Run(map[bool]string{false: "metadata", true: "technical"}[technical]+"/"+mutation, func(t *testing.T) {
				fixture := setupMovieScanner(t)
				defer fixture.db.Close()
				s := fixture.scanner
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				path := filepath.Join(t.TempDir(), "Local.mkv")
				err := os.WriteFile(path, []byte("movie"), 0600)
				if err != nil {
					t.Fatal(err)
				}
				file := scanner.ScanFile{Path: path, Ext: "mkv"}
				s.ffprobe = &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("120")}
				_, err = s.processFile(ctx, nextMovieScan(t, s), file)
				if err != nil {
					t.Fatal(err)
				}
				before, err := readTestMovieByPath(ctx, s.queries, path)
				if err != nil {
					t.Fatal(err)
				}
				seedMoviePlaybackWork(t, s, before.ID)
				scan := nextMovieScan(t, s)
				baseline := scan.movieIndex[path]
				if technical {
					err = os.WriteFile(path, []byte("changed"), 0600)
					if err != nil {
						t.Fatal(err)
					}
				}
				invalidations := 0
				s.invalidateCommittedMovie = func(int64) { invalidations++ }
				details := retryMovieFixture(t)
				client := &hookedMovieTmdb{stubMovieScannerTmdb: stubMovieScannerTmdb{searchResults: []tmdb.TmdbMovie{{TmdbID: 42, Title: "Local"}}, detailMovies: map[int]tmdb.TmdbMovie{42: details}}}
				client.hook = func() {
					// Taking this lock also verifies external requests run outside it.
					s.scannerDBMu.Lock()
					defer s.scannerDBMu.Unlock()
					switch mutation {
					case "identify":
						tx, e := s.db.BeginTx(ctx, nil)
						if e != nil {
							t.Fatal(e)
						}
						manual := tmdb.TmdbMovie{TmdbID: 99, Title: "Manual identity"}
						e = ApplyTmdbMetadata(ctx, s.queries.WithTx(tx), before.ID, &manual)
						if e != nil {
							tx.Rollback()
							t.Fatal(e)
						}
						e = tx.Commit()
						if e != nil {
							t.Fatal(e)
						}
					case "delete", "replace":
						_, err = s.db.Exec("DELETE FROM movies WHERE id=?", before.ID)
						if err == nil && mutation == "replace" {
							_, err = s.queries.UpsertMovie(ctx, database.UpsertMovieParams{Title: "Replacement", FilePath: path, FileName: "Local.mkv", Container: "mkv", MimeType: "video/x-matroska"})
						}
					case "path":
						_, err = s.db.Exec("UPDATE movies SET file_path=? WHERE id=?", path+".moved", before.ID)
					case "rollback":
						_, err = s.db.Exec("CREATE TRIGGER fail_retry BEFORE DELETE ON movie_tmdb_retries BEGIN SELECT RAISE(ABORT,'retry failure'); END")
					case "cancel":
						cancel()
					case "file":
						err = os.WriteFile(path, []byte("raced replacement"), 0600)
					}
					if err != nil {
						t.Fatal(err)
					}
				}
				s.tmdb = client
				_, err = s.processFile(ctx, scan, file)
				wantError := mutation == "rollback" || mutation == "cancel" || mutation == "file"
				if (err != nil) != wantError {
					t.Fatalf("persistence error=%v wantError=%v", err, wantError)
				}
				if mutation == "identify" {
					current, e := s.queries.GetMovieByID(context.Background(), before.ID)
					if e != nil {
						t.Fatal(e)
					}
					if current.TmdbID.Int64 != 99 || current.Title != "Manual identity" {
						t.Fatalf("stale enrichment applied: %+v", current)
					}
					if technical && (current.Size != 7 || invalidations != 1) {
						t.Fatal("valid technical update was lost")
					}
					if countScannerRows(t, s.db, "SELECT count(*) FROM movie_tmdb_retries") != 0 {
						t.Fatal("manual identify retry was restored")
					}
				} else {
					if invalidations != 0 || scan.movieIndex[path] != baseline || scan.enriched != 0 {
						t.Fatal("discarded/failed transaction published changes")
					}
					if mutation == "rollback" || mutation == "cancel" || mutation == "file" {
						stored := nextMovieScan(t, s)
						if stored.movieIndex[path] != baseline {
							t.Fatal("partial fingerprint published")
						}
						current, e := s.queries.GetMovieByID(context.Background(), before.ID)
						if e != nil {
							t.Fatal(e)
						}
						if !reflect.DeepEqual(current, before) {
							t.Fatal("partial metadata published")
						}
						assertMoviePlaybackWork(t, s)
						if countScannerRows(t, s.db, "SELECT count(*) FROM movie_tmdb_retries") != 1 {
							t.Fatal("lost retry after rollback")
						}
						if scan.genreIDs.Has(scanner.NormalizedScanCacheKey("Drama", "movie")) {
							t.Fatal("rolled back genre cache published")
						}
					}
					if mutation == "delete" || mutation == "replace" {
						if countScannerRows(t, s.db, "SELECT count(*) FROM movie_tmdb_retries") != 0 {
							t.Fatal("retry deletion did not cascade")
						}
						if mutation == "replace" {
							current, e := readTestMovieByPath(context.Background(), s.queries, path)
							if e != nil || current.ID == before.ID || current.Title != "Replacement" {
								t.Fatalf("replacement overwritten: %+v %v", current, e)
							}
						}
					}
				}
			})
		}
	}
}

func readTestMovieByPath(ctx context.Context, q *database.Queries, path string) (database.Movie, error) {
	row, err := q.GetMovieByPath(ctx, path)
	if err != nil {
		return database.Movie{}, err
	}
	return q.GetMovieByID(ctx, row.ID)
}
