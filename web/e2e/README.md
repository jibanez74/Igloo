# Playwright end-to-end tests

The functional specs (`*.spec.ts`, run by `playwright.config.ts`) and the
screenshot specs (`visual/*.visual.ts`, run by `playwright.visual.config.ts`;
see `docs/design-system.md` §2.5) live here. Everything runs from `web/`.

```bash
bun run test:e2e                                  # dev server + mock API
bun run build && E2E_PRODUCTION=1 bun run test:e2e   # what CI runs
bun run test:e2e -- e2e/movies-index.spec.ts      # one spec
make test-visual-docker                           # screenshots (repo root)
```

Port 3000 or 8080 may already be in use on the machine. `E2E_WEB_PORT` and
`E2E_MOCK_API_PORT` move Vite and the mock API, and both configs and
`vite.config.ts` read them. Give each run that shares the machine its own
`--output` directory.

## Two modes

- **Mocked (default).** Playwright starts `mock-api-server.ts` and Vite, or
  `vite preview` of `dist` under `E2E_PRODUCTION=1`. Specs run one at a time
  (`workers: 1`) because the mock is stateful.
- **A real instance.** Set `E2E_BASE_URL`, plus `E2E_ADMIN_EMAIL` and
  `E2E_ADMIN_PASSWORD`, and Playwright drives that server instead. Never point
  this at a server whose data matters: specs create and delete users and change
  settings, and they restore only what they change.

Each spec says which mode it needs, using the guards in `e2e-env.ts`.
`requireRealInstance(reason)` skips a test that needs real media or would only
exercise the mock's own logic. `requireMockApi()` skips a test that depends on
mock fixtures. A test with neither guard must pass in both modes.

## Two ways to mock

- **The stateful mock** (`mock-api-server.ts`, with its watch rooms in
  `mock-watch-rooms.ts`) serves auth, users, settings, devices,
  notifications, the players' routes and watch rooms. A route it does not
  serve answers 404 `Unhandled mock API route`, which fails the test as an
  `/api/` error. Each test starts from its initial state (see below).
- **Per-test stubs.** `mockApi(page, { user, handle })` (`e2e-mock-api.ts`)
  installs one `**/api/**` catch-all. It answers what the signed-in shell
  always asks for and hands everything else to `handle`. Any request that
  `handle` does not answer is recorded and fails the test. The library,
  details, home, music and search specs use it.

