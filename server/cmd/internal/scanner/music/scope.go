package music

import (
	"context"
	"slices"

	"igloo/cmd/internal/database"
)

// musicScope holds the albums and artists of the tracks a run of
// watcher-detected changes touched. A library scan has none: its enrichment
// passes page through every candidate, while a run of changes reads the
// candidates among these ids in one query, so its only page is the whole set.
type musicScope struct {
	albums    []int64
	musicians []int64
}

// touchedScope reads the albums and artists the tracks at paths belong to
// now, after any merge this run made.
func (s *Scanner) touchedScope(ctx context.Context, paths []string) (*musicScope, error) {
	scope := &musicScope{}
	for _, path := range paths {
		rows, err := s.queries.MusicTrackAffected(ctx, path)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if row.AlbumID.Valid {
				scope.albums = append(scope.albums, row.AlbumID.Int64)
			}
			if row.MusicianID.Valid {
				scope.musicians = append(scope.musicians, row.MusicianID.Int64)
			}
		}
	}
	slices.Sort(scope.albums)
	scope.albums = slices.Compact(scope.albums)
	slices.Sort(scope.musicians)
	scope.musicians = slices.Compact(scope.musicians)
	return scope, nil
}

// refreshScope reads the scope again after a merge: the pass that merged a
// touched album or artist into an existing owner left the scope holding the
// deleted id, and the next pass would miss the owner.
func (s *Scanner) refreshScope(ctx context.Context, scan *musicScanContext) error {
	if scan.scope == nil || !scan.merged {
		return nil
	}
	scope, err := s.touchedScope(ctx, scan.touched)
	if err != nil {
		return err
	}
	scan.scope, scan.merged = scope, false
	return nil
}

// scopedPage returns one page of an enrichment pass's candidates: the page
// after `after` from all for a library scan, or for a run of changes the
// candidates among its scope from in, which is its only page.
func scopedPage[Row, In any](scope *musicScope, after int64, all func() ([]Row, error), in func(*musicScope) ([]In, error), convert func(In) Row) ([]Row, error) {
	if scope == nil {
		return all()
	}
	if after != 0 {
		return nil, nil
	}
	rows, err := in(scope)
	if err != nil {
		return nil, err
	}
	page := make([]Row, 0, len(rows))
	for _, row := range rows {
		page = append(page, convert(row))
	}
	return page, nil
}

// scopedCount counts an enrichment pass's candidates: countAll for a library
// scan, or the size of the only page for a run of changes.
func scopedCount[Row any](scope *musicScope, countAll func() (int64, error), firstPage func() ([]Row, error)) (int64, error) {
	if scope == nil {
		return countAll()
	}
	rows, err := firstPage()
	return int64(len(rows)), err
}

func (s *Scanner) appleAlbumCandidates(ctx context.Context, scan *musicScanContext, after int64) ([]database.MusicAppleAlbumCandidatesRow, error) {
	return scopedPage(scan.scope, after,
		func() ([]database.MusicAppleAlbumCandidatesRow, error) {
			return s.queries.MusicAppleAlbumCandidates(ctx, after)
		},
		func(scope *musicScope) ([]database.MusicAppleAlbumCandidatesInRow, error) {
			return s.queries.MusicAppleAlbumCandidatesIn(ctx, scope.albums)
		},
		func(row database.MusicAppleAlbumCandidatesInRow) database.MusicAppleAlbumCandidatesRow {
			return database.MusicAppleAlbumCandidatesRow(row)
		})
}

func (s *Scanner) countAppleAlbumCandidates(ctx context.Context, scan *musicScanContext) (int64, error) {
	return scopedCount(scan.scope,
		func() (int64, error) { return s.queries.CountMusicAppleAlbumCandidates(ctx) },
		func() ([]database.MusicAppleAlbumCandidatesRow, error) { return s.appleAlbumCandidates(ctx, scan, 0) })
}

