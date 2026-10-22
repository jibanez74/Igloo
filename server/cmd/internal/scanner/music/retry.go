package music

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/scanner"
	spotifyapi "igloo/cmd/internal/spotify"
)

// Retry only persisted catalog metadata. Pages are closed before any requests
// or writes. A tripped breaker ends the pass; what is left retries next scan.
func (s *Scanner) retrySpotify(ctx context.Context, scan *musicScanContext, report *scanReport) error {
	if s.spotify == nil {
		return nil
	}
	stopped := s.noteSpotifyStop(scan, report)
	if stopped {
		return nil
	}
	artists, err := s.queries.CountMusicArtistRetryCandidates(ctx)
	if err != nil {
		return err
	}
	albums, err := s.queries.CountMusicAlbumRetryCandidates(ctx)
	if err != nil {
		return err
	}
	report.status.EnrichmentTotal = int(artists + albums)
	s.publish(report)
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
			contextErr := ctx.Err()
			if contextErr != nil {
				return contextErr
			}
			stopped := s.noteSpotifyStop(scan, report)
			if stopped {
				return nil
			}
			after = candidate.ID
			resolved, err := s.resolveMusician(ctx, scan, candidate.Name, "")
			if err != nil {
				return err
			}
			err = s.persistMusicianEnrichment(ctx, scan, resolved)
			if err != nil {
				return err
			}
			if resolved.splitCompoundOnNoMatch {
				err = s.reconcileCompoundCredits(ctx, scan, candidate.ID)
				if err != nil {
					return err
				}
			}
			report.status.EnrichmentProcessed++
			s.publish(report)
		}
	}
	after = 0
	for {
		candidates, err := s.queries.MusicAlbumRetryCandidates(ctx, after)
		if err != nil {
			return err
		}
		if len(candidates) == 0 {
			break
		}
		for _, candidate := range candidates {
			contextErr := ctx.Err()
			if contextErr != nil {
				return contextErr
			}
			stopped := s.noteSpotifyStop(scan, report)
			if stopped {
				return nil
			}
			after = candidate.ID
			query := albumQuery{title: candidate.Title, sortTitle: candidate.Title, artist: candidate.Musician.String}
			if candidate.Year.Valid {
				query.year = int(candidate.Year.Int64)
			}
			resolved, err := s.resolveAlbum(ctx, scan, query)
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
			if resolved.spotifyAlbum != nil {
				adopted, adoptErr := s.adoptAlbumArtist(ctx, scan, candidate.Musician.String, resolved.spotifyAlbum)
				if adoptErr != nil {
					return adoptErr
				}
				if adopted != nil {
					err = s.persistMusicianEnrichment(ctx, scan, adopted)
					if err != nil {
						return err
					}
				}
			}
			report.status.EnrichmentProcessed++
			s.publish(report)
		}
	}
	err = s.reconcilePendingCompoundCredits(ctx, scan)
	if err != nil {
		return err
	}
	return s.linkAlbumTracks(ctx, scan, report)
}

