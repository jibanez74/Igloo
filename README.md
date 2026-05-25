# Igloo

Igloo is a self-hosted media center for personal movie and music libraries. It is built for people who want to own their media, run their own server, and enjoy a polished playback experience without depending on a managed cloud platform.

Accessibility is one of Igloo's core design values, especially strong screen reader support, but Igloo is not meant to be a media platform only for blind users. The goal is to build a media center that feels excellent for everyone: fast, attractive, reliable, comfortable to navigate, and usable whether someone is browsing visually, using a keyboard, navigating with a remote, or relying on assistive technology.

Igloo is intended to run on user-managed hardware, usually inside a private network or Tailscale tailnet. It is pre-v1 software, so API, database, configuration, and client behavior may change before a stable v1 release.

## What This Repository Contains

- `server/`: Go backend, chi API, SQLite startup schema, media scanning, playback endpoints, HLS support, database access, and FFmpeg/ffprobe integration.
- `web/`: React web client for browser-based library management, administration, and playback.
- `docs/`: OpenAPI documentation and project notes.

Native TV clients, including the planned Android TV / Google TV app, are not part of this repository.

## Current Status

What works today:

- Movie library scanning with local metadata, optional TMDB enrichment, posters/backdrops, trailers where available, and technical details.
- Movie features including watch progress, likes, playlists, direct streaming, HLS playback, WebVTT subtitle extraction, and admin metadata editing.
- Music library scanning with albums, tracks, musicians, cover art, optional Spotify enrichment, playlists with collaborators, liked tracks, playback, and listening statistics.
- Watch rooms for shared movie playback, including direct stream and HLS room playback.
- Session-based accounts, admin user management, general settings, and playback preferences.
- A React web client served by the Go server in production and by Vite during development.
- OpenAPI documentation in `docs/openapi.json`, with a route coverage test to keep the spec aligned with the Go router.

Current limitations:

- TV shows and photos have UI placeholders, but they are not complete library features yet.
- APIs may still change before v1.
- Metadata providers are optional; without TMDB or Spotify, Igloo relies on local file metadata.
- Full backend tests require a SQLite build with FTS5 enabled.

See [docs/roadmap.md](docs/roadmap.md) for planned work and known follow-up items.

## Quick Start

Use a packaged binary for your platform:

```bash
tar -xzf igloo-server-linux-amd64.tar.gz
cd igloo-server-linux-amd64
cp .env.example .env
./igloo-server
```

On Apple Silicon, use the `igloo-server-darwin-arm64.tar.gz` package instead.

Before first start, edit `.env`:

- Set `DEFAULT_ADMIN_EMAIL` and `DEFAULT_ADMIN_PASSWORD`.
- Set `SESSION_COOKIE_SECURE=false` when testing over plain HTTP, such as `http://localhost:8080`.
- Keep `SESSION_COOKIE_SECURE=true` when running behind HTTPS, including Tailscale Serve or a reverse proxy.
- Set only the media variables you use, such as `MOVIES_DIR` or `MUSIC_DIR`.
- Each configured media directory must already exist. Igloo will not create empty media library directories.

Igloo listens on `PORT`, defaulting to `8080`.

## Configuration

Igloo reads environment variables from the process environment and from `.env` files. Existing process environment variables win over values loaded from files.

Startup loads env files in this order:

1. `IGLOO_ENV_FILE`, if that process variable is set.
2. `.env` in the current working directory.
3. `../.env`, useful when running from `server/` during development.
4. `.env` next to the running binary.

The most important variables are:

| Variable | Purpose |
| --- | --- |
| `PORT` | HTTP listener port, default `8080` |
| `IGLOO_DATA_DIR` | Base directory for runtime files, default `./data` |
| `DB_PATH` | SQLite database file; overrides `$IGLOO_DATA_DIR/igloo.db` and is always read at startup |
| `STATIC_DIR` | First-run default for downloaded artwork/static files |
| `LOGS_DIR` | First-run default for file logs |
| `TRANSCODE_DIR` | First-run default for the temporary HLS workspace |
| `SESSION_COOKIE_SECURE` | `true` behind HTTPS; `false` for plain HTTP development |
| `DEFAULT_ADMIN_NAME`, `DEFAULT_ADMIN_EMAIL`, `DEFAULT_ADMIN_PASSWORD` | Bootstrap admin account, used only when the database has no admin user |
| `MOVIES_DIR`, `SHOWS_DIR`, `MUSIC_DIR` | First-run media library defaults; configured paths must already exist |
| `TMDB_API_KEY` | Optional TMDB API key for movie metadata and in-theaters data |
| `SPOTIFY_CLIENT_ID`, `SPOTIFY_CLIENT_SECRET` | Optional Spotify credentials for music metadata enrichment |
| `ENABLE_LOGGER`, `ENABLE_WATCHER`, `DOWNLOAD_IMAGES` | Runtime feature flags |
| `LOG_TO_STDOUT` | Send logs to stdout instead of `LOGS_DIR` |
| `HARDWARE_ACCELERATION_DEVICE` | Transcode target: `cpu`, `apple`, `nvidia`, or `intel` |
| `IGLOO_ENV_FILE` | Explicit env file path to load before startup |

