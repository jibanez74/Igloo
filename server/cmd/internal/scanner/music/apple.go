package music

import (
	"context"
	"database/sql"
	"errors"
	"maps"
	"slices"
	"strconv"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/itunes"
	"igloo/cmd/internal/scanner"
)

// appleAlbum is an Apple pass candidate: the local album, the Apple collection
// ids its files carry, the one most of them carry first, and the collection
// the album already matched, zero until it has.
type appleAlbum struct {
	id          int64
	collections []int64
	matched     int64
}

// enrichFromApple settles the albums whose files carry Apple collection ids,
// a page of candidates at a time: it asks Apple for the page's collections,
// up to itunes.MaxLookupIDs per request and each at most once per scan, then
// settles each album in its own transaction. An album with a collection Apple
// did not answer for waits for the next scan; a tripped breaker ends the pass.
func (s *Scanner) enrichFromApple(ctx context.Context, scan *musicScanContext, report *scanReport) error {
	if !s.appleReady(scan) {
		return nil
	}
	total, err := s.queries.CountMusicAppleAlbumCandidates(ctx)
	if err != nil {
		return err
	}
	report.status.EnrichmentTotal += int(total)
	s.publish(report)
	// answers holds Apple's answer for every collection asked about this
	// scan: the collection, or nil when Apple no longer lists it.
	answers := make(map[int64]*itunes.Album)
	var after int64
	for !scan.appleBreaker.Stopped {
		rows, err := s.queries.MusicAppleAlbumCandidates(ctx, after)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			break
		}
		albums := make([]appleAlbum, 0, len(rows))
		queued := make(map[int64]bool)
		var unasked []int64
		for _, row := range rows {
			after = row.ID
			collections, err := s.queries.MusicAlbumAppleCollections(ctx, sql.NullInt64{Int64: row.ID, Valid: true})
			if err != nil {
				return err
			}
			album := appleAlbum{id: row.ID, matched: row.AppleID.Int64}
			for _, collection := range collections {
				id := collection.Int64
				album.collections = append(album.collections, id)
				_, asked := answers[id]
				if !asked && !queued[id] {
					queued[id] = true
					unasked = append(unasked, id)
				}
			}
			albums = append(albums, album)
		}
		err = s.askAppleForCollections(ctx, scan, unasked, answers)
		if err != nil {
			return err
		}
		for _, album := range albums {
			contextErr := ctx.Err()
			if contextErr != nil {
				return contextErr
			}
			err = s.settleAppleAlbum(ctx, scan, album, answers)
			if err != nil {
				return err
			}
			report.status.EnrichmentProcessed++
			s.publish(report)
		}
	}
	s.noteAppleStop(scan, report)
	return nil
}

// askAppleForCollections records Apple's answer for each collection id. A
// failed request leaves its ids unanswered.
func (s *Scanner) askAppleForCollections(ctx context.Context, scan *musicScanContext, ids []int64, answers map[int64]*itunes.Album) error {
	for chunk := range slices.Chunk(ids, itunes.MaxLookupIDs) {
		if scan.appleBreaker.Stopped {
			return nil
		}
		albums, err := s.apple.LookupAlbums(ctx, chunk)
		contextErr := ctx.Err()
		if contextErr != nil {
			return contextErr
		}
		scan.appleBreaker.Observe(err)
		if err != nil {
			s.logger.Warn("apple music lookup failed", "collections", len(chunk), "error", err)
			continue
		}
		for _, id := range chunk {
			album, listed := albums[id]
			if listed {
				answers[id] = &album
			} else {
				answers[id] = nil
			}
		}
	}
	return nil
}

// settleAppleAlbum records Apple's answer for one album. Its collection is the
// one most of its files carry, or the one it already matched. Its pending
// tracks pair by the Apple track id each file carries, against every listed
// collection the album's files name. An album whose collection Apple no
// longer lists is handed to Spotify.
func (s *Scanner) settleAppleAlbum(ctx context.Context, scan *musicScanContext, album appleAlbum, answers map[int64]*itunes.Album) error {
	countKey := musicEntityAlbum + ":" + strconv.FormatInt(album.id, 10)
	for _, id := range album.collections {
		_, answered := answers[id]
		if !answered {
			if album.matched == 0 {
				scan.countEnrichment(countKey, &resolvedMatch{status: musicStatusFailed})
			}
			return nil
		}
	}
	var collection *itunes.Album
	if album.matched == 0 {
		collection = answers[album.collections[0]]
		if collection == nil {
			return s.handAlbumToSpotify(ctx, scan, album.id, countKey)
		}
	}

	owner := album.id
	err := s.persistEnrichment(ctx, scan, func(qtx *database.Queries, txScan *musicScanContext) error {
		if collection != nil {
			var claimErr error
			owner, claimErr = s.claimAppleAlbum(ctx, qtx, txScan, album.id, *collection)
			if claimErr != nil {
				return claimErr
			}
		}
		collections, err := qtx.MusicAlbumAppleCollections(ctx, sql.NullInt64{Int64: owner, Valid: true})
		if err != nil {
			return err
		}
		catalog := make(map[int64]itunes.Track)
		for _, id := range collections {
			answer, answered := answers[id.Int64]
			if !answered {
				// Keep the merge, but leave tracks pending until every owner
				// collection is answered by a later candidate or scan.
				return nil
			}
			if answer != nil {
				maps.Copy(catalog, answer.Tracks)
			}
		}
		return pairAppleTracks(ctx, qtx, txScan, owner, catalog)
	})
	if err != nil {
		return err
	}
	if collection != nil {
		scan.countEnrichment(musicEntityAlbum+":"+strconv.FormatInt(owner, 10), &resolvedMatch{status: musicStatusMatched})
	}
	return nil
}

