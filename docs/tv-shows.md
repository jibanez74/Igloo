# TV show details page specification

Igloo scans a local TV catalog but does not read it back: one route, `GET /api/shows/latest`, serves Home's recently-added rail, and `/tv-shows/$id` renders a placeholder, so every scanned show dead-ends. This document specifies a read-only show details page — hero, seasons, episodes, credits, and descriptive metadata — over the catalog the scanner already persists.

Show identity is the show row id. Responses never expose `directory_path` or `local_name`. TMDB totals stay separate from local availability derived from file links, as [media behavior](ffmpeg.md#tv-show-scanning) requires. The [API contract](openapi.json) and the [design system](design-system.md) remain authoritative, and both are updated in the same task as the implementation they describe.

The phases below are implemented; see Implementation status. [Scanner behavior](ffmpeg.md#tv-show-scanning) is unchanged by this work.

## Phase 1 — SQL queries

Do not modify the schema. Every column the page needs already exists, so `server/sqlc/schema.sql` is untouched, no dev database needs an `ALTER`, and no migration question arises. Add read queries to `server/sqlc/queries/shows.sql` and run `make generate`. Every query there today serves the scanner: the few that read catalog rows — `GetShow`, `GetShowSeasons`, `GetShowEpisodes` — select whole rows keyed by internal ids, so they are neither addressable from a URL nor safe to hand to a client.

Add `GetShowDetails` as an explicit projection mirroring `GetMovieDetails`: `id, name, original_name, premiere_year, tmdb_id, overview, tagline, language, origin_countries, first_air_date, last_air_date, status, type, poster_path, backdrop_path, vote_average, vote_count, certification, tmdb_season_count, tmdb_episode_count`. Exclude `directory_path` and `local_name`, which are filesystem detail, along with `adult`, `imdb_id`, `homepage`, `popularity`, and the timestamps, which the page does not render.

Add `GetShowSeasonSummaries`, joining `show_seasons` to `show_episodes` and counting episodes per season as `available_episode_count`, ordered `season_number = 0, season_number` so specials sort last rather than first. Add `GetShowSeasonSummaryByNumber` for the same aggregate over one `(show_id, season_number)` pair, and `GetShowEpisodesBySeasonNumber`, which joins `show_seasons` so a season is addressed by its number rather than by the internal season id, ordered by `episode_number`. Pruning guarantees that a stored episode always has at least one file, so the episode count is the available count.

Add `GetCastByShowID` and `GetCrewByShowID`, joining `artist` for the name and profile path. `show_cast` and `show_crew` have composite primary keys and no row id, so project `credit_id` as the stable row identity alongside `artist_id`, `character` or `department`/`job`, `cast_order`, and `episode_count`. Order cast by `cast_order` then name, and crew by `department` then `job`, matching the movie queries. Add `GetCreatorsByShowID`, `GetGenresByShowID`, `GetNetworksByShowID` — the only query in the project that reads `networks`, including its logo and country — `GetProductionCompaniesByShowID`, and `GetShowExtraVideos`.

Add no indexes. Every filter rides an existing key prefix: `show_seasons` is unique on `(show_id, season_number)`, `show_episodes` on `(season_id, episode_number)`, and the credit and taxonomy tables lead their composite primary keys with `show_id`. Confirm each plan with `EXPLAIN QUERY PLAN` executed through `python3`, not the `sqlite3` binary, which this checkout does not provide.

## Phase 2 — HTTP API

Add two authenticated, non-admin routes to `registerShowRoutes` in `server/cmd/api/routes.go` and two handlers to `server/cmd/api/show_handler.go`. Each handler parses its path parameters with `strconv.ParseInt` and rejects failures with `400`, then opens one read-only transaction with `defer tx.Rollback()` and reads every collection through `app.Queries.WithTx(tx)`, so a page's payload is one consistent snapshot. This is `GetMovieDetails` exactly. Assign results and errors before the conditions that test them, keep internal detail out of client messages, and log the underlying error separately.

`GET /api/shows/details/{id}` returns `show`, `seasons`, `cast`, `crew`, `creators`, `genres`, `networks`, `production_companies`, and `extra_videos`, answering `400` for an unparsable id, `404` for an unknown show, and `500` for a query failure. The path segment order matches the movie analogue `/api/movies/details/{id}`.

`GET /api/shows/{id}/seasons/{seasonNumber}/episodes` returns `season` and `episodes` for one season, answering `404` when either the show or that season number does not exist. Seasons are fetched per selection rather than embedded in the details payload: a long-running show holds hundreds of episodes with overview text, and the page renders one season at a time.

Update `docs/openapi.json` in the same task. Follow the conforming TV precedent, `LatestShowsEnvelope` over the named `LatestShowsData`, and not `MovieDetailsEnvelope`, which inlines its `data` and types its collections as open objects against the rule in [OpenAPI maintenance](openapi-maintenance.md). Reuse the existing `TV Shows` tag. Add the parameter `SeasonNumberPath` with `minimum: 0`, because specials are season zero. Add the responses `ShowDetailsResponse` and `ShowSeasonEpisodesResponse`, the envelopes `ShowDetailsEnvelope` and `ShowSeasonEpisodesEnvelope` — each `allOf` over `JsonSuccess` plus an open branch whose `data` is a `$ref`, so `openapi_composition_test.go` passes — and the closed payload schemas `ShowDetailsData`, `ShowSeasonEpisodesData`, `Show`, `ShowSeasonSummary`, `ShowEpisode`, `ShowCastCredit`, `ShowCrewCredit`, `ShowPerson`, `ShowGenre`, `ShowNetwork`, `ShowProductionCompany`, and `ExtraVideo`. Nullable columns are `$ref`s to `SqlNullString`, `SqlNullInt64`, and `SqlNullFloat64`, as the existing `LatestShow` schema does.

`ShowSeasonSummary` carries `available_episode_count` beside TMDB's `tmdb_episode_count` so the page can state how much of a season is present; the local season count is the length of `seasons`. `ShowEpisode` is `id, episode_number, name, overview, air_date, still_path, tmdb_runtime, vote_average, vote_count` and carries no file, stream, codec, or chapter data.

Correct the contract text that this work makes false: the `triggerShowScan` description ends "TV browsing, playback, and manual identification are not provided."

## Phase 3 — Web client

Alias the new schemas in `web/src/types/shows.ts`, which holds two aliases today, and re-export them from `web/src/types/index.ts`. Alias the named `*Data` payload schemas directly; never index an envelope, which inherits `JsonSuccess.data`'s open index signature and silently disables property checking. Add `getShowDetails` and `getShowSeasonEpisodes` to `web/src/lib/api.ts` on the existing `apiRequest` helper, which never rejects — check the envelope rather than wrapping the call in `try`/`catch`. Add `showDetailsQueryOpts(id)` and `showSeasonEpisodesQueryOpts(showId, seasonNumber)` to `web/src/lib/query-opts.ts` with `STALE_LIST`/`GC_DEFAULT` and the existing `enabled` guards, and register `SHOW_DETAILS_KEY` and `SHOW_SEASON_EPISODES_KEY` in `web/src/lib/constants.ts`.

Add `showDetailsSearchSchema` to `web/src/lib/route-search.ts`: one optional non-negative integer `season`, wrapped in `z.catch` like `moviesSearchSchema`, so a malformed URL degrades to the default season instead of failing the route. The selected season is URL state, never component state.

Replace the placeholder in `web/src/routes/_auth/tv-shows/$id.tsx`. Keep it a flat route file; movies split into `$id/index.tsx` only because they gained a `play.tsx` sibling, and this page does not. The route declares `validateSearch` and `loaderDeps`, then awaits the details query, resolves the season as the search parameter or the first season in the returned order, and awaits that season's episodes, so the page paints complete. Gating mirrors the movie route: an error or a failed envelope renders `MediaNotFound` with `backTo="/"` and "Back to Home", since `/tv-shows` is still a placeholder; a pending query renders the skeleton; a missing payload renders a not-found heading. React 19 document metadata lives inside the article element, titled with the show name and premiere year and described by the first 160 characters of the overview.

Give `ShowCard` detail-query prefetching on `onMouseEnter` and `onFocus` and delete its comment about having no detail query. Media cards prefetch their detail query once one exists.

Generalize three components rather than duplicating them, updating the movie call sites in the same change. `MovieDetailsBackdrop` is already generic over a backdrop URL and becomes `components/shared/DetailBackdrop.tsx`. `MovieExtraVideosSection` becomes `components/shared/ExtraVideosSection.tsx` taking a required `returnTo` in place of `movieId` plus the optional override it has now. `CastSection` becomes `components/shared/CastSection.tsx` and takes items carrying an explicit string `key` and an optional episode count: the movie mapper passes the cast row id, while a show must pass `credit_id`, because TMDB aggregate credits can give one artist several roles and keying on the artist id would duplicate React keys. The optional episode count renders the per-actor episode tally that only shows have.

Add the page's own components under `web/src/components/shows/`, mirroring the `MovieDetails*` set and reusing the shared detail-hero, scrim, content-enter, and detail-list class constants, the `Badge` and button primitives, the rating-tier helpers, the nullable unwrappers, the formatters, `buildTmdbImageUrl`, and `usePosterFallback`.

`ShowDetailsHero` renders the backdrop, the poster that appears only below `lg` with a `Tv` fallback, the page heading, the tagline, the genre list, and a metadata slot. It has no actions slot, because there is nothing to play. `ShowDetailsMetadataChips` renders the TMDB score, certification, series status, air-date range, and season and episode availability as list items following the established chip recipe: the spoken value in an `sr-only` span and the formatted value marked `aria-hidden`, never an `aria-label` on the list item itself.

`ShowSeasonSelector` is a shadcn `Tabs` control using the library tabs-list and trigger classes, horizontally scrollable for a long run, with every trigger fully named — "Season 3", "Specials" — and a change handler that navigates with `replace: true` so the season lands in the URL without stacking history entries. `ShowSeasonEpisodeList` renders the selected season inside the shared detail-list frame with divided rows in the album track-list idiom: episode number, a sixteen-by-nine still in a muted aspect box, name, TMDB runtime, air date, and a two-line overview. It owns its own states, because its query is separate from the page's: a skeleton whose row geometry matches the real rows, a minimal empty state, and the shared inline load error with a retry that refetches.

`ShowCrewSection` bills creators first, then the aggregate crew behind a labelled "Show all crew" disclosure, mirroring the movie key-crew section. `ShowAboutSection` is a description list of original name, status, type, original language, first and last air date, networks with their logos, and production companies. `ShowDetailsSkipLinks` and `ShowDetailsSkeleton` follow the movie pattern: a screen-reader-only skip navigation that becomes visible on focus, and a skeleton built from the same hero classes so arrival shifts nothing.

Add `TMDB_STILL_SIZE` and `TMDB_LOGO_SIZE` to the client constants. The image proxy accepts only `original`, `w1280`, `w500`, `w185`, and `w92`, so stills use `w500` and network logos `w92`; the design system already documents a logo constant that does not exist yet. Never request `image.tmdb.org` from the client.

Accessibility is part of the work, not a later pass: every control carries an accessible name, the season tabs are operable from the keyboard with visible focus, no affordance is hover-only, availability is never communicated by color alone, and the heading hierarchy runs from the single page heading through each section's heading.

## Phase 4 — Tests and documentation

Cover the server in `server/cmd/api/show_handler_test.go`, built on `setupTestApp` and seeded through the scanner's own upserts so the fixtures cannot drift from what a scan produces. Every operation with a JSON success response must call `assertOpenAPIExchange` from a passing focused test, or the unfiltered API package run fails at completion. Cover an unparsable id, an unknown show, an unknown season number, an unauthenticated request, specials ordering last, and a season whose TMDB episode count exceeds the episodes actually present.

Cover the client in `web/src/test/shows/`, which does not exist yet; unit tests are not colocated. A route test built on the shared `renderRoute` helper and a fetch stub that fails unexpected URLs asserts the default season, a season switch, the three-stage entrance stagger through the shared motion helper, and the not-found path. Component tests for the metadata chips and the episode list follow the existing chip test.

Add `web/e2e/tv-show-details.spec.ts` with a per-spec route mock that collects unexpected API requests and asserts the collection is empty, and add a show-details fixture and the two new route branches to `handleShowsRoutes` in `web/e2e/mock-api-server.ts`. A page-level call that is not stubbed hangs a mocked spec. Run Playwright on `E2E_MOCK_API_PORT` rather than the default port, and stub the unread-notification poll.

Update the documents this work makes stale, in the same task: the API contract as described above; the deferral sentences that say TV browsing is not provided, in [media behavior](ffmpeg.md#tv-show-scanning) and in the scan-trigger description; the two statements in `README.md`; and the [design system](design-system.md), for the three moved component names and one sentence naming the season-selector and episode-list patterns.

## Scope and defaults

This page plays nothing. There is no show stream, HLS, subtitle, keyframe-index, or remux-safety route, no show watch-progress table or endpoint, no resume, and no play route; `docs/ffmpeg.md` therefore describes no new media behavior. The episode payload carries no file, stream, codec, or chapter data. That is deliberate rather than an oversight: technical metadata belongs to a physical file, files are season-scoped, one combined file can back several episodes, and file duration is never divided into guessed episode runtimes, so presenting per-episode technical data requires decisions that belong with the playback work.

There is no TV library index, genre browse, search, pagination, like, playlist, or watch-room support. `/tv-shows` stays a placeholder and the page's back link points at Home. There are no administrative actions: no identify, no metadata edit, no delete. Numbering is standard TMDB numbering; alternate and DVD ordering remain deferred. The schema does not change, no migration is added, and `GetLatestShows` keeps its fixed limit of twelve.

## Verification

Run `make generate` for the new queries. From `server/`, run the API package unfiltered — `CGO_ENABLED=1 go test -count=1 -tags "externalbin sqlite_fts5" ./cmd/api` — so the contract-coverage check at package completion actually runs; a filtered `-run` invocation skips it. Run `make test-openapi` for lint and route coverage, then `make generate-openapi` and `make check-openapi` with the regenerated types staged, since the drift check compares against the git index. From `web/`, run the new unit tests and the new Playwright spec. Finish with `make check`, which is required for any server change.

Then exercise the page against a real `make dev` instance holding a scanned sample library, not only the mocks: keyboard-only traversal of the season tabs and the episode list, visible focus on every control, accessible names on tabs and stills, a screen-reader pass over the metadata chips and section headings, a show with specials, a show with a season whose episodes are partly missing, a show with no TMDB match at all, and the layout at 1440, 768, and 390 pixels.

## Implementation status

This document was approved on 2026-09-12 and is the plan of record for `feature/tv-shows-details`.

### 2026-09-12 — implemented

All four phases landed on `feature/tv-shows-details`. The schema did not change and no index was added; every new query plans as an indexed `SEARCH`, confirmed with `EXPLAIN QUERY PLAN` through `python3`.

Three deviations from the text above, each deliberate:

- **`docs/tv-show-scanner-plan.md` no longer exists.** It was deleted when the scanner merged, so Phase 4's instruction to update its scope section is moot, and the two links to it here plus the one in `README.md` were repointed.
- **`ExtraVideosSection` takes one required `returnTo`**, replacing both `movieId` and the optional `trailerReturnTo` it already had, rather than keeping the override alongside it.
- **Specials are excluded from the hero's season and episode tallies.** TMDB's `tmdb_season_count` and `tmdb_episode_count` cover the numbered run only, so counting season zero locally would render a show with two seasons and a specials season as "3 of 2 seasons". The season list itself still shows specials, sorted last.

Verified: the unfiltered API package run (contract-coverage gate included), `make test-openapi`, `make check`, 18 new web unit tests, and 5 new Playwright specs covering the default season, a season switch through the URL, keyboard operation of the season tabs, and the 390-pixel layout.