Response data comes from the contract-typed builders in `fixtures/*.ts`
(music's are shared with the unit tests through `src/test/helpers/music.ts`)
and the envelope helpers in `e2e-api.ts`.

## The shared `test`

Import `test` and `expect` from `./e2e-test`, never from `@playwright/test`
(types still come from there). Every test then gets three checks without
asking for them:

1. **A fresh mock.** In mocked mode, `POST /api/e2e/reset` puts the stateful
   mock back to its initial users, sessions, settings, preferences, devices
   and rooms before each test. A failed test cannot dirty the next one, and a
   mock-only test does not restore what it changed. A test that also runs on a
   real instance still restores what it changes (`putApiData` in `e2e-api.ts`),
   because nothing resets a real server.
2. **A clean browser.** The default `page` is watched for console errors and
   warnings, uncaught errors, failed requests, and `/api/` responses of 400 or
   more. The rules are in `e2e-browser-issues.ts`, which also lists the aborts
   it ignores.
3. **No unexpected requests** reached a `mockApi` catch-all.

All three are asserted when the test ends. When a test has already failed,
the issues are attached to its report (`browser-issues.json`) instead.

To allow what a test drives on purpose, use one of these:

```ts
// a whole spec or describe
test.use({ browserIssueOptions: { ignoreResponse: r => r.status() === 409 } });

// one test, before the actions that cause it
test("…", async ({ page, browserIssues }) => {
  browserIssues.ignore({ ignoreConsole: (_type, text) => text.includes("403") });
});

// a second page, e.g. another user's context
browserIssues.track(otherPage, { /* its own rules */ });
```

Never ignore something just to make a test pass. If it is a real bug, fix it
in the app. If it is expected, write down why. A spec can opt out entirely
with `test.use({ browserIssueOptions: { skip: "reason" } })`, and the reason
shows as an annotation, but no spec does: the real-media specs are tracked
too.

## Running against a real instance

The real-instance tests (`requireRealInstance`) need real media, ffmpeg and a
server whose data does not matter. Run a throwaway server rather than one
people use:

1. **Build and start it** from a scratch directory, not the repo root: the
   server reads `.env` from its working directory. A release binary
   (`make build`) embeds the web bundle, so the watch-room socket stays
   same-origin. Give it its own `PORT`, `DB_PATH`, `STATIC_DIR`, `LOGS_DIR`
   and `TRANSCODE_DIR`, `SESSION_COOKIE_SECURE=false`, and
   `DEFAULT_ADMIN_EMAIL` / `DEFAULT_ADMIN_PASSWORD` equal to the
   `E2E_ADMIN_*` values (the defaults are `admin@example.com` /
   `AdminPassword`). Leave `TMDB_API_KEY` empty, `ENABLE_WATCHER` and
   `DOWNLOAD_IMAGES` off.
2. **Give it a small library.** `MOVIES_DIR` can be a directory of symlinks to
   files (the scanner follows a symlinked file, not a symlinked directory).
   The movie IDs each spec needs are listed at the top of
   `direct-play-media.spec.ts`, `hls-transcode.spec.ts` and
   `watch-room.spec.ts`: an MKV with H.264 and AAC, a 10-bit H.264 MP4, an
   MP4 with two or more audio streams (AAC first), an MP4 whose first
   subtitle is text with real cues (some files carry an empty one), a plain
   H.264 + AAC MP4, a source of 2160p or more, a second movie for another
   profile, and an episode at least 720p tall (the HLS case transcodes it to
   720p). A 10-bit H.264 MP4 is rare in a library; make one
   with Igloo's ffmpeg (`-c:v libx264 -profile:v high10 -pix_fmt
   yuv420p10le -c:a aac`).
3. **Pick the IDs** once the scan is done (`GET /api/settings/scan`) from
   `GET /api/movies/library` and each movie's `technical-details`, and export
   them as `E2E_DIRECT_*`, `E2E_HLS_*`, `E2E_EPISODE_ID` and
   `E2E_WATCH_ROOM_*`. The watch-room direct movie must pass the server's
   direct checks (`watch_room_handler.go`): an MP4 with 8-bit H.264 and AAC
   first.
4. **Mind the transcoder.** The 4K case runs `2160p_16mbps` by default, which
   only a hardware encoder offers. On a CPU-only server, whose largest
   profile is 1080p, set `E2E_HLS_4K_PROFILE=1080p_8mbps` (the source must
   still be 2160p or more).
5. **Run them**, from `web/`:

   ```bash
   REAL='real media|HLS transcoding playback|Watch room realtime|profile PIN'
   REAL+="|paired device's token|login failure without|regular user's direct write"
   E2E_BASE_URL=http://127.0.0.1:18210 bunx playwright test --grep "$REAL" \
     e2e/account-settings.spec.ts e2e/device-lifecycle.spec.ts e2e/login.spec.ts \
     e2e/playback-settings.spec.ts e2e/direct-play-media.spec.ts \
     e2e/hls-transcode.spec.ts e2e/watch-room.spec.ts
   ```

   `--list` should show 16 tests; the titles pick them out of their files.

The server allows 10 sign-ins per 5 minutes from one IP, and one run signs in
about two dozen times. The helpers in `e2e-auth.ts` wait a refused sign-in
out and try again, stretching the test's timeout, so a whole run takes about
ten minutes longer than its tests do; the limit itself is never loosened.
Bundled Chromium decodes H.264 and AAC, so no other browser is needed.

## Writing a spec

- Find elements by role and accessible name (`getByRole`, `getByLabel`).
  Reach for a CSS locator only when the element has no name, and say why.
- Wait for a state, never for time: web-first assertions, `expect.poll`, or a
  route held with `gateRoute`. `waitForTimeout` is not used. When a test needs
  time to pass, install `page.clock` and run it forward. The only waits on
  the wall clock are against a real server's own: its sign-in window
  (`e2e-auth.ts`) and a player's last progress save
  (`closePlayerAndClearProgress`).
- Register `waitForResponse` and request listeners before the action that
  triggers them.
- Use `networkidle` only where "nothing else loads" is what the test checks,
  and say so in a comment.
- Lint checks `e2e/**` with type information, so a promise without `await`
  (an assertion that can never fail) is an error.
- Keep checks that only render something static in vitest (`src/test/**`).
  An e2e test is for behaviour that needs a real browser, a real router or
  real requests.
