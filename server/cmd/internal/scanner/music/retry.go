package music

import (
	"context"
	"database/sql"
	"strconv"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/scanner"
	spotifyapi "igloo/cmd/internal/spotify"
)

// retrySpotify retries the catalog, then pairs the tracks of matched albums.
// The pairing runs even after the breaker trips, since the albums this scan
// matched brought their track lists along.
func (s *Scanner) retrySpotify(ctx context.Context, scan *musicScanContext, report *scanReport) error {
	if s.spotify == nil {
		return nil
	}
	err := s.retryCatalog(ctx, scan, report)
	if err != nil {
		return err
	}
	return s.linkAlbumTracks(ctx, scan, report)
}

// Retry only persisted catalog metadata. Pages are closed before any requests
// or writes. A tripped breaker ends the pass; what is left retries next scan.
func (s *Scanner) retryCatalog(ctx context.Context, scan *musicScanContext, report *scanReport) error {
	stopped := s.noteSpotifyStop(scan, report)
	if stopped {
		return nil
	}
	artists, err := s.queries.CountMusicArtistRetryCandidates(ctx)
	if err != nil {
		return err
	}
	albums, err := s.queries.CountMusicAlbumRetryCandidates(ctx, scan.appleEnabled)
	if err != nil {
		return err
	}
	report.status.EnrichmentTotal += int(artists + albums)
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
		candidates, err := s.queries.MusicAlbumRetryCandidates(ctx, database.MusicAlbumRetryCandidatesParams{AfterID: after, AppleEnabled: scan.appleEnabled})
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
			var adopted *resolvedMusician
			if resolved.spotifyAlbum != nil {
				adopted, err = s.adoptAlbumArtist(ctx, scan, candidate.Musician.String, resolved.spotifyAlbum)
				if err != nil {
					return err
				}
			}
			err = s.persistEnrichment(ctx, scan, func(qtx *database.Queries, txScan *musicScanContext) error {
				if adopted != nil {
					persistErr := s.persistEnrichedMusician(ctx, qtx, txScan, *adopted)
					if persistErr != nil {
						return persistErr
					}
				}
				id, persistErr := s.persistAlbum(ctx, qtx, txScan, *resolved)
				if persistErr != nil {
					return persistErr
				}
				return reconcileAlbum(ctx, qtx, id)
			})
			if err != nil {
				return err
			}

			report.status.EnrichmentProcessed++
			s.publish(report)
		}
	}
	return s.reconcilePendingCompoundCredits(ctx, scan)
}

// persistMusicianEnrichment commits one artist outcome reached outside a
// track transaction and re-votes its sort name.
func (s *Scanner) persistMusicianEnrichment(ctx context.Context, scan *musicScanContext, resolved *resolvedMusician) error {
	return s.persistEnrichment(ctx, scan, func(qtx *database.Queries, txScan *musicScanContext) error {
		return s.persistEnrichedMusician(ctx, qtx, txScan, *resolved)
	})
}