// persistMusicianEnrichment commits one artist outcome reached outside a
// track transaction and re-votes its sort name.
func (s *Scanner) persistMusicianEnrichment(ctx context.Context, scan *musicScanContext, resolved *resolvedMusician) error {
	return s.persistEnrichment(ctx, scan, func(qtx *database.Queries, txScan *musicScanContext) error {
		id, persistErr := s.persistMusician(ctx, qtx, txScan, *resolved)
		if persistErr != nil {
			return persistErr
		}
		return qtx.ReconcileMusicArtistSort(ctx, id)
	})
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

// linkAlbumTracks pairs the tracks of matched albums that still lack a
// Spotify outcome: tracks persisted before their album matched, files added
// to an album matched in an earlier scan, and pairings a failed request
// skipped. Each album costs at most one request, and only when this scan did
// not match it.
func (s *Scanner) linkAlbumTracks(ctx context.Context, scan *musicScanContext, report *scanReport) error {
	pending, err := s.queries.CountMusicTrackLinkAlbumCandidates(ctx)
	if err != nil {
		return err
	}
	report.status.EnrichmentTotal += int(pending)
	s.publish(report)
	var after int64
	for {
		candidates, err := s.queries.MusicTrackLinkAlbumCandidates(ctx, after)
		if err != nil {
			return err
		}
		if len(candidates) == 0 {
			return nil
		}
		for _, candidate := range candidates {
			contextErr := ctx.Err()
			if contextErr != nil {
				return contextErr
			}
			stopped := s.noteSpotifyStop(scan, report)
			if stopped {
				return nil
			}
			after = candidate.ID
			err = s.linkAlbum(ctx, scan, candidate)
			if err != nil {
				return err
			}
			report.status.EnrichmentProcessed++
			s.publish(report)
		}
	}
}

func (s *Scanner) linkAlbum(ctx context.Context, scan *musicScanContext, candidate database.MusicTrackLinkAlbumCandidatesRow) error {
	tracks, available, err := s.albumTracksFor(ctx, scan, candidate)
	if err != nil || !available {
		return err
	}
	rows, err := s.queries.MusicAlbumTracksToLink(ctx, sql.NullInt64{Int64: candidate.ID, Valid: true})
	if err != nil {
		return err
	}
	return s.persistEnrichment(ctx, scan, func(qtx *database.Queries, txScan *musicScanContext) error {
		for _, row := range rows {
			query := spotifyapi.TrackQuery{Title: row.Title, Disc: int(row.Disc), Number: int(row.TrackIndex), DurationMs: row.Duration}
			link, match := pairTrack(query, tracks)
			if link != nil {
				err := qtx.LinkMusicTrackSpotify(ctx, database.LinkMusicTrackSpotifyParams{SpotifyID: link.spotifyID, Explicit: link.explicit, TrackIndex: link.trackNumber, ID: row.ID})
				if err != nil {
					return err
				}
				txScan.invalidatedTracks[row.ID] = true
				txScan.tracksLinked++
			}
			err := saveMusicSpotifyMatch(ctx, qtx, musicSpotifyEntityTrack, row.ID, &match)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// albumTracksFor returns the album's Spotify track list, from this scan's
// match or a fetch by id. A 404 means the catalog entry is gone, which reads
// as an empty list so the pending tracks are settled rather than refetched
// every scan; any other failure leaves them pending.
func (s *Scanner) albumTracksFor(ctx context.Context, scan *musicScanContext, candidate database.MusicTrackLinkAlbumCandidatesRow) (tracks []spotifyapi.SimpleTrack, available bool, err error) {
	key := scanner.NormalizedScanCacheKey(candidate.Title, candidate.Musician.String)
	tracks, ok := scan.albumTracks[key]
	if ok {
		return tracks, true, nil
	}
	album, fetchErr := s.spotify.GetAlbum(ctx, spotifyapi.ID(candidate.SpotifyID.String))
	contextErr := ctx.Err()
	if contextErr != nil {
		return nil, false, contextErr
	}
	scan.breaker.Observe(fetchErr)
	if fetchErr == nil {
		tracks = album.Tracks
	} else {
		var status *spotifyapi.StatusError
		gone := errors.As(fetchErr, &status) && status.StatusCode == http.StatusNotFound
		if !gone {
			s.logger.Warn("spotify album tracks unavailable", "album_id", candidate.ID, "spotify_id", candidate.SpotifyID.String, "error", fetchErr)
			return nil, false, nil
		}
		tracks = []spotifyapi.SimpleTrack{}
	}
	scan.albumTracks[key] = tracks
	return tracks, true, nil
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
			contextErr := ctx.Err()
			if contextErr != nil {
				return contextErr
			}
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
				for _, id := range uniqueIDs(ids, []int64{musicianID}) {
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
			contextErr := ctx.Err()
			if contextErr != nil {
				return contextErr
			}
			after = candidate
			err = s.reconcileCompoundCredits(ctx, scan, candidate)
			if err != nil {
				return err
			}
		}
	}
}
