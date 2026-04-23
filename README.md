# Igloo

The goal of this project is to build a modern, inclusive media system that prioritizes accessible interfaces and dependable local media playback. It is designed to deliver a high-quality experience for users who value reliability, flexibility, and full control over their media.
At its core, this system is hyper-focused on media playback, supporting a wide range of codecs while maintaining a strong commitment to accessibility—especially for blind users. As a blind developer, I created this system to ensure that I can fully manage and enjoy my own media environment without missing out on any features or relying on others. Accessibility is not treated as an afterthought, but as a fundamental requirement.
This project is also deeply personal. In my family, we value the experience of watching movies together, enjoying music, and revisiting photos and videos that hold meaningful memories. As the person responsible for managing our technology, I need tools that are both powerful and accessible—tools that won't fail because of inaccessible interfaces or overlooked details. This system is built to meet that need, empowering not only me but other blind users to independently manage and enjoy their own media ecosystems.

Igloo is currently in active development and has not yet reached its first stable release. It is being built as a focused media server platform for movies, TV shows, personal videos, and music, with multiple clients planned over time.

This repository contains the Igloo server, including the Go backend, APIs, media indexing and management logic, playback and transcoding workflows, and the React-based web client. The web client is part of this repository, but it is only one client of the platform. Dedicated TV and mobile clients are planned as separate applications that will connect to the same server.

Igloo exists in part because current media servers, while powerful, still leave important accessibility gaps.

Igloo is intentionally focused on personal media libraries and local playback. Rather than trying to reproduce every feature found in larger media platforms, it focuses on a smaller set of core capabilities done well. Features such as live TV, torrent integrations, and large plugin ecosystems are outside the current scope.

For video playback, Igloo supports direct streaming when a file can be played in its original format, preserving full quality. When direct playback is not possible or not ideal for the device or network, Igloo can transcode to HLS and offer multiple bitrate options to adapt to different connection speeds.

For photos, Igloo is intended to integrate with Immich instead of duplicating functionality that Immich already handles well. Rather than building a separate photo platform from scratch, the long-term goal is to connect Igloo with Immich as part of a broader self-hosted media ecosystem.

## Project Status

APIs, features, playback workflows, and client applications are still evolving.

The current focus is the server platform and the React web client contained in this repository. Dedicated TV and mobile clients are planned as separate projects.

The goal is for the project to release a beta version by the end of May 2026, and a v1 stable release by fall of the same year.

## Future Fixes

- Watch room WebSocket broadcast delivery should be made resilient to slow clients. The current server-side broadcast path writes to each socket serially, so one slow connection can delay room events for everyone else. A future version should move watch room clients to dedicated buffered outbound queues with a single writer loop per client and non-blocking broadcast fan-out.

- `web/src/routes/_admin/settings/libraries.lazy.tsx` — Movies scan cache invalidation is eager: after `triggerMovieScan()` resolves, `queryClient.invalidateQueries` is called immediately for `MOVIES_STATS_KEY`, `MOVIES_KEY`, `LATEST_MOVIES_KEY`, `MOVIE_PLAYLISTS_KEY`, `MOVIE_PLAYLIST_DETAILS_KEY`, and `MOVIE_PLAYLIST_MOVIES_KEY`, before the background scan has actually finished. The fix is to remove those invalidation calls from the `triggerMovieScan` success branch and instead poll a scan-status endpoint (e.g. `checkMovieScanStatus`) until it returns completed or failed, then run the invalidations and call `showSuccess` or `showActionFailed` accordingly. Alternatively, a WebSocket or server-sent event on scan completion could trigger the same invalidation. `startTransition`, `triggerMovieScan`, and the toast helpers should remain; only the invalidation timing changes.

