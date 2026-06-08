# Music index E2E test plan

Use `web/e2e/music-index.spec.ts` for index-wide Playwright coverage of `/music`. Keep `web/e2e/music-tracks.spec.ts` focused on virtualized track-list behavior and preserve its existing infinite-scroll assertions for offsets `0`, `50`, and `100`.

## Mock setup

Follow the deterministic mocked-API style from `web/e2e/music-tracks.spec.ts`. Tests must not require a seeded backend.

Shared music E2E mocks should cover:

- `GET /api/auth/user`
- `GET /api/music/stats`
- `GET /api/music/albums`
- `GET /api/music/musicians`
- `GET /api/music/tracks`
- `GET /api/music/tracks/liked`
- `GET /api/music/tracks/liked-ids`
- `GET /api/music/playlists`
- `POST /api/music/playlists`

Tests should fail on console errors, page errors, failed app API responses, and unexpected API routes.

## Test checklist

### Music shell and tabs

- Assert `/music` loads with the page title, heading, stats, and four accessible tabs.
- Assert the default visible content is the `albums` tab.
- Click Musicians, Albums, Tracks, and Playlists, then assert visible content and URL search state update.

Reason: catches broken route loaders, auth mocking, tab roles, default tab behavior, URL-backed tab state, and screen-reader-visible tab state.

### Albums tab

- Mock at least two albums with musician names and one missing cover.
- Assert album cards render with accessible link names and fallback cover behavior.
- Assert pagination changes `albumsPage` and requests the next album page.

Reason: covers the default music landing content and paginated album browsing.

### Musicians tab

- Mock at least two musicians with album and track counts.
- Assert musician cards expose accessible link names with count text.
- Assert pagination changes `musiciansPage` and requests the next musician page.

Reason: covers the artist-browsing workflow and count text used by screen readers.

### Tracks tab controls

- Keep the existing infinite-scroll coverage in `web/e2e/music-tracks.spec.ts`.
- Add index-level assertions for `Play all`, `Shuffle all`, liked-state heart labels, the track action menu, `Go to Album`, and `Go to Artist`.

Reason: covers primary track actions without turning this into audio playback or detail-page testing.

### Playlists tab

- Mock owned and non-owned playlists.
- Assert playlist count, playlist cards, owner badge, `Liked tracks`, and `New playlist`.
- Open the create playlist dialog, submit a valid playlist, assert `POST /api/music/playlists`, dialog close, and focus restoration.

Reason: covers playlist discovery and the main create workflow.

### Liked tracks subview

- Enter through the `Liked tracks` button with `playlistsView=liked`.
- Assert the back button, heading, total count, liked track rows, pagination, and URL update.
- Assert returning to playlists removes the liked-view URL state.

Reason: covers the nested playlists-tab workflow most likely to regress because it shares a top-level tab with a subview.

### Mobile layout

- Use a `390x844` viewport.
- Assert the tablist, stats, cards and lists, playlist controls, and track actions have no horizontal overflow.

Reason: music tabs use dense controls and grids that are vulnerable to mobile overflow.

## Acceptance criteria

- Use role-based or accessible-name locators where possible.
- Verify every icon-only or compact control through its accessible name.
- Do not add runtime dependencies.
- Do not require backend changes.
- Keep album, musician, and playlist detail pages out of scope except for asserting correct index-level link destinations.

Verification command:

```sh
cd web && bun run test:e2e -- e2e/music-index.spec.ts e2e/music-tracks.spec.ts
```