func (s *Scanner) artistRetryCandidates(ctx context.Context, scan *musicScanContext, after int64) ([]database.MusicArtistRetryCandidatesRow, error) {
	return scopedPage(scan.scope, after,
		func() ([]database.MusicArtistRetryCandidatesRow, error) {
			return s.queries.MusicArtistRetryCandidates(ctx, after)
		},
		func(scope *musicScope) ([]database.MusicArtistRetryCandidatesInRow, error) {
			return s.queries.MusicArtistRetryCandidatesIn(ctx, scope.musicians)
		},
		func(row database.MusicArtistRetryCandidatesInRow) database.MusicArtistRetryCandidatesRow {
			return database.MusicArtistRetryCandidatesRow(row)
		})
}

func (s *Scanner) countArtistRetryCandidates(ctx context.Context, scan *musicScanContext) (int64, error) {
	return scopedCount(scan.scope,
		func() (int64, error) { return s.queries.CountMusicArtistRetryCandidates(ctx) },
		func() ([]database.MusicArtistRetryCandidatesRow, error) { return s.artistRetryCandidates(ctx, scan, 0) })
}

func (s *Scanner) albumRetryCandidates(ctx context.Context, scan *musicScanContext, after int64) ([]database.MusicAlbumRetryCandidatesRow, error) {
	return scopedPage(scan.scope, after,
		func() ([]database.MusicAlbumRetryCandidatesRow, error) {
			return s.queries.MusicAlbumRetryCandidates(ctx, database.MusicAlbumRetryCandidatesParams{AfterID: after, AppleEnabled: scan.appleEnabled})
		},
		func(scope *musicScope) ([]database.MusicAlbumRetryCandidatesInRow, error) {
			return s.queries.MusicAlbumRetryCandidatesIn(ctx, database.MusicAlbumRetryCandidatesInParams{AppleEnabled: scan.appleEnabled, Ids: scope.albums})
		},
		func(row database.MusicAlbumRetryCandidatesInRow) database.MusicAlbumRetryCandidatesRow {
			return database.MusicAlbumRetryCandidatesRow(row)
		})
}

func (s *Scanner) countAlbumRetryCandidates(ctx context.Context, scan *musicScanContext) (int64, error) {
	return scopedCount(scan.scope,
		func() (int64, error) { return s.queries.CountMusicAlbumRetryCandidates(ctx, scan.appleEnabled) },
		func() ([]database.MusicAlbumRetryCandidatesRow, error) { return s.albumRetryCandidates(ctx, scan, 0) })
}

func (s *Scanner) trackLinkAlbumCandidates(ctx context.Context, scan *musicScanContext, after int64) ([]database.MusicTrackLinkAlbumCandidatesRow, error) {
	return scopedPage(scan.scope, after,
		func() ([]database.MusicTrackLinkAlbumCandidatesRow, error) {
			return s.queries.MusicTrackLinkAlbumCandidates(ctx, after)
		},
		func(scope *musicScope) ([]database.MusicTrackLinkAlbumCandidatesInRow, error) {
			return s.queries.MusicTrackLinkAlbumCandidatesIn(ctx, scope.albums)
		},
		func(row database.MusicTrackLinkAlbumCandidatesInRow) database.MusicTrackLinkAlbumCandidatesRow {
			return database.MusicTrackLinkAlbumCandidatesRow(row)
		})
}

func (s *Scanner) countTrackLinkAlbumCandidates(ctx context.Context, scan *musicScanContext) (int64, error) {
	return scopedCount(scan.scope,
		func() (int64, error) { return s.queries.CountMusicTrackLinkAlbumCandidates(ctx) },
		func() ([]database.MusicTrackLinkAlbumCandidatesRow, error) {
			return s.trackLinkAlbumCandidates(ctx, scan, 0)
		})
}

func (s *Scanner) compoundReconciliationCandidates(ctx context.Context, scan *musicScanContext, after int64) ([]int64, error) {
	return scopedPage(scan.scope, after,
		func() ([]int64, error) { return s.queries.MusicCompoundReconciliationCandidates(ctx, after) },
		func(scope *musicScope) ([]int64, error) {
			return s.queries.MusicCompoundReconciliationCandidatesIn(ctx, scope.musicians)
		},
		func(id int64) int64 { return id })
}