- `web/src/routes/_admin/settings/libraries.lazy.tsx` — The movies stats section treats backend failures as an absent library: `const stats = statsData?.error === false ? statsData.data : null` silently falls back to `null`, which causes the UI to render "0 Movies" when the stats endpoint returns an error. Fix by setting `stats` to `undefined` only when `statsData?.error === false` holds, and add an explicit error/unavailable state to the stats rendering block: show a loading spinner while `statsLoading` is true, show an error message when `statsData?.error === true`, and only fall back to `0` when `stats` is a valid resolved object.

## Features

- Movies and TV shows: library scanning, metadata enrichment, artwork, trailers where available, technical media details, watch progress, direct streaming, and HLS playback with hardware-accelerated transcoding where supported
- Music: albums, tracks, musicians, playlists with collaborators, liked tracks, listening statistics, and cover art; metadata and cover enrichment use Spotify when it is configured (Spotify is optional), and when Spotify is not configured the system relies on basic file metadata only
- Accounts and settings: session-based authentication with SQLite-backed users and application settings
- Multi-client platform: the server exposes APIs used by the built-in web client in this repository and by future dedicated TV and mobile clients

## Repository Layout

| Path           | Purpose                                                                            |
| -------------- | ---------------------------------------------------------------------------------- |
| `server/`      | Go server, API, embedded schema, sqlc queries, and media tooling wrappers          |
| `server/sqlc/` | SQL schema and queries; generated Go code lives in `server/cmd/internal/database/` |
| `web/`         | React-based web client built and served by the Igloo server                        |

## Docker Deployment

The recommended way to run Igloo in production is with Docker Compose. A pre-built image is published to GitHub Container Registry on every versioned release and can be pulled without cloning the repository.

### Quick start

```bash
# Download the two required files
curl -O https://raw.githubusercontent.com/jibanez74/Igloo/main/compose.yaml
curl -O https://raw.githubusercontent.com/jibanez74/Igloo/main/.env.example

# Create your environment file
cp .env.example .env
```

Edit `.env` and set at minimum:

- `MOVIES_DIR`, `SHOWS_DIR`, `MUSIC_DIR` — absolute paths to your media on the host
- `DEFAULT_ADMIN_EMAIL` and `DEFAULT_ADMIN_PASSWORD` — credentials for the first login

Then prepare the data directories and start the server:

```bash
mkdir -p ./config ./transcode
chown -R 1000:1000 ./config ./transcode

docker compose --profile cpu up -d
```

The server will be available on port `8080` by default. On a fresh database it creates one admin account using the credentials from your `.env` file.

### Hardware acceleration profiles

The compose file ships three profiles. Pick the one that matches your host:

| Profile  | Command                                 | Requirements                                                                                                                              |
| -------- | --------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| `cpu`    | `docker compose --profile cpu up -d`    | No GPU required                                                                                                                           |
| `nvidia` | `docker compose --profile nvidia up -d` | [nvidia-container-toolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html) installed on host |
| `intel`  | `docker compose --profile intel up -d`  | Set `RENDER_GROUP_ID` in `.env` first (see below)                                                                                         |

For Intel QSV, find your render group ID and add it to `.env`:

```bash
getent group render | cut -d: -f3
# Add the result to .env: RENDER_GROUP_ID=<number>
```

### Updating to a new release

```bash
docker compose --profile cpu pull
docker compose --profile cpu up -d
```

Replace `cpu` with whichever profile you use.

### Volume permissions

The container runs as a non-root user (`igloo`, UID/GID 1000). The `config` and `transcode`
directories on the host must be owned by that UID before first start:

```bash
chown -R 1000:1000 ./config ./transcode
```

Media directories are mounted read-only and do not need this change.

---

## Development Setup

### Prerequisites

- Go: version aligned with `server/go.mod`
- CGO: required for `github.com/mattn/go-sqlite3` (`CGO_ENABLED=1`)
- SQLite development libraries: required for CGO linking
- Bun: for installing and running the web client in `web/`
- sqlc: for generating database code from SQL

### FFmpeg and ffprobe