See [.env.example](.env.example) for the full reference.

Environment values that are stored in Settings are seed values only. Igloo reads them when the database has no settings row, saves that row, and then uses the database on later starts. Edit static, log, transcode, and media library directories from Settings after first launch. `IGLOO_DATA_DIR` and `DB_PATH` stay environment-driven because the database location must be known before Settings can be loaded.

## Hardware Acceleration

CPU transcoding is the portable default. `HARDWARE_ACCELERATION_DEVICE=apple` enables Apple VideoToolbox on macOS builds. `intel` and `nvidia` are Linux hardware targets and require the corresponding host drivers and device access.

For implementation details, hardware acceleration behavior, and operational notes, see [docs/ffmpeg.md](docs/ffmpeg.md).

## Development Setup

Prerequisites:

- Go `1.26.2`, matching `server/go.mod`
- CGO enabled, with a working C compiler
- SQLite support with FTS5 enabled; use the documented Make and Go test commands so the `sqlite_fts5` build tag is applied
- `sqlc` on your `PATH`
- Bun for the web client
- `ffmpeg` and `ffprobe` on your `PATH` for `make dev` and backend tests
- Local embedded ffmpeg/ffprobe payload files only when building release binaries

Create your environment file:

```bash
cp .env.example .env
```

For local development, use:

```env
DEBUG=true
PORT=8080
SESSION_COOKIE_SECURE=false
IGLOO_DATA_DIR=./data
```

Start the web client:

```bash
cd web
bun install
bun run dev
```

Vite runs on `http://localhost:3000` and proxies `/api` requests to the Go server.

Start the backend in another terminal:

```bash
make dev
```

`make dev` runs sqlc generation, syncs the embedded schema copy, builds a development API binary, and starts it with `VITE_DEV_SERVER=http://localhost:3000` so non-API browser requests are handed to Vite.

Make targets do not create, copy, rewrite, or delete `.env` files. `make dev` runs the API from the repository root, so a root `.env` and default `./data` directory resolve consistently with production `make start`.

## Building Binaries

Release builds embed the web client plus platform-specific ffmpeg and ffprobe payloads. The payload files are intentionally ignored by git because they are large.

Required payload paths:

| Platform | ffmpeg | ffprobe |
| --- | --- | --- |
| Linux AMD64 | `server/cmd/internal/ffmpeg/ffmpeg_linux_amd64` | `server/cmd/internal/ffprobe/ffprobe_linux_amd64` |
| macOS ARM64 | `server/cmd/internal/ffmpeg/ffmpeg_darwin_arm64` | `server/cmd/internal/ffprobe/ffprobe_darwin_arm64` |

Build the complete binary for the current supported platform:

```bash
make build
```

`make build` writes the production binary to `server/dist/igloo-server`. Builds are native-only: Linux AMD64 builds must run on Linux AMD64, and macOS ARM64 builds must run on macOS ARM64.

Run the built application in the background:

```bash
make start
```

`make start` rebuilds the app, runs `server/dist/igloo-server` from the repo root, and writes its PID and log file under `server/dist/`. It sets only `VITE_DEV_SERVER=` so the embedded web app is served. Startup-only configuration comes from the shell environment and `.env` files; Settings-owned values come from the database after first launch. Stop that process with:

```bash
make stop
```

## Useful Commands

From the repository root:

| Command | Description |
| --- | --- |
| `make dev` | Generate sqlc code and run the API for local development using host ffmpeg/ffprobe |
| `make build` | Build the full native binary with embedded web assets and media tools |
| `make start` | Build and run the full application in the background |
| `make stop` | Stop the application started by `make start` |
| `make clean` | Remove build artifacts while preserving `.env`, database, media, and runtime data |

From `web/`:

