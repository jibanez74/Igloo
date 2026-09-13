# Igloo

Igloo is a self-hosted media center for personal movie and music libraries, with a Go server and a React web client. It is designed for home servers and small personal infrastructure, usually on a private network or Tailscale tailnet.

Accessibility is a core design goal: keyboard navigation, visible focus, screen reader labels, skip links, and reduced-motion behavior are part of the web client's design rules. See the [design system](docs/design-system.md) for the concrete UI requirements.

Igloo is **pre-production**. APIs, database schema, configuration, and client behavior may change before v1, without backward compatibility or database migrations.

## Contents

- [Overview and current status](#overview-and-current-status)
- [Supported platforms and limitations](#supported-platforms-and-limitations)
- [Getting started](#getting-started)
- [Configuration and playback](#configuration-and-playback)
- [Building and running a binary](#building-and-running-a-binary)
- [Development reference](#development-reference)
  - [Repository structure](#repository-structure)
  - [Commands](#commands)
  - [Database and sqlc](#database-and-sqlc)
  - [API documentation](#api-documentation)
  - [Testing and CI](#testing-and-ci)
  - [Browser tests](#browser-tests)

## Overview and current status

- **Movies:** library scanning, local metadata and optional TMDB enrichment, posters and backdrops, trailers where available, cast and crew, technical stream details, and admin metadata editing. Playback includes direct streaming, HLS remuxing and transcoding, supported text subtitles, audio/subtitle selection, watch progress, likes, and playlists. HLS remains a work in progress.
- **Music:** library scanning, albums, tracks, musicians, cover art, optional Spotify enrichment, multi-artist credits, collaborative playlists, liked tracks, playback, and listening statistics.
- **TV shows:** library scanning with file-owned technical metadata, combined episodes and duplicate copies, optional TMDB show/season/episode enrichment, and safe missing-file cleanup, plus a read-only show details page with seasons, episodes, and credits. Playback is not implemented.
- **Shared playback:** watch rooms synchronize movie playback over WebSockets, with direct-stream and HLS playback paths.
- **Accounts and administration:** session-based sign-in, admin user management, account settings and avatars, profile PINs, Quick Connect device pairing, device listing/renaming/revocation, server and library settings, and per-user playback preferences.

Library search covers movies, albums, musicians, and tracks. The Go server serves the web client in production; Vite serves it during development.

## Supported platforms and limitations

The supported server platforms are **Linux x64 (AMD64)** and **macOS ARM64 (Apple Silicon)**. Windows, Docker deployment, and other Linux architectures are not supported. Production builds are native to the host platform.

Current limitations:

- HLS transcoding and decoding are works in progress. Playback depends on source media, browser capabilities, FFmpeg support, and host hardware; see [media behavior and operational notes](docs/ffmpeg.md).
- TV library scanning and the show details page are implemented; TV playback is not. The TV library index and photos still have UI placeholders.
- Automatic filesystem watching is not implemented, even though an `ENABLE_WATCHER` setting exists. Use startup or manual library scans.
- Jellyfin and Immich fields are stored settings, not working integrations.
- TMDB and Spotify enrichment are optional. Without them, scanning uses local file metadata and movie filename defaults.
- Native TV clients, including the planned Android TV / Google TV app, are outside this repository.

## Getting started

Start from source; there are currently no published GitHub releases to download.

### Prerequisites

Install these tools on a supported platform and make them available on your `PATH`:

- Git and Make.
- Go `1.26.2`, matching [server/go.mod](server/go.mod).
- A C compiler for CGO and SQLite. The Make targets enable CGO and apply the `sqlite_fts5` build tag required by search and backend tests.
- `sqlc` for generating database access code.
- Bun for web dependencies and scripts.
- `ffmpeg` and `ffprobe` for development and backend tests. Embedded media payloads and `zstd` are only needed for [production binary builds](#building-and-running-a-binary).

### Set up the checkout

```bash
git clone https://github.com/jibanez74/Igloo.git
cd Igloo
cp .env.example .env
cd web
bun install --frozen-lockfile
cd ..
```

Edit the root `.env` before starting the backend:

- Choose `DEFAULT_ADMIN_NAME`, `DEFAULT_ADMIN_EMAIL`, and a nonempty `DEFAULT_ADMIN_PASSWORD`. These create the initial administrator; use that email and password to sign in.
- Keep `PORT=8080` for the development workflow below and `SESSION_COOKIE_SECURE=false` for local plain HTTP. Use `true` when serving over HTTPS.
- Optionally set `MOVIES_DIR` and `MUSIC_DIR` to existing directories on the server. Leave unused library paths blank. You can configure paths from Settings after signing in.

The [configuration reference](#configuration-and-playback) explains optional metadata credentials, storage paths, and which environment values apply only on first launch.

### Start and open Igloo

In one terminal, from the repository root:

```bash
cd web
bun run dev
```

In a second terminal, from the repository root:

```bash
make dev
```

Vite runs at `http://localhost:3000` and proxies `/api` to the backend on port `8080`. `make dev` generates sqlc code, prepares a placeholder web asset directory, builds the development API binary, and runs it from the repository root with `VITE_DEV_SERVER=http://localhost:3000`.

Open **http://localhost:3000** and sign in with the administrator credentials from `.env`. In **Settings → Libraries**, enter the movie and music paths, save them, then use **Scan movies library** or **Scan music library**. Paths refer to the server's filesystem and must exist and be readable by the server process. Configured libraries are also scanned at backend startup. Wait for scanning to finish before browsing or playing imported media; run another scan after adding or changing files.

## Configuration and playback

Igloo loads one optional `.env` from its **current working directory**. Process environment variables take precedence. The file is runtime configuration and is never embedded into a build. Make targets do not create, copy, rewrite, or delete `.env`; both `make dev` and `make start` use the repository root as the runtime working directory.

[.env.example](.env.example) is a starting template, not an exhaustive list. Configuration has three lifetimes:

- **Startup:** read when the process starts.
- **First-run Settings seed:** saved only when the database has no Settings row. On later starts the database wins; edit these values in Settings.
- **Admin bootstrap:** used when the database has no administrator. Changing these values does not reset an existing account.

| Variable | Lifetime | Purpose and default |
| --- | --- | --- |
| `PORT` | Startup | HTTP listener port; `8080` |
| `DB_PATH` | Startup | SQLite file; `db/igloo.db` |
| `SESSION_COOKIE_SECURE` | Startup | `false` for plain HTTP; set `true` behind HTTPS, including Tailscale Serve or a reverse proxy |
| `LOG_TO_STDOUT` | Startup | Send logs to stdout instead of file logs; defaults to `DEBUG` when unset (`false` normally). The example explicitly sets `false` |
| `DEBUG` | Startup | Debug logging; `false` |
| `HLS_MAX_CPU_TRANSCODES` | Startup | Positive limit on concurrent HLS sessions encoding video or audio, including hardware transcodes; defaults to `max(1, NumCPU/4)` |
| `HLS_MAX_SESSIONS_PER_USER` | Startup | Positive personal HLS session limit per user, including pending creations; default `3`. Covers remux and transcode sessions; watch rooms are separate |
| `IGLOO_FFMPEG_PATH`, `IGLOO_FFPROBE_PATH` | Startup, `externalbin` builds only | Override media executable resolution; otherwise use `ffmpeg` and `ffprobe` on `PATH`. Make's development/test prerequisites still check `PATH` |
| `VITE_DEV_SERVER` | Startup | Forward non-API browser requests to Vite; set by `make dev`, cleared by `make start` |
| `DEFAULT_ADMIN_NAME`, `DEFAULT_ADMIN_EMAIL`, `DEFAULT_ADMIN_PASSWORD` | Admin bootstrap | Initial administrator; password required. Set all three explicitly in `.env` |
| `STATIC_DIR` | Settings seed | Downloaded artwork and uploaded static files; `static` |
| `LOGS_DIR` | Startup | File logs; `logs` |
| `TRANSCODE_DIR` | Settings seed | Temporary HLS workspace; `transcode` |
| `MOVIES_DIR`, `SHOWS_DIR`, `MUSIC_DIR` | Settings seed | Existing library directories; empty by default |
| `TMDB_API_KEY` | Settings seed | Optional movie and TV metadata enrichment; empty by default |
| `SPOTIFY_CLIENT_ID`, `SPOTIFY_CLIENT_SECRET` | Settings seed | Optional music metadata enrichment; empty by default |
| `JELLYFIN_API_KEY` | Settings seed | Stored Jellyfin setting; no working integration |
| `DOWNLOAD_IMAGES` | Settings seed | Image downloading setting; `false` |
| `ENABLE_WATCHER` | Settings seed | Stored watcher setting; `false`. Automatic watching is unimplemented |
| `HARDWARE_ACCELERATION_DEVICE` | Settings seed | `cpu` (default), `apple`, `nvidia`, or `intel` |

Relative runtime paths resolve from the working directory. Igloo creates its application storage directories as needed, but does not create media library directories. The process needs permission to write its database, static files, logs, and transcode workspace. Allow space for temporary HLS output under the configured transcode directory.

### TV library scanning

Set `SHOWS_DIR` on first launch or save the TV path in library Settings. Startup scans TV alongside movies and music. Admin clients can start another scan with `POST /api/settings/scan/shows` and read progress from `GET /api/settings/scan/shows`; saving Settings alone does not launch one. Trigger responses are `200` started, `409` already running, or `500` unconfigured, with normal authentication and admin authorization.

Use `Show Name (optional year)/Season N/filename.mkv`, or `Show Name/Specials/filename.mkv` for season zero. Filename numbering accepts `S01E02`, `S01E02-E04`, `S01E02E03`, and `1x02`, case-insensitively. The filename season must match its directory. Hidden backups, hidden files, nested extras, NFO files, and subtitle sidecars are excluded. Show folders remain separate identities even if TMDB matches the same show.

Files must be quiet for 60 seconds. TMDB is optional: local imports remain in the catalog, and later scans retry pending enrichment without probing unchanged files. Combined files link separate episodes to one physical file and retain its complete duration without guessed episode boundaries. See [media scanning](docs/ffmpeg.md#tv-show-scanning).

### Playback and hardware acceleration

Direct playback serves the original file without FFmpeg when eligible. Other playback paths use FFmpeg to remux, convert audio, or transcode video into HLS. Supported text subtitles can be converted to WebVTT; bitmap subtitles such as PGS and DVD subtitles cannot be converted this way. Alternate audio tracks can require HLS even when the video itself is directly playable.

CPU transcoding is the default. `apple` uses VideoToolbox on macOS; `intel` and `nvidia` target Linux hosts with the corresponding drivers and device access. Igloo probes FFmpeg capabilities at startup and falls back to CPU when the selected hardware path is unavailable. Hardware support still depends on the host and FFmpeg build.

The authoritative [FFmpeg documentation](docs/ffmpeg.md) covers scanning, direct-play eligibility and fallback, HLS sessions and limits, hardware decoding/encoding, HDR tone mapping, audio selection, subtitles, and troubleshooting.

## Building and running a binary

Production builds embed the web client and platform-specific FFmpeg/ffprobe payloads. In addition to the source setup prerequisites, install `zstd` and supply **Jellyfin FFmpeg** payloads for the current supported platform. Use FFmpeg and ffprobe from the same stable Jellyfin release, following the [binary strategy](docs/ffmpeg.md#binary-strategy); do not substitute generic upstream release payloads. These large files are intentionally ignored by Git.

| Platform | FFmpeg payload | ffprobe payload |
| --- | --- | --- |
| Linux x64 | `server/cmd/internal/ffmpeg/ffmpeg_linux_amd64` | `server/cmd/internal/ffprobe/ffprobe_linux_amd64` |
| macOS ARM64 | `server/cmd/internal/ffmpeg/ffmpeg_darwin_arm64` | `server/cmd/internal/ffprobe/ffprobe_darwin_arm64` |

From the repository root:

```bash
make build
```

This compresses the media payloads into `.zst` files, generates sqlc code, builds the web client, copies `web/dist` into `server/cmd/api/webdist`, and writes `server/dist/igloo-server`. Build Linux x64 on Linux x64 and macOS ARM64 on macOS ARM64. The binary embeds web assets and compressed media tools; it extracts or reuses cached media tools and validates them at startup, with temporary-directory extraction as a fallback (see the [binary strategy](docs/ffmpeg.md#binary-strategy)). Host hardware drivers are still required for acceleration.

Run the binary in the foreground from the repository root to load the root `.env`:

```bash
./server/dist/igloo-server
```

Open `http://localhost:8080` with the default port. If you move the executable, start it from the directory containing your runtime `.env`, or provide configuration through the shell or a service manager. Igloo does not automatically look for `.env` next to the executable.

Alternatively, build and run in the background from the repository root:

```bash
make start
```

`make start` rebuilds the application, clears `VITE_DEV_SERVER` to serve embedded assets, and writes `server/dist/igloo-server.pid` and `server/dist/igloo-server.log`. Stop that process with:

```bash
make stop
```

## Development reference

### Repository structure

| Path | Contents |
| --- | --- |
| [server/](server/) | Go backend, chi router, SQLite, scanning, playback, and media tools |
| [web/](web/) | React/Vite browser client and frontend tests |
| [docs/](docs/) | Design system, media documentation, and API contract/maintenance notes |

Follow the existing [repository instructions](AGENTS.md), [backend instructions](server/AGENTS.md), and [web instructions](web/AGENTS.md). The [design system](docs/design-system.md), [media documentation](docs/ffmpeg.md), and [OpenAPI contract](docs/openapi.json) are authoritative.

### Commands

Run these from the **repository root**:

| Command | Description |
| --- | --- |
| `make dev` | Generate sqlc code and run the development API using host media tools |
| `make dev-profile` | Development API with admin-only pprof endpoints at `/api/debug/pprof` |
| `make build` | Build a native binary with embedded web assets and media tools |
| `make start` / `make stop` | Build/start or stop the background application |
| `make clean` | Stop the background app and remove build artifacts; preserve `.env`, database, media, and runtime data |
| `make generate` | Regenerate database access code with sqlc |
| `make check` | OpenAPI lint/coverage and generated-type checks, backend lint/tests, web lint/tests, and web build/type-check |
| `make test` | Backend and web unit tests |
| `make lint-server` | Go vet and whole-program dead-code reachability check |
| `make test-server` | Backend tests with CGO, `externalbin sqlite_fts5`, and placeholder web assets |
| `make test-tmdb-integration` | Live TMDB tests; requires `TMDB_API_KEY` in the process environment |
| `make test-web` / `make lint-web` / `make build-web` | Vitest, ESLint, or web build/type-check |
| `make test-openapi` | OpenAPI lint and registered-route coverage test |
| `make lint-openapi` | Redocly lint for `docs/openapi.json` |
| `make generate-openapi` / `make check-openapi` | Generate frontend schemas or check committed output for drift |
| `make preview-openapi` | Build and serve API reference HTML at `127.0.0.1:8081` |

Run these from **web/**:

| Command | Description |
| --- | --- |
| `bun run dev` | Vite development server on port `3000` |
| `bun run typecheck` / `bun run build` | TypeScript checking, or checking plus production build |
| `bun run build:analyze` | Build with bundle visualization in `web/dist/bundle-analysis.html` |
| `bun run lint` / `bun run test` / `bun run test:coverage` | ESLint, Vitest, or Vitest with coverage |
| `bun run generate:openapi` / `bun run check:openapi` | Generate/check `src/types/openapi.gen.ts` |
| `bun run lint:openapi` / `bun run preview:openapi` | Lint the API contract or serve a temporary API reference |
| `bun run test:e2e` | Playwright specs; default configuration starts the frontend and mock API |
| `bun run test:e2e:boot` | Build production assets and test startup/reloads through Vite preview and the mock API |
| `bun run preview` | Preview an existing production build |
| `bun run doctor` | React Doctor analysis |

### Database and sqlc

SQLite runs in WAL mode. [server/sqlc/schema.sql](server/sqlc/schema.sql) is the schema source of truth and embedded startup schema; queries live in [server/sqlc/queries/](server/sqlc/queries/). Generated access code lives in `server/cmd/internal/database/`.

Modify the current schema directly during pre-production; do not add migrations or manually edit generated code. After changing schema or queries, run `make generate` from the repository root. It runs `sqlc generate` in `server/sqlc`.

### API documentation

The manually maintained [OpenAPI contract](docs/openapi.json) covers registered `/api` routes, including JSON endpoints, static files, media streams, HLS playlists and segments, subtitles, and the watch-room WebSocket. Read it before changing API behavior or frontend API calls/types, and update it whenever the contract changes.

After API changes, run `make generate-openapi` when schemas change and `make test-openapi check-openapi` to check the contract, route coverage, and generated types. See [OpenAPI maintenance](docs/openapi-maintenance.md) for the workflow.

### Testing and CI

Run `make check` after every backend change, as required by [backend instructions](server/AGENTS.md), and `make lint-web test-web build-web` for frontend changes. `make check` runs the combined contract, generated-type, backend lint/test, and frontend lint/test/build checks. Backend Make targets supply the required FTS5 and external-media build tags.

[GitHub Actions](.github/workflows/ci.yml) runs backend vet/dead-code checks and tests, OpenAPI lint/route coverage/generated-type checks, and frontend lint, generated-type checks, unit tests, and type-check/build. It also installs Chromium and runs a **separate production-startup browser test**, which is not included in `make check`. No CI job publishes binaries.

Live TMDB integration tests are outside the default suite. From the repository root:

```bash
TMDB_API_KEY=your_tmdb_v3_key make test-tmdb-integration
```

### Browser tests

Install Playwright Chromium once from `web/`:

```bash
bun x playwright install --with-deps chromium
```

With `E2E_BASE_URL` **unset**, Playwright starts a mock API on `127.0.0.1:8080` and Vite on `127.0.0.1:3000`; it does not start the Go backend. These ports must be free, or set `E2E_WEB_PORT` and `E2E_MOCK_API_PORT`. Run all specs with `bun run test:e2e`, or one spec with `bun run test:e2e -- e2e/<name>.spec.ts`. Fixture-dependent live suites need the configuration below; a default run does not exercise all real-server behavior.

For the production-startup check, run `bun run test:e2e:boot`. To test an already-built web bundle as CI does, use `E2E_PRODUCTION=1 bun run test:e2e e2e/boot.spec.ts`. Leave `E2E_BASE_URL` unset for these checks so Playwright starts Vite preview and the mock API.

For live suites, start Igloo separately and configure the test shell once. From `web/`, replacing the credentials with your existing administrator's:

```bash
export E2E_BASE_URL=http://localhost:8080
export E2E_ADMIN_EMAIL=admin@example.com
export E2E_ADMIN_PASSWORD='your-admin-password'
bun run test:e2e:login
```

An explicit `E2E_BASE_URL` disables both managed servers; the URL must serve the frontend and its `/api` requests. A development frontend at `http://localhost:3000` with the Go backend running also works. E2E credentials default to `admin@example.com` / `AdminPassword`; these test defaults do not configure a real Igloo account.

Suite commands below run from `web/` and share that setup where a live server is needed:

| Command | Scope and requirements |
| --- | --- |
| `bun run test:e2e:login` | Login, invalid credentials, redirects, responsive layout, accessibility, and browser errors |
| `bun run test:e2e:account-settings` | Account/profile, password, avatar, and PIN controls; creates temporary test users |
| `bun run test:e2e:general-settings` | Updates stored Jellyfin/Immich settings, checks persistence, and restores originals |
| `bun run test:e2e:playback-settings` | Playback preference controls and persistence |
| `bun run test:e2e:libraries-settings` | Library path validation, saving, and scan controls; live server must see the temporary directories created by the test runner |
| `bun run test:e2e:user-settings` | Admin user management |
| `bun run test:e2e:movies` | Movie navigation, liked movies, and tab transitions |
| `bun run test:e2e:quick-connect` | Live pairing, device tokens, rename/revoke, and lifecycle checks; requires explicit `E2E_BASE_URL` |
| `bun run test:e2e:device-lifecycle` | Mock device lifecycle UI checks; skipped when `E2E_BASE_URL` is set |
| `bun run test:e2e:movies:index` / `bun run test:e2e:movie-extra-videos` | Mocked movie index and extra-video/YouTube player checks |
| `bun run test:e2e:movie-player` / `bun run test:e2e:direct-fallback` | Mocked player and direct-play fallback checks |
| `bun run test:e2e:album-details` / `bun run test:e2e:musician-details` | Mocked music detail pages |
| `bun run test:e2e:hls` | Live HLS transcodes; two scanned movie IDs as described below |
| `bun run test:e2e:watch-room` | Live HTTP/WebSocket synchronization with a temporary guest and room; browser media playback is stubbed |
| `bun run test:e2e:direct-media` | Live direct-play media fixtures; see below |

Mocked suites use the mock API and/or stub requests in Playwright, so they need no Go backend. Use the default managed setup. Fully intercepted page suites, such as movie index and album/musician details, can also target an existing frontend; the movie-player and direct-fallback suites depend on mock API fixtures. Additional specs cover home, movie details, music index/tracks, search, trailers, head metadata, motion, and browser issues; see [web/e2e/](web/e2e/).

For HLS, supply a scanned 4K movie and a second scanned movie using a different transcode profile. With the shared live configuration still set:

```bash
E2E_HLS_4K_MOVIE_ID=1 E2E_HLS_SECOND_MOVIE_ID=2 bun run test:e2e:hls
E2E_WATCH_ROOM_MOVIE_ID=1 bun run test:e2e:watch-room
```

Replace example IDs with your library's IDs. HLS profile defaults are `2160p_16mbps` and `720p_3mbps`; optional overrides are `E2E_HLS_4K_PROFILE`, `E2E_HLS_SECOND_PROFILE`, `E2E_HLS_AUDIO_TRACK`, `E2E_HLS_TEST_TIMEOUT_MS`, and `E2E_HLS_RESPONSE_TIMEOUT_MS`. Set `E2E_EPISODE_ID` to a scanned TV episode to add the episode case to the HLS suite (its profile defaults to `720p_3mbps`; override with `E2E_HLS_EPISODE_PROFILE`). The watch-room suite needs a movie suitable for direct playback and accepts `E2E_WATCH_ROOM_RESPONSE_TIMEOUT_MS`.

The [direct-media suite](web/e2e/direct-play-media.spec.ts) runs its movie cases when `E2E_DIRECT_MKV_MOVIE_ID` (H.264/AAC MKV), `E2E_DIRECT_10BIT_MOVIE_ID` (10-bit H.264 MP4), and `E2E_DIRECT_MULTIAUDIO_MOVIE_ID` (MP4 with multiple audio streams) are all set, and its episode case when `E2E_EPISODE_ID` is set; either group runs without the other. Optional controls use `E2E_DIRECT_MP4_MOVIE_ID` (ordinary H.264/AAC MP4) and `E2E_DIRECT_SUBTITLE_MOVIE_ID` (direct-eligible MP4 with an embedded text subtitle and at least 90 seconds duration).

Unset `E2E_BASE_URL` before returning to the managed mock or production-startup setup.