The Docker build downloads and embeds the correct Jellyfin FFmpeg binaries automatically. For local development you need to supply them manually.

Platform-specific binaries belong under `server/cmd/internal/ffmpeg/` and `server/cmd/internal/ffprobe/`, named to match the `//go:embed` directives in the corresponding build tag files (e.g. `ffmpeg_darwin_arm64` for `ffmpeg_darwin_arm64.go`).

This project uses the [Jellyfin FFmpeg builds](https://github.com/jellyfin/jellyfin-ffmpeg/releases) because they include codec and hardware acceleration support beyond what standard FFmpeg distributions provide.

### 1. Create `server/.env`

The server calls `godotenv.Load()` on startup to read `server/.env`. If the file is missing it logs a warning and continues, relying on environment variables already present in the process. For local development, create the file:

```env
PORT=8080
DEBUG=true
DB_PATH=igloo.db
STATIC_DIR=static
LOGS_DIR=logs
TMDB_API_KEY=your_tmdb_v3_key
SPOTIFY_CLIENT_ID=your_spotify_client_id
SPOTIFY_CLIENT_SECRET=your_spotify_client_secret
MOVIES_DIR=/path/to/movies
MUSIC_DIR=/path/to/music
SHOWS_DIR=/path/to/shows
DOWNLOAD_IMAGES=true
ENABLE_LOGGER=true
ENABLE_WATCHER=false
HARDWARE_ACCELERATION_DEVICE=cpu
```

Notes:

- `HARDWARE_ACCELERATION_DEVICE`: `cpu`, `apple`, `nvidia`, or `intel`
- `TMDB_API_KEY`: enables movie matching, in-theaters data, and background movie scanning when `MOVIES_DIR` is configured
- `SPOTIFY_CLIENT_ID` and `SPOTIFY_CLIENT_SECRET`: optional, but required if you want Spotify-based music metadata and cover enrichment
- `JELLYFIN_TOKEN`: optional and only relevant if you are using Jellyfin-related integrations

### 2. Start the backend

From `server/`:

```bash
make dev
```

This runs sqlc generation, syncs the schema into `cmd/api`, and starts the server with `VITE_DEV_SERVER=http://localhost:3000` so the backend hands browser requests off to the Vite app during development.

### 3. Start the web client

From `web/` in another terminal:

```bash
bun install
bun run dev
```

Open the URL printed by Vite, usually `http://localhost:3000`.

### Default admin account

On a fresh database the server creates one admin account:

- Email: `admin@sample.com`
- Password: `AdminPassword`

Change this password immediately after first login. These credentials are only intended as a bootstrap account for local setup. In Docker, override them with `DEFAULT_ADMIN_EMAIL` and `DEFAULT_ADMIN_PASSWORD` in `.env`.

---

## Production Build (without Docker)

In production, the web client is built and embedded into the server binary.

Build the full server with embedded web assets:

```bash
cd server
make build-full
```

This process:

1. Generates sqlc code
2. Builds the web client into `web/dist`
3. Copies the built assets into `server/cmd/api/webdist/`
4. Builds the `igloo-server` binary

Backend-only build:

```bash
cd server
make build
```

Use this only if you are handling web assets separately or copying them yourself.

## Useful Make Targets

| Target                                | Description                                                          |
| ------------------------------------- | -------------------------------------------------------------------- |
| `make dev`                            | Generate sqlc code and run the API with Vite development URL support |
| `make generate`                       | Run `sqlc generate` and sync `schema.sql` into `cmd/api`             |
| `make build`                          | Build the `igloo-server` binary for the current platform             |
| `make build-web`                      | Build the web client into `web/dist`                                 |
| `make build-full`                     | Build the web client and embed it into the server binary             |
| `make build-mac` / `make build-linux` | Cross-compile the backend                                            |
| `make test`                           | Run `go test -v ./...`                                               |
| `make clean`                          | Remove generated binaries and `web/dist`                             |

## Database and SQL

- Engine: SQLite with WAL journaling
- Database path: controlled by `DB_PATH` (default `igloo.db` in development, `/config/igloo.db` in Docker)
- Schema source of truth: `server/sqlc/schema.sql`
- Embedded schema copy: `server/cmd/api/schema.sql`
- Query files: `server/sqlc/queries/*.sql`

After changing schema or query files, run:

```bash
cd server
make generate
```

## Frontend Scripts

| Script            | Description                                |
| ----------------- | ------------------------------------------ |
| `bun run dev`     | Start the Vite development server          |
| `bun run build`   | Type-check and build the production bundle |
| `bun run lint`    | Run ESLint                                 |
| `bun run preview` | Preview the production build               |

## Configuration Reference

All configuration is read from environment variables. In local development these are loaded from `server/.env`. In Docker they are passed through `compose.yaml` (see `.env.example` for the full list with descriptions).

| Variable                                 | Role                                                                     |
| ---------------------------------------- | ------------------------------------------------------------------------ |
| `PORT`                                   | HTTP listen port                                                         |
| `DB_PATH`                                | SQLite database file path                                                |
| `DEBUG`                                  | Enables debug-friendly behavior such as stdout logging                   |
| `STATIC_DIR`                             | Static file directory for avatars, downloaded images, and related assets |
| `LOGS_DIR`                               | Log directory when not running in debug mode                             |
| `TMDB_API_KEY`                           | TMDB API v3 key                                                          |
| `SPOTIFY_CLIENT_ID`                      | Spotify client ID for optional music metadata enrichment                 |
| `SPOTIFY_CLIENT_SECRET`                  | Spotify client secret for optional music metadata enrichment             |
| `JELLYFIN_TOKEN`                         | Optional Jellyfin integration token                                      |
| `MOVIES_DIR` / `SHOWS_DIR` / `MUSIC_DIR` | Media library root directories                                           |
| `DOWNLOAD_IMAGES`                        | Controls whether remote images are downloaded during scanning            |
| `ENABLE_LOGGER` / `ENABLE_WATCHER`       | Feature flags for file logging and filesystem watchers                   |
| `HARDWARE_ACCELERATION_DEVICE`           | Transcoding target: `cpu`, `apple`, `nvidia`, or `intel`                 |
| `SESSION_COOKIE_SECURE`                  | Set `true` when running behind HTTPS (e.g. Tailscale)                    |
| `LOG_TO_STDOUT`                          | Force log output to stdout regardless of other settings                  |
| `VITE_DEV_SERVER`                        | Development only — proxies browser requests to the Vite dev server       |
| `DEFAULT_ADMIN_NAME`                     | Name for the bootstrap admin account (used only on first run)            |
| `DEFAULT_ADMIN_EMAIL`                    | Email for the bootstrap admin account (used only on first run)           |
| `DEFAULT_ADMIN_PASSWORD`                 | Password for the bootstrap admin account (used only on first run)        |

## CI/CD

Two GitHub Actions workflows run on every push:

### CI (`ci.yml`)

Runs on every branch push and on pull requests to `main`. Never publishes anything.

- `test-backend` — runs `go test -v ./...` against the Go server
- `test-frontend` — runs ESLint and a full TypeScript + Vite build of the web client

### Publish (`docker-publish.yml`)

Runs only when a `v*` tag is pushed. Tests must pass before the image is built.

```
test-backend  ┐
              ├── build-and-push → ghcr.io/jibanez74/igloo
test-frontend ┘
```

Pushing `v1.2.3` produces three image tags: `:v1.2.3`, `:1.2`, and `:latest`.

To cut a release:

```bash
git tag v0.1.0
git push origin v0.1.0
```

The published image appears at `ghcr.io/jibanez74/igloo` under the repository's Packages tab.

## Testing

From `server/`:

```bash
make test
```

This runs `go test -v ./...`.
