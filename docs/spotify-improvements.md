# Spotify client improvements for the music scanner

Suggestions for `server/cmd/internal/spotify`, the in-house Spotify client that replaced `github.com/zmb3/spotify/v2` and `golang.org/x/oauth2` in commit 04c45138. None of them are implemented yet. They are ordered by how much they would help the music scanner.

Any implementation must keep existing behavior working, leave no dead code, stay DRY, and keep every test passing (`make check`, plus `make test-spotify-integration` for the live API tests).

## Background: Spotify API changes in 2026

In February 2026, Spotify restricted apps in Development Mode, which is the default for apps not approved for higher quotas ([migration guide](https://developer.spotify.com/documentation/web-api/tutorials/february-2026-migration-guide)):

- `popularity` was removed from artists, albums and tracks; `followers` from artists; `label` from albums.
- The endpoints that fetch several albums, artists or tracks in one call were removed. Single-item endpoints such as `GET /albums/{id}` and `GET /artists/{id}` remain.
- The search `limit` maximum dropped from 50 to 10, and the default from 20 to 5.
- `external_ids` was removed from tracks and albums, then restored in March 2026.

In July 2026, the request quota became shared across all Development Mode apps under one developer account, and a 429 response body now includes `"reason": "QUOTA_EXCEEDED"` ([quota update](https://developer.spotify.com/blog/2026-07-23-web-api-quota-updates)).

Igloo's Spotify app also gets no genres back, so musician summaries are always empty.

## Suggestions

### 1. Stop the scan when Spotify is failing

**Today:** every Spotify error becomes the reason `search_failed` and the status `failed`, and the scan keeps sending requests. With a revoked secret or an exhausted quota, every remaining artist and album costs one doomed request.

**Precedent:** the movie and show scanners stop sending requests after a login failure, or after 3 transient failures in a row, through `scanner.ProviderBreaker` in `server/cmd/internal/scanner/backoff.go`. That breaker is hardwired to `tmdb.ProviderFailure`.

**Change:**

- Add `spotify.ProviderFailure(err) (authentication, transient bool)`, mirroring the TMDB one.
- Return a typed error from the token endpoint. A 400 `invalid_client` is a login failure, but today it is a plain `fmt` error.
- Decode the 429 `reason` and the `Retry-After` header into `StatusError`. A `QUOTA_EXCEEDED` 429 should stop the scan rather than be retried.
- Give `ProviderBreaker` a classifier function so TMDB and Spotify share it, then use it in `resolveMusician`, `resolveAlbum` and `retrySpotify` (`server/cmd/internal/scanner/music/`).

### 2. Skip the second request per album

**Today:** `SearchAndGetAlbumDetails` runs a search and then `GET /albums/{id}` for the chosen album.

**Why it can go:** for a Development Mode app the details call adds only `genres` (empty for this app) and `popularity` (removed). The search result already carries everything else the scanner stores: ID, name, artists, images, release date and total tracks.

**Change:** return the matched search item directly. This halves album requests.

**Related product decision:** `musicians.spotify_popularity`, `musicians.spotify_followers`, `albums.spotify_popularity` and `generateMusicianSummary` are now always empty. They could be removed; schema edits are fine before v1.

### 3. Link artist and album matches by Spotify ID

**Today:** album matching compares artist *names* fuzzily (`scoreAlbumArtist`), even when the album's artist is already matched to a Spotify ID.

**Change:**

- Album search results include `artists[].id`. When the album artist is already matched (in this scan or in the database), pass that ID in and treat an ID hit as a certain artist match. This fixes same-named bands and helps with "Various Artists".
- In the other direction, when an artist name search falls below the threshold, take the artist ID from an accepted album match and call `GET /artists/{id}`.
- The API would become something like `MatchAlbum(ctx, AlbumQuery{Title, Artist, ArtistID, Year, TrackTotal})`.

### 4. Use the files' own tags to pick the right edition

**Today:** deluxe, remastered and standard editions often score the same on title alone, so the first search result wins.

**Change:** the scanner already reads `Date` and `Track` (the "n/total" format) in `ffprobe.FormatTags`. In `selectBestAlbumMatch`, add points when the year agrees with `release_date` and when the track total agrees with `total_tracks`.

### 5. Retry misses with a growing wait, as movies and shows do

**Today:**

- `unmatched` is permanent for music, so an album that Spotify adds later is never matched.
- `failed` is retried on every scan with no wait.

**Change:** add `attempts` and `last_attempt_at` to `music_spotify_matches`, then apply `scanner.MissBackoffElapsed` (already used by movies and shows) to the retry-candidate queries in `server/sqlc/queries/music_metadata.sql`.

### 6. Space out requests

**Today:** requests go out as fast as the scanner produces them, and each one is tried once.

**Why it matters:** the quota is now shared across all your apps, so a large first scan can use it all up.

**Change:**

- Add a small minimum gap between requests, using only the standard library.
- Optionally, add one capped retry that waits for the time in `Retry-After`, as `tmdb.retryDelay` does. This reverses the earlier "single attempt" decision, so it needs your approval.

### 7. Remove the client's own cache

**Today:** `spotify_cache.go` keeps a 15-minute go-cache of successful artist and album lookups, keyed by lowercased strings.

**Why it is redundant:**

- The scanner already avoids repeat lookups within a scan (`artistAttempts`, `spotifyArtistMisses`, `spotifyAlbumMisses`, `musicianIDs`, keyed by `NormalizedScanCacheKey`) and saves outcomes in `music_spotify_matches`.
- The `/api/spotify` search endpoints don't use the cache.
- `ClearAllCaches` runs only at shutdown, when the process is exiting anyway.

**Change:** delete the cache and remove `ClearAllCaches` from the interface and from `shutdown.go`.

### 8. Smaller interfaces

**Today:** `SpotifyInterface` mixes the scanner's matching methods with the handler's search methods, so every fake implements all five methods.

**Change:** the music scanner declares its own two-method interface, and `spotify_handler.go` declares its own. Each consumer states exactly what it needs, and the test fakes get smaller.

### 9. Record why a match failed (lower priority)

**Today:** `music_spotify_matches` stores only `status` and `reason`. The candidate name and score are logged in `MatchDebugInfo` and then lost.

**Change:** store the best candidate's name and score next to the reason. A bad miss can then be explained, and the thresholds (`spotifyArtistThreshold` = 78, `spotifyAlbumThreshold` = 76) can be tuned from real data.

### 10. Match by ISRC (lower priority, very precise)

An ISRC is a standard code that identifies one recording, and many files carry it as a tag.

**Change:** add `isrc` to `ffprobe.FormatTags`. When a file has one, run an `isrc:` track search first. It returns the exact track, album and artist IDs, and fuzzy search becomes the fallback.

## Suggested PR grouping

| PR | Items | Theme |
| --- | --- | --- |
| A | 1, 6 | Reliability: stop on failure, respect the quota |
| B | 2, 7, 8 | Simplification: fewer requests, less code |
| C | 3, 4 | Match quality |
| D | 5, optionally 9 | Retries and diagnostics |

Verify each PR with:

- `make check`;
- `make test-spotify-integration`;
- a before/after comparison of a real scan against the live API.
