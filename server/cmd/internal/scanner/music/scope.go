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

func (s *Scanner) appleAlbumCandidates(ctx context.Context, scan *musicScanContext, after int64) ([]database.MusicAppleAlbumCandidatesRow, error) {
	if scan.scope == nil {
		return s.queries.MusicAppleAlbumCandidates(ctx, after)
	}
	if after != 0 {
		return nil, nil
	}
	rows, err := s.queries.MusicAppleAlbumCandidatesIn(ctx, scan.scope.albums)
	candidates := make([]database.MusicAppleAlbumCandidatesRow, 0, len(rows))
	for _, row := range rows {
		candidates = append(candidates, database.MusicAppleAlbumCandidatesRow(row))
	}
	return candidates, err
}

func (s *Scanner) countAppleAlbumCandidates(ctx context.Context, scan *musicScanContext) (int64, error) {
	if scan.scope == nil {
		return s.queries.CountMusicAppleAlbumCandidates(ctx)
	}
	rows, err := s.appleAlbumCandidates(ctx, scan, 0)
	return int64(len(rows)), err
}

func (s *Scanner) artistRetryCandidates(ctx context.Context, scan *musicScanContext, after int64) ([]database.MusicArtistRetryCandidatesRow, error) {
	if scan.scope == nil {
		return s.queries.MusicArtistRetryCandidates(ctx, after)
	}
	if after != 0 {
		return nil, nil
	}
	rows, err := s.queries.MusicArtistRetryCandidatesIn(ctx, scan.scope.musicians)
	candidates := make([]database.MusicArtistRetryCandidatesRow, 0, len(rows))
	for _, row := range rows {
		candidates = append(candidates, database.MusicArtistRetryCandidatesRow(row))
	}
	return candidates, err
}

func (s *Scanner) countArtistRetryCandidates(ctx context.Context, scan *musicScanContext) (int64, error) {
	if scan.scope == nil {
		return s.queries.CountMusicArtistRetryCandidates(ctx)
	}
	rows, err := s.artistRetryCandidates(ctx, scan, 0)
	return int64(len(rows)), err
}

func (s *Scanner) albumRetryCandidates(ctx context.Context, scan *musicScanContext, after int64) ([]database.MusicAlbumRetryCandidatesRow, error) {
	if scan.scope == nil {
		return s.queries.MusicAlbumRetryCandidates(ctx, database.MusicAlbumRetryCandidatesParams{AfterID: after, AppleEnabled: scan.appleEnabled})
	}
	if after != 0 {
		return nil, nil
	}
	rows, err := s.queries.MusicAlbumRetryCandidatesIn(ctx, database.MusicAlbumRetryCandidatesInParams{AppleEnabled: scan.appleEnabled, Ids: scan.scope.albums})
	candidates := make([]database.MusicAlbumRetryCandidatesRow, 0, len(rows))
	for _, row := range rows {
		candidates = append(candidates, database.MusicAlbumRetryCandidatesRow(row))
	}
	return candidates, err
}

func (s *Scanner) countAlbumRetryCandidates(ctx context.Context, scan *musicScanContext) (int64, error) {
	if scan.scope == nil {
		return s.queries.CountMusicAlbumRetryCandidates(ctx, scan.appleEnabled)
	}
	rows, err := s.albumRetryCandidates(ctx, scan, 0)
	return int64(len(rows)), err
}

func (s *Scanner) trackLinkAlbumCandidates(ctx context.Context, scan *musicScanContext, after int64) ([]database.MusicTrackLinkAlbumCandidatesRow, error) {
	if scan.scope == nil {
		return s.queries.MusicTrackLinkAlbumCandidates(ctx, after)
	}
	if after != 0 {
		return nil, nil
	}
	rows, err := s.queries.MusicTrackLinkAlbumCandidatesIn(ctx, scan.scope.albums)
	candidates := make([]database.MusicTrackLinkAlbumCandidatesRow, 0, len(rows))
	for _, row := range rows {
		candidates = append(candidates, database.MusicTrackLinkAlbumCandidatesRow(row))
	}
	return candidates, err
}

func (s *Scanner) countTrackLinkAlbumCandidates(ctx context.Context, scan *musicScanContext) (int64, error) {
	if scan.scope == nil {
		return s.queries.CountMusicTrackLinkAlbumCandidates(ctx)
	}
	rows, err := s.trackLinkAlbumCandidates(ctx, scan, 0)
	return int64(len(rows)), err
}

func (s *Scanner) compoundReconciliationCandidates(ctx context.Context, scan *musicScanContext, after int64) ([]int64, error) {
	if scan.scope == nil {
		return s.queries.MusicCompoundReconciliationCandidates(ctx, after)
	}
	if after != 0 {
		return nil, nil
	}
	return s.queries.MusicCompoundReconciliationCandidatesIn(ctx, scan.scope.musicians)
}