// claimAppleAlbum matches the album to an Apple collection and returns the
// album that holds it: a collection another local album already holds merges
// this album into that one.
func (s *Scanner) claimAppleAlbum(ctx context.Context, qtx *database.Queries, txScan *musicScanContext, albumID int64, collection itunes.Album) (int64, error) {
	appleID := sql.NullInt64{Int64: collection.ID, Valid: true}
	owner, err := qtx.GetAlbumByAppleID(ctx, appleID)
	missing := errors.Is(err, sql.ErrNoRows)
	if missing {
		owner = albumID
		err = qtx.SetMusicAlbumAppleID(ctx, database.SetMusicAlbumAppleIDParams{AppleID: appleID, ID: albumID})
	}
	if err != nil {
		return 0, err
	}
	if owner != albumID {
		err = s.mergeMusicAlbum(ctx, qtx, txScan, albumID, owner)
		if err != nil {
			return 0, err
		}
	}
	err = qtx.UpdateMusicAlbumEnrichment(ctx, database.UpdateMusicAlbumEnrichmentParams{
		ID:          owner,
		TotalTracks: helpers.NullInt64(int64(collection.TrackCount)),
		AlbumType:   albumTypeValue(collection.Type()),
	})
	if err != nil {
		return 0, err
	}
	cover := collection.CoverURL()
	if cover != "" {
		_, err = qtx.UpdateAlbumCover(ctx, database.UpdateAlbumCoverParams{ID: owner, Cover: helpers.NullString(cover)})
		if err != nil {
			return 0, err
		}
	}
	return owner, saveMusicMatch(ctx, qtx, musicProviderApple, musicEntityAlbum, owner, &resolvedMatch{status: musicStatusMatched})
}

// pairAppleTracks settles the album's tracks that have no outcome: a file
// whose Apple track id a listed collection holds is that catalog track, and
// any other file is unmatched.
func pairAppleTracks(ctx context.Context, qtx *database.Queries, txScan *musicScanContext, albumID int64, catalog map[int64]itunes.Track) error {
	rows, err := qtx.MusicAlbumTracksToPairApple(ctx, sql.NullInt64{Int64: albumID, Valid: true})
	if err != nil {
		return err
	}
	for _, row := range rows {
		match := resolvedMatch{status: musicStatusUnmatched}
		if row.AppleTrackID.Valid {
			match.reason = sql.NullString{String: musicAppleReasonNotListed, Valid: true}
		}
		track, listed := catalog[row.AppleTrackID.Int64]
		if listed {
			err = qtx.LinkMusicTrackApple(ctx, database.LinkMusicTrackAppleParams{AppleID: row.AppleTrackID, Explicit: track.Explicit, ID: row.ID})
			if err != nil {
				return err
			}
			txScan.invalidatedTracks[row.ID] = true
			txScan.tracksLinked++
			match = resolvedMatch{status: musicStatusMatched}
		}
		err = saveMusicMatch(ctx, qtx, musicProviderApple, musicEntityTrack, row.ID, &match)
		if err != nil {
			return err
		}
	}
	return nil
}

// handAlbumToSpotify records that Apple no longer lists the album's
// collection, which leaves the album for Spotify to search. When Spotify
// cannot search it this scan, the album counts as unmatched here.
func (s *Scanner) handAlbumToSpotify(ctx context.Context, scan *musicScanContext, albumID int64, countKey string) error {
	match := &resolvedMatch{status: musicStatusUnmatched, reason: sql.NullString{String: musicAppleReasonNotListed, Valid: true}}
	err := s.persistEnrichment(ctx, scan, func(qtx *database.Queries, _ *musicScanContext) error {
		return saveMusicMatch(ctx, qtx, musicProviderApple, musicEntityAlbum, albumID, match)
	})
	if err != nil {
		return err
	}
	// The walk cached the album's id under each name it was resolved by, and
	// a cached id reads as already resolved, so the Spotify retry would skip
	// it. Dropping the cache costs that retry one identity read per album.
	scan.albumIDs = scanner.NewScanCache[string, int64]()
	if !s.spotifyReady(scan) {
		scan.countEnrichment(countKey, match)
	}
	return nil
}

// appleReady reports whether lookups may still go to Apple this scan.
func (s *Scanner) appleReady(scan *musicScanContext) bool {
	return scan.appleEnabled && !scan.appleBreaker.Stopped
}

// noteAppleStop reports the Apple breaker tripping once per scan.
func (s *Scanner) noteAppleStop(scan *musicScanContext, report *scanReport) {
	s.noteProviderStop(report, scan.appleBreaker, &scan.appleStopReported, musicProviderApple, reasonAppleStopped)
}
