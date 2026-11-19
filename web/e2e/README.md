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
  `mock-watch-rooms.ts`) serves auth, users, settings, devices, the players'
  routes and watch rooms. A route it does not serve answers 404
  `Unhandled mock API route`, which fails the test as an `/api/` error. Each
  test starts from its initial state (see below).
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
with `test.use({ browserIssueOptions: { skip: "reason" } })`. Only the
real-media specs do, and the reason shows as an annotation.

## Writing a spec

- Find elements by role and accessible name (`getByRole`, `getByLabel`).
  Reach for a CSS locator only when the element has no name, and say why.
- Wait for a state, never for time: web-first assertions, `expect.poll`, or a
  route held with `gateRoute`. `waitForTimeout` is not used. When a test needs
  time to pass, install `page.clock` and run it forward.
- Register `waitForResponse` and request listeners before the action that
  triggers them.
- Use `networkidle` only where "nothing else loads" is what the test checks,
  and say so in a comment.
- Lint checks `e2e/**` with type information, so a promise without `await`
  (an assertion that can never fail) is an error.
- Keep checks that only render something static in vitest (`src/test/**`).
  An e2e test is for behaviour that needs a real browser, a real router or
  real requests.
