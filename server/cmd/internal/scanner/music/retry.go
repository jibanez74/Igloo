package music

import (
	"context"
	"database/sql"

	"igloo/cmd/internal/database"
)

// Retry only persisted catalog metadata. Pages are closed before any requests or writes.
func (s *Scanner) retrySpotify(ctx context.Context, scan *musicScanContext) error {
	if s.spotify == nil {
		return nil
	}
	var after int64
	for {
		candidates, err := s.queries.MusicArtistRetryCandidates(ctx, after)
		if err != nil {
			return err
		}
		if len(candidates) == 0 {
			break
		}
		for _, candidate := range candidates {
			after = candidate.ID
			resolved, err := s.resolveMusician(ctx, scan, candidate.Name, "")
			if err != nil {
				return err
			}
			err = s.persistEnrichment(ctx, scan, func(qtx *database.Queries, txScan *musicScanContext) error {
				id, persistErr := s.persistMusician(ctx, qtx, txScan, *resolved)
				if persistErr != nil {
					return persistErr
				}
				return qtx.ReconcileMusicArtistSort(ctx, id)
			})
			if err != nil {
				return err
			}
			if resolved.splitCompoundOnNoMatch {
				err = s.reconcileCompoundCredits(ctx, scan, candidate.ID)
				if err != nil {
					return err
				}
			}
		}
	}
	after = 0
	for {
		candidates, err := s.queries.MusicAlbumRetryCandidates(ctx, after)
		if err != nil {
			return err
		}
		if len(candidates) == 0 {
			return s.reconcilePendingCompoundCredits(ctx, scan)
		}
		for _, candidate := range candidates {
			after = candidate.ID
			resolved, err := s.resolveAlbum(ctx, scan, candidate.Title, candidate.Title, candidate.Musician.String)
			if err != nil {
				return err
			}
			err = s.persistEnrichment(ctx, scan, func(qtx *database.Queries, txScan *musicScanContext) error {
				id, persistErr := s.persistAlbum(ctx, qtx, txScan, *resolved)
				if persistErr != nil {
					return persistErr
				}
				return reconcileAlbum(ctx, qtx, id)
			})
			if err != nil {
				return err
			}
		}
	}
}

func (s *Scanner) persistEnrichment(ctx context.Context, scan *musicScanContext, persist func(*database.Queries, *musicScanContext) error) error {
	txScan := scan.clone()
	err := s.tx.Run(ctx, func(qtx *database.Queries) error {
		return persist(qtx, txScan)
	}, func() {
		for id := range txScan.invalidatedTracks {
			s.invalidateCommittedTrack(id)
		}
	})
	if err != nil {
		return err
	}
	scan.mergeFrom(txScan)
	return nil
}

func (s *Scanner) reconcileCompoundCredits(ctx context.Context, scan *musicScanContext, musicianID int64) error {
	var after int64
	for {
		rows, err := s.queries.MusicArtistTrackMetadata(ctx, database.MusicArtistTrackMetadataParams{MusicianID: musicianID, AfterID: after})
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		for _, row := range rows {
			after = row.TrackID
			parsed := parseCompoundArtistCredits(row.ArtistTag)
			if len(parsed.parts) < 2 {
				continue
			}
			credits, err := s.resolveTrackMusicians(ctx, scan, row.ArtistTag, row.ArtistSort)
			if err != nil {
				return err
			}
			err = s.persistEnrichment(ctx, scan, func(qtx *database.Queries, txScan *musicScanContext) error {
				ids, persistErr := s.persistMusicians(ctx, qtx, txScan, credits)
				if persistErr != nil {
					return persistErr
				}

				syncErr := s.syncTrackMusicians(ctx, qtx, row.TrackID, ids)
				if syncErr != nil {
					return syncErr
				}
				var primary sql.NullInt64
				if len(ids) > 0 {
					primary = sql.NullInt64{Int64: ids[0], Valid: true}
				}
				syncErr = qtx.UpdateMusicTrackPrimaryArtist(ctx, database.UpdateMusicTrackPrimaryArtistParams{ID: row.TrackID, MusicianID: primary})
				if syncErr != nil {
					return syncErr
				}
				syncErr = qtx.DeleteMusicCreditMetadata(ctx, row.TrackID)
				if syncErr != nil {
					return syncErr
				}
				for i, id := range ids {
					syncErr = qtx.SaveMusicCreditMetadata(ctx, database.SaveMusicCreditMetadataParams{TrackID: row.TrackID, MusicianID: id, SortName: credits[i].sortName})
					if syncErr != nil {
						return syncErr
					}
				}
				for _, id := range append(ids, musicianID) {
					syncErr = qtx.ReconcileMusicArtistSort(ctx, id)
					if syncErr != nil {
						return syncErr
					}
				}
				txScan.invalidatedTracks[row.TrackID] = true
				return nil
			})
			if err != nil {
				return err
			}
		}
	}
}

// A previous scan may have committed the match before cancellation interrupted
// credit reconciliation. Final non-matches still need to finish those credits.
func (s *Scanner) reconcilePendingCompoundCredits(ctx context.Context, scan *musicScanContext) error {
	var after int64
	for {
		candidates, err := s.queries.MusicCompoundReconciliationCandidates(ctx, after)
		if err != nil {
			return err
		}
		if len(candidates) == 0 {
			return ctx.Err()
		}
		for _, candidate := range candidates {
			after = candidate
			err = s.reconcileCompoundCredits(ctx, scan, candidate)
			if err != nil {
				return err
			}
		}
	}
}