| Command | Description |
| --- | --- |
| `bun run dev` | Start Vite on port `3000` |
| `bun run build` | Build the production bundle and run TypeScript checking |
| `bun run lint` | Run ESLint |
| `bun run test` | Run Vitest |
| `bun run test:e2e:hls` | Run opt-in Playwright HLS transcoding checks against an existing server |
| `bun run test:e2e:watch-room` | Run opt-in Playwright watch-room sync checks against an existing server |
| `bun run preview` | Preview the production build |

### HLS Transcoding E2E Checks

The Playwright HLS suite targets an already-running Igloo instance. On the first run, install Chromium with `bun x playwright install chromium`. Set the base URL, admin credentials, and two scanned movie IDs before running it. One movie must be a 4K source; the second must use a different transcode profile.

```bash
cd web
E2E_BASE_URL=http://localhost:8080 \
E2E_ADMIN_EMAIL=admin@sample.com \
E2E_ADMIN_PASSWORD=AdminPassword \
E2E_HLS_4K_MOVIE_ID=1 \
E2E_HLS_SECOND_MOVIE_ID=2 \
bun run test:e2e:hls
```

Optional overrides are `E2E_HLS_4K_PROFILE`, `E2E_HLS_SECOND_PROFILE`, `E2E_HLS_AUDIO_TRACK`, `E2E_HLS_TEST_TIMEOUT_MS`, and `E2E_HLS_RESPONSE_TIMEOUT_MS`.

### Watch Room E2E Checks

The Playwright watch-room suite also targets an already-running Igloo instance. It logs in as an admin, creates a temporary guest user and direct-play watch room, drives two browser contexts through the real HTTP and WebSocket flow, and stubs only browser media playback.

```bash
cd web
E2E_BASE_URL=http://localhost:8080 \
E2E_ADMIN_EMAIL=admin@sample.com \
E2E_ADMIN_PASSWORD=AdminPassword \
E2E_WATCH_ROOM_MOVIE_ID=1 \
bun run test:e2e:watch-room
```

Optional override: `E2E_WATCH_ROOM_RESPONSE_TIMEOUT_MS`.

## API Documentation

The OpenAPI document lives at [docs/openapi.json](docs/openapi.json). It covers the registered `/api` routes, including JSON endpoints, static files, media streams, HLS playlists and segments, subtitles, and the watch-room WebSocket.

When adding or changing an API route, update the OpenAPI file and run the route coverage test:

```bash
cd server
go test -tags "externalbin sqlite_fts5" ./cmd/api -run TestOpenAPIDocumentsRegisteredAPIRoutes -count=1
```

See [docs/openapi-maintenance.md](docs/openapi-maintenance.md) for the maintenance workflow.

## Database and SQL

- SQLite is the database engine.
- WAL mode is enabled at startup.
- `DB_PATH` controls the database file path; the binary default is `./data/igloo.db`.
- `server/sqlc/schema.sql` is the schema source of truth.
- `server/cmd/api/schema.sql` is the embedded startup schema copy.
- Query files live under `server/sqlc/queries/`.

After changing schema or query files:

```bash
cp server/sqlc/schema.sql server/cmd/api/schema.sql
cd server/sqlc
sqlc generate
```

## Testing

Backend:

```bash
cd server
mkdir -p cmd/api/webdist
touch cmd/api/webdist/.keep
go test -tags "externalbin sqlite_fts5" -v ./...
```

CI-equivalent backend suite:

```bash
cd server
mkdir -p cmd/api/webdist
touch cmd/api/webdist/.keep
go test -count=1 -v -tags "externalbin sqlite_fts5" ./...
```

Frontend:

```bash
cd web
bun run lint
bun run build
bun run test
```

Live TMDB integration tests are intentionally outside the default suite:

```bash
cd server
TMDB_API_KEY=your_tmdb_v3_key go test -v -tags integration ./cmd/internal/tmdb
```

## CI and Releases

GitHub Actions runs backend tests plus frontend linting and build checks. Production binaries are built with `make build` from the repository root.

## AI Coding Agent Notes

This repository may be used with AI coding agents such as Codex. Project-specific instructions should live in a root-level `AGENTS.md` file.

Recommended guidance for `AGENTS.md`:

```md
# Agent Instructions

This repository contains the Igloo Go server and React web client.

Do not implement native TV client code in this repository unless explicitly requested.

Server:
- Use Go.
- Use chi for routing.
- Use sqlc for database access.
- Keep OpenAPI documentation aligned with registered routes.

Web:
- Use React, TypeScript, Vite, TanStack Router, TanStack Query, and Bun.
- Do not use npm, yarn, or pnpm.

General:
- Prefer explicit, readable code.
- Treat accessibility as part of the feature, not a separate cleanup step.
- Avoid unnecessary abstraction.
```
