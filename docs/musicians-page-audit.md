# Musicians Page Audit

Audit date: June 10, 2026

## Scope

- App: `http://localhost:3000`
- API: `http://localhost:8080`
- Auth: E2E admin user, `admin@example.com`
- Listing route: `http://localhost:3000/music?tab=musicians`
- Detail route inspected from the listing: `http://localhost:3000/music/musician/278` (`Abel Pintos`)
- Browser inspection: Playwright browser automation with screenshots, DOM layout metrics, accessibility snapshots, keyboard tab traversal, console events, failed requests, and HTTP response status tracking.

Tooling note: Playwright MCP and Chrome DevTools MCP were attempted first, but the MCP browser profile was locked and Chrome DevTools could not start a headful browser in this environment. I used a one-off headless Playwright browser inspection from the repo dependency instead. No Playwright tests or app code were added.

## Viewports Inspected

| Viewport | Listing Result | Detail Result |
| --- | --- | --- |
| Mobile narrow, `360x740` | No horizontal page overflow. Music tabs wrap into two columns and remain readable. Musician cards render as a two-column grid. Pagination remains usable. | No horizontal page overflow. Artist image, `h1`, stats, Play All, Shuffle, discography, track row, and back link remain readable and operable. |
| Mobile common, `390x844` | No horizontal page overflow. Cards, tabs, library stats, and pagination remain readable. | No horizontal page overflow. Primary actions stack cleanly. |
| Tablet, `768x1024` | No horizontal page overflow. The sidebar is visible, content remains readable, and pagination is usable. | The page root does not report horizontal scroll, but the Shuffle button is visually clipped by the right viewport edge. |
| Desktop, `1440x900` | No horizontal page overflow. Sidebar, tabs, cards, and pagination are readable and operable. | No horizontal page overflow. Detail layout has enough space for the hero controls. |

## Console And Network

| Viewport | Listing / Detail Console Result |
| --- | --- |
| `360x740` | No warnings, console errors, page errors, request failures, or HTTP `>=400` responses during audited page load and traversal. |
| `390x844` | No warnings, console errors, page errors, request failures, or HTTP `>=400` responses during audited page load and traversal. |
| `768x1024` | One console resource error and one HTTP `503`: `GET /api/tmdb/movies/in-theaters`. |
| `1440x900` | One console resource error and one HTTP `503`: `GET /api/tmdb/movies/in-theaters`. |

## Accessibility And Keyboard Findings

- The listing exposes a `main` landmark, search region, navigation controls, `h1` (`Music Library`), a named library statistics region, a `tablist`, selected `Musicians` tab, named musician card links, live/status text (`Showing 24 musicians, page 1 of 14`), and named pagination controls.
- Musician card links include useful names such as `Abel Pintos, 1 albums, 1 tracks`.
- Pagination controls expose names including `Go to previous page`, `Page 1, current page`, `Go to page 2`, `Go to page 14`, and `Go to next page`.
- The detail page exposes the artist article as `Abel Pintos`, an `h1`, musician statistics, named Play All and Shuffle buttons, `Discography`, `All Tracks`, track buttons, and `Back to Musicians library`.
- Keyboard focus reaches the primary detail controls without trapping. On desktop/tablet with the sidebar visible, focus passes through sidebar navigation before the page controls, then reaches search, notifications, cast, artist summary, Play All, Shuffle, discography, track actions, and the back link.

## Issues

| Severity | Issue | Evidence | Recommended Fix |
| --- | --- | --- | --- |
| Medium | Detail page action row clips the Shuffle button at tablet width. | At `768x1024`, the `Shuffle play all 1 tracks by Abel Pintos` button measured `left: 654`, `right: 784`, `width: 131` in a `768` px viewport. The screenshot shows the right side of the button cut off. | Allow the hero action row to wrap or stack at the sidebar/tablet breakpoint, or constrain the hero text/action column to the available content width. |
| Low | Music pages can emit an unrelated movie API console error on wider layouts. | At `768x1024` and `1440x900`, the audit captured `Failed to load resource: the server responded with a status of 503` for `GET /api/tmdb/movies/in-theaters`. | Avoid fetching the in-theaters endpoint from the music route/sidebar unless needed, or handle the unavailable response without emitting user-visible console noise. |
| Low | A non-interactive artist summary is in the keyboard tab order on the detail page. | Detail focus order includes a `span` named `Abel Pintos. 1 album, 1 track. Total duration: 3m 42s. Genres: Pop Latino.` between Cast and Play All. | If the summary is intended as screen-reader context, attach it with `aria-describedby` or an article label instead of making a static `span` tabbable. Keep the tab order limited to interactive controls where possible. |

## Residual Risk

The audit used browser accessibility snapshots and keyboard traversal, not a full manual pass with NVDA, JAWS, VoiceOver, or TalkBack. A manual screen-reader smoke test is still recommended before closing accessibility work on these pages.

