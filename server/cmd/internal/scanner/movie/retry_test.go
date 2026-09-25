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
	"igloo/cmd/internal/scanner/scannertest"
	"igloo/cmd/internal/tmdb"
)

func TestMovieEnrichmentRecovery(t *testing.T) {
	for _, failure := range []string{"offline", "empty", "search", "details", "timeout"} {
		for _, metadataChanged := range []bool{false, true} {
			t.Run(failure+map[bool]string{false: "/unchanged", true: "/metadata-change"}[metadataChanged], func(t *testing.T) {
				fixture := setupMovieScanner(t)
				s := fixture.scanner
				ctx := context.Background()
				probe := &scannertest.CountingProbe{Default: movieScannerMetadataFixture("7200")}
				s.ffprobe = probe
				path := filepath.Join(t.TempDir(), "Local (2001).mkv")
				scannertest.WriteFile(t, path, "movie")
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
				_, err := s.processFile(ctx, scan, file)
				// A provider failure is reported as an enrichment failure; an
				// empty result or no provider is a miss, not a failure.
				providerFailed := failure == "search" || failure == "details" || failure == "timeout"
				if (err != nil) != providerFailed {
					t.Fatalf("first scan error = %v, want failure %v", err, providerFailed)
				}
				movie, err := readTestMovieByPath(ctx, s.queries, path)
				if err != nil {
					t.Fatal(err)
				}
				if movie.Title != "Local" || movie.TmdbID.Valid || movie.RunTime.Int64 != 120 || movie.Duration.Float64 != 7200 {
					t.Fatalf("offline defaults: %+v", movie)
				}
				if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM movie_tmdb_retries") != 1 {
					t.Fatal("missing retry")
				}
				// Rows imported before retry bookkeeping also recover by missing identity.
				if failure == "offline" && !metadataChanged {
					err = s.queries.ClearMovieTmdbRetry(ctx, movie.ID)
					if err != nil {
						t.Fatal(err)
					}
				}
				_, err = s.tx.DB.Exec("UPDATE movies SET audience_rating=9 WHERE id=?", movie.ID)
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
				if metadataChanged {
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
				if metadataChanged {
					want = scanner.FileNeedsProcessing
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
				if !metadataChanged && !reflect.DeepEqual(chapters, afterChapters) {
					t.Fatal("metadata retry rebuilt chapters")
				}
				if metadataChanged {
					if probe.Calls() != 2 || invalidations != 1 {
						t.Fatal("changed metadata must re-probe and invalidate")
					}
				} else if !reflect.DeepEqual(streams, afterStreams) || probe.Calls() != 1 || invalidations != 0 {
					t.Fatal("metadata retry rebuilt technical state")
				}
				if !metadataChanged {
					assertMoviePlaybackWork(t, s)
				}
				if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM movie_tmdb_retries") != 0 || scan.enriched != 1 {
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
		_, err := s.tx.DB.Exec(query, id)
		if err != nil {
			t.Fatal(err)
		}
	}
}
func assertMoviePlaybackWork(t *testing.T, s *Scanner) {
	t.Helper()
	if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM keyframe_indexes") != 1 || scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM remux_safety_verdicts") != 1 {
		t.Fatal("metadata retry invalidated playback work")
	}
}

func TestChangedMoviePreservesConfirmedMatchAndMetadata(t *testing.T) {
	fixture := setupMovieScanner(t)
	s := fixture.scanner
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "Unrelated Filename.mkv")
	scannertest.WriteFile(t, path, "movie")
	file := scanner.ScanFile{Path: path, Ext: "mkv"}
	details := retryMovieFixture(t)
	client := &stubMovieScannerTmdb{searchResults: []tmdb.TmdbMovie{{TmdbID: 42, Title: "Enriched"}}, detailMovies: map[int]tmdb.TmdbMovie{42: details}}
	s.tmdb = client
	probe := &scannertest.CountingProbe{Default: movieScannerMetadataFixture("7200")}
	s.ffprobe = probe
	_, err := s.processFile(ctx, nextMovieScan(t, s), file)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.tx.DB.Exec("UPDATE movies SET audience_rating=9")
	if err != nil {
		t.Fatal(err)
	}
	if len(s.logger.(*scannertest.Logger).WarnEntries) != 0 {
		t.Fatal("low-confidence import produced a warning")
	}
	before, err := readTestMovieByPath(ctx, s.queries, path)
	if err != nil {
		t.Fatal(err)
	}
	var relationIDs string
	err = s.tx.DB.QueryRow("SELECT group_concat(id) FROM cast").Scan(&relationIDs)
	if err != nil {
		t.Fatal(err)
	}
	client.detailErr = errors.New("TMDB down")
	probe.Default = movieScannerMetadataFixture("7260")
	scannertest.WriteFile(t, path, "changed movie")
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
	err = s.tx.DB.QueryRow("SELECT group_concat(id) FROM cast").Scan(&afterIDs)
	if err != nil {
		t.Fatal(err)
	}
	if relationIDs != afterIDs {
		t.Fatal("failed refresh rebuilt relationships")
	}
	for _, table := range []string{"cast", "crew", "movie_genres", "movie_production_companies", "movie_extra_videos"} {
		if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM "+table) != 1 {
			t.Fatalf("lost %s", table)
		}
	}
	// The changed file never reached TMDB: a confirmed match is not re-searched,
	// so the provider being down could not have cost anything here.
	if len(client.searchCalls) != 1 || len(client.detailCalls) != 1 {
		t.Fatalf("confirmed match was re-searched: searches=%d details=%v", len(client.searchCalls), client.detailCalls)
	}
	pending, err := s.queries.HasMovieTmdbRetry(ctx, after.ID)
	if err != nil {
		t.Fatal(err)
	}
	if pending {
		t.Fatal("technical change re-queued enrichment for a confirmed match")
	}

	// A healthy provider does not change that, and an unchanged file leaves the
	// persisted playback work alone.
	seedMoviePlaybackWork(t, s, after.ID)
	client.detailErr = nil
	details.Title = "Refreshed"
	client.detailMovies[42] = details
	_, err = s.processFile(ctx, nextMovieScan(t, s), file)
	if err != nil {
		t.Fatal(err)
	}
	rescanned, err := s.queries.GetMovieByID(ctx, after.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rescanned.Title != "Enriched" || rescanned.AudienceRating != after.AudienceRating || rescanned.RunTime != after.RunTime || rescanned.Duration != after.Duration || probe.Calls() != 2 {
		t.Fatalf("confirmed match was refreshed by a rescan: %+v", rescanned)
	}
	if len(client.detailCalls) != 1 {
		t.Fatalf("detail calls = %v, want the confirmed match left alone", client.detailCalls)
	}
	assertMoviePlaybackWork(t, s)
}

// A hook executes while the scanner is waiting for remote details.

func TestMovieRetryAtomicityAndStaleResults(t *testing.T) {
	for _, technical := range []bool{false, true} {
		for _, mutation := range []string{"identify", "identify-same", "delete", "replace", "path", "rollback", "cancel", "file"} {
			t.Run(map[bool]string{false: "metadata", true: "technical"}[technical]+"/"+mutation, func(t *testing.T) {
				fixture := setupMovieScanner(t)
				s := fixture.scanner
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				path := filepath.Join(t.TempDir(), "Local.mkv")
				scannertest.WriteFile(t, path, "movie")
				file := scanner.ScanFile{Path: path, Ext: "mkv"}
				s.ffprobe = &scannertest.CountingProbe{Default: movieScannerMetadataFixture("120")}
				_, err := s.processFile(ctx, nextMovieScan(t, s), file)
				if err != nil {
					t.Fatal(err)
				}
				if mutation == "identify-same" {
					_, err = s.tx.DB.Exec("UPDATE movies SET tmdb_id=99")
					if err != nil {
						t.Fatal(err)
					}
				}
				before, err := readTestMovieByPath(ctx, s.queries, path)
				if err != nil {
					t.Fatal(err)
				}
				seedMoviePlaybackWork(t, s, before.ID)
				scan := nextMovieScan(t, s)
				baseline := scan.movieIndex[path]
				if technical {
					scannertest.WriteFile(t, path, "changed")
				}
				invalidations := 0
				s.invalidateCommittedMovie = func(int64) { invalidations++ }
				details := retryMovieFixture(t)
				// identify-same starts from a movie already pinned to 99, so the
				// scanner refreshes 99 by id; the refetch must lose to the manual
				// identity applied meanwhile.
				client := &stubMovieScannerTmdb{searchResults: []tmdb.TmdbMovie{{TmdbID: 42, Title: "Local"}}, detailMovies: map[int]tmdb.TmdbMovie{42: details, 99: {TmdbID: 99, Title: "Refetched identity"}}}
				// The hook runs on an enrichment worker, so it reports with t.Error.
				client.detailHook = func() {
					if technical {
						baseline = scan.movieIndex[path]
						before, err = s.queries.GetMovieByID(ctx, before.ID)
						if err != nil {
							t.Error(err)
							return
						}
						if invalidations != 1 {
							t.Error("technical movie was not committed before enrichment")
							return
						}
						invalidations = 0
						seedMoviePlaybackWork(t, s, before.ID)
					}
					// Taking this lock also verifies external requests run outside it.
					s.tx.Mu.Lock()
					defer s.tx.Mu.Unlock()
					switch mutation {
					case "identify", "identify-same":
						tx, e := s.tx.DB.BeginTx(ctx, nil)
						if e != nil {
							t.Error(e)
							return
						}
						manual := tmdb.TmdbMovie{TmdbID: 99, Title: "Manual identity"}
						e = ApplyTmdbMetadata(ctx, s.queries.WithTx(tx), before.ID, &manual)
						if e != nil {
							tx.Rollback()
							t.Error(e)
							return
						}
						e = tx.Commit()
						if e != nil {
							t.Error(e)
							return
						}
					case "delete", "replace":
						_, err = s.tx.DB.Exec("DELETE FROM movies WHERE id=?", before.ID)
						if err == nil && mutation == "replace" {
							_, err = s.queries.UpsertMovie(ctx, database.UpsertMovieParams{Title: "Replacement", FilePath: path, FileName: "Local.mkv", Container: "mkv", MimeType: "video/x-matroska"})
						}
					case "path":
						_, err = s.tx.DB.Exec("UPDATE movies SET file_path=? WHERE id=?", path+".moved", before.ID)
					case "rollback":
						_, err = s.tx.DB.Exec("CREATE TRIGGER fail_retry BEFORE DELETE ON movie_tmdb_retries BEGIN SELECT RAISE(ABORT,'retry failure'); END")
					case "cancel":
						cancel()
					case "file":
						err = os.WriteFile(path, []byte("raced replacement"), 0600)
					}
					if err != nil {
						t.Error(err)
						return
					}
				}
				s.tmdb = client
				_, err = s.processFile(ctx, scan, file)
				// A file replaced under the enrichment is deferred to the next
				// scan rather than failed; the other two abort the transaction.
				wantError := mutation == "rollback" || mutation == "cancel"
				if (err != nil) != wantError {
					t.Fatalf("persistence error=%v wantError=%v", err, wantError)
				}
				if mutation == "file" && !s.logger.(*scannertest.Logger).DebugMentions("deferred movie enrichment", path) {
					t.Fatalf("replaced file was not deferred: %+v", s.logger.(*scannertest.Logger).DebugEntries)
				}
				if mutation == "identify" || mutation == "identify-same" {
					current, e := s.queries.GetMovieByID(context.Background(), before.ID)
					if e != nil {
						t.Fatal(e)
					}
					if current.TmdbID.Int64 != 99 || current.Title != "Manual identity" {
						t.Fatalf("stale enrichment applied: %+v", current)
					}
					if technical && (current.Size != 7 || invalidations != 0) {
						t.Fatal("valid technical update was lost")
					}
					if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM movie_tmdb_retries") != 0 {
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
						if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM movie_tmdb_retries") != 1 {
							t.Fatal("lost retry after rollback")
						}
						_, cached := scan.genreIDs.Get(scanner.NormalizedScanCacheKey("Drama", "movie"))
						if cached {
							t.Fatal("rolled back genre cache published")
						}
						_, cached = scan.artistIDs.Get(1)
						if cached {
							t.Fatal("rolled back artist cache published")
						}
					}
					if mutation == "delete" || mutation == "replace" {
						if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM movie_tmdb_retries") != 0 {
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

// A movie the user edited through the Edit dialog keeps those values when the
// file changes. Enrichment owns the same columns and overwrites all of them, so
// re-queueing a confirmed match on a technical change reverted every edit.
// The same change must not strand an unmatched movie: it still gets queued and
// searched on the technical change that re-imported it.
func TestTechnicalRescanStillQueuesUnmatchedMovie(t *testing.T) {
	fixture := setupMovieScanner(t)
	s := fixture.scanner
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "Unmatched Filename.mkv")
	scannertest.WriteFile(t, path, "movie")
	file := scanner.ScanFile{Path: path, Ext: "mkv"}
	client := &stubMovieScannerTmdb{searchErr: tmdb.ErrNoMoviesFound}
	s.tmdb = client
	s.ffprobe = &scannertest.CountingProbe{Default: movieScannerMetadataFixture("7200")}
	_, err := s.processFile(ctx, nextMovieScan(t, s), file)
	if err != nil {
		t.Fatal(err)
	}
	scannertest.WriteFile(t, path, "changed movie")
	_, err = s.processFile(ctx, nextMovieScan(t, s), file)
	if err != nil {
		t.Fatal(err)
	}
	movie, err := readTestMovieByPath(ctx, s.queries, path)
	if err != nil {
		t.Fatal(err)
	}
	if movie.TmdbID.Valid {
		t.Fatalf("unmatched movie gained a match: %+v", movie)
	}
	if movie.Size != 13 {
		t.Fatalf("technical refresh did not run: size=%d", movie.Size)
	}
	pending, err := s.queries.HasMovieTmdbRetry(ctx, movie.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !pending {
		t.Fatal("unmatched movie was not queued for enrichment")
	}
}