// persistEnrichedMusician saves an artist outcome inside a transaction and
// re-votes its sort name.
func (s *Scanner) persistEnrichedMusician(ctx context.Context, qtx *database.Queries, txScan *musicScanContext, resolved resolvedMusician) error {
	id, err := s.persistMusician(ctx, qtx, txScan, resolved)
	if err != nil {
		return err
	}
	return qtx.ReconcileMusicArtistSort(ctx, id)
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
// Spotify outcome: every track of an album matched this scan, files added to
// an album matched in an earlier scan, and pairings a failed request skipped.
// Each album costs at most one request, and only when this scan did not match
// it.
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
			break
		}
		for _, candidate := range candidates {
			contextErr := ctx.Err()
			if contextErr != nil {
				return contextErr
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
	// A breaker tripped by a track-list fetch stops only the fetches.
	s.noteSpotifyStop(scan, report)
	return nil
}

// linkAlbum pairs an album's pending tracks with its Spotify track list in one
// pass, so each Spotify track goes to one file, and drops the album's match
// when its tracks contradict it.
func (s *Scanner) linkAlbum(ctx context.Context, scan *musicScanContext, candidate database.MusicTrackLinkAlbumCandidatesRow) error {
	tracks, available, err := s.albumTracksFor(ctx, scan, candidate)
	if err != nil || !available {
		return err
	}
	rows, err := s.queries.MusicAlbumTracksToLink(ctx, sql.NullInt64{Int64: candidate.ID, Valid: true})
	if err != nil {
		return err
	}
	var pending []database.MusicAlbumTracksToLinkRow
	var queries []spotifyapi.TrackQuery
	var held []spotifyapi.HeldTrack
	for _, row := range rows {
		query := spotifyapi.TrackQuery{Title: row.Title, Disc: int(row.Disc), Number: int(row.TrackIndex), DurationMs: row.Duration}
		switch {
		case !row.Status.Valid || row.Status.String == musicStatusFailed:
			pending = append(pending, row)
			queries = append(queries, query)
		case row.SpotifyID.Valid:
			held = append(held, spotifyapi.HeldTrack{TrackQuery: query, ID: spotifyapi.ID(row.SpotifyID.String)})
		}
	}
	pairings := spotifyapi.MatchAlbumTracks(queries, held, tracks)
	contradicted := albumContradicted(len(rows), held, pairings, tracks)
	err = s.persistEnrichment(ctx, scan, func(qtx *database.Queries, txScan *musicScanContext) error {
		if contradicted {
			return dropAlbumMatch(ctx, qtx, txScan, candidate.ID, rows)
		}
		for i, row := range pending {
			pairing := pairings[i]
			match := resolvedMatch{status: musicStatusMatched}
			if pairing.Track == nil {
				match = resolvedSpotifyMatchFromInfo(pairing.Info)
			} else {
				err := qtx.LinkMusicTrackSpotify(ctx, database.LinkMusicTrackSpotifyParams{SpotifyID: sql.NullString{String: pairing.Track.ID.String(), Valid: true}, Explicit: pairing.Track.Explicit, TrackIndex: int64(pairing.Track.TrackNumber), ID: row.ID})
				if err != nil {
					return err
				}
				txScan.invalidatedTracks[row.ID] = true
				txScan.tracksLinked++
			}
			err := saveMusicMatch(ctx, qtx, musicProviderSpotify, musicEntityTrack, row.ID, &match)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil || !contradicted {
		return err
	}
	return s.recountContradictedAlbum(ctx, scan, candidate.ID)
}

// albumContradicted reports an album match its tracks disagree with: fewer
// than half of at least albumCheckMinTracks local tracks pair, and the album
// is not a bigger local edition holding every Spotify track. A Spotify release
// shorter than albumCheckMinTracks is a single, not an edition of the album.
func albumContradicted(local int, held []spotifyapi.HeldTrack, pairings []spotifyapi.TrackPairing, tracks []spotifyapi.SimpleTrack) bool {
	paired := len(held)
	taken := make(map[spotifyapi.ID]bool, len(held)+len(pairings))
	for _, holder := range held {
		taken[holder.ID] = true
	}
	for _, pairing := range pairings {
		if pairing.Track != nil {
			paired++
			taken[pairing.Track.ID] = true
		}
	}
	if local < albumCheckMinTracks || paired*2 >= local {
		return false
	}
	if len(tracks) < albumCheckMinTracks {
		return true
	}
	for _, track := range tracks {
		if !taken[track.ID] {
			return true
		}
	}
	return false
}

// dropAlbumMatch undoes an album match its tracks contradicted: what the match
// stored on the album and its tracks goes, and the album is recorded as
// unmatched for good.
func dropAlbumMatch(ctx context.Context, qtx *database.Queries, txScan *musicScanContext, albumID int64, rows []database.MusicAlbumTracksToLinkRow) error {
	tracksAlbum := sql.NullInt64{Int64: albumID, Valid: true}
	err := qtx.ClearMusicAlbumSpotify(ctx, albumID)
	if err != nil {
		return err
	}
	err = qtx.ClearMusicAlbumSpotifyDate(ctx, albumID)
	if err != nil {
		return err
	}
	err = qtx.ClearMusicAlbumTrackLinks(ctx, tracksAlbum)
	if err != nil {
		return err
	}
	err = qtx.DeleteMusicAlbumTrackMatches(ctx, tracksAlbum)
	if err != nil {
		return err
	}
	err = reconcileAlbum(ctx, qtx, albumID)
	if err != nil {
		return err
	}
	for _, row := range rows {
		txScan.invalidatedTracks[row.ID] = true
	}
	return saveMusicMatch(ctx, qtx, musicProviderSpotify, musicEntityAlbum, albumID, &resolvedMatch{
		status: musicStatusUnmatched,
		reason: sql.NullString{String: musicSpotifyReasonTracksDisagree, Valid: true},
	})
}

// recountContradictedAlbum moves every tally this scan made for the album to
// unmatched: the one under its id, and those under each alias name it was
// resolved by before persistence merged it into this row. An album matched by
// an earlier scan was not tallied, so it counts as one unmatched outcome.
func (s *Scanner) recountContradictedAlbum(ctx context.Context, scan *musicScanContext, albumID int64) error {
	aliases, err := s.queries.MusicAlbumIdentityKeys(ctx, albumID)
	if err != nil {
		return err
	}
	idKey := musicEntityAlbum + ":" + strconv.FormatInt(albumID, 10)
	keys := []string{idKey}
	for _, alias := range aliases {
		keys = append(keys, musicEntityAlbum+":"+scanner.NormalizedScanCacheKey(alias.TitleKey, alias.ArtistKey))
	}
	recounted := false
	for _, key := range keys {
		_, counted := scan.enrichmentOutcome[key]
		if counted {
			scan.recountEnrichment(key, musicStatusUnmatched)
			recounted = true
		}
	}
	if !recounted {
		scan.recountEnrichment(idKey, musicStatusUnmatched)
	}
	return nil
}

// albumTracksFor returns the album's Spotify track list, from this scan's
// match or a fetch by id. A 404 means the catalog entry is gone, which reads
// as an empty list so the pending tracks are settled rather than refetched
// every scan; any other failure leaves them pending.
func (s *Scanner) albumTracksFor(ctx context.Context, scan *musicScanContext, candidate database.MusicTrackLinkAlbumCandidatesRow) (tracks []spotifyapi.SimpleTrack, available bool, err error) {
	key := candidate.SpotifyID.String
	tracks, ok := scan.albumTracks[key]
	if ok {
		return tracks, true, nil
	}
	if scan.albumTrackFailures[key] || !s.spotifyReady(scan) {
		return nil, false, nil
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
		gone := spotifyNotFound(fetchErr)
		if !gone {
			scan.albumTrackFailures[key] = true
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
