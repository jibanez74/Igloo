# Igloo

The goal of this project is to build a modern, inclusive media system that prioritizes accessible interfaces and dependable local media playback. It is designed to deliver a high-quality experience for users who value reliability, flexibility, and full control over their media.
At its core, this system is hyper-focused on media playback, supporting a wide range of codecs while maintaining a strong commitment to accessibility—especially for blind users. As a blind developer, I created this system to ensure that I can fully manage and enjoy my own media environment without missing out on any features or relying on others. Accessibility is not treated as an afterthought, but as a fundamental requirement.
This project is also deeply personal. In my family, we value the experience of watching movies together, enjoying music, and revisiting photos and videos that hold meaningful memories. As the person responsible for managing our technology, I need tools that are both powerful and accessible—tools that won’t fail because of inaccessible interfaces or overlooked details. This system is built to meet that need, empowering not only me but other blind users to independently manage and enjoy their own media ecosystems.

Igloo is currently in active development and has not yet reached its first stable release. It is being built as a focused media server platform for movies, TV shows, personal videos, and music, with multiple clients planned over time.

This repository contains the Igloo server, including the Go backend, APIs, media indexing and management logic, playback and transcoding workflows, and the React-based web client. The web client is part of this repository, but it is only one client of the platform. Dedicated TV and mobile clients are planned as separate applications that will connect to the same server.

Igloo exists in part because current media servers, while powerful, still leave important accessibility gaps.

Igloo is intentionally focused on personal media libraries and local playback. Rather than trying to reproduce every feature found in larger media platforms, it focuses on a smaller set of core capabilities done well. Features such as live TV, torrent integrations, and large plugin ecosystems are outside the current scope.

For video playback, Igloo supports direct streaming when a file can be played in its original format, preserving full quality. When direct playback is not possible or not ideal for the device or network, Igloo can transcode to HLS and offer multiple bitrate options to adapt to different connection speeds.

For photos, Igloo is intended to integrate with Immich instead of duplicating functionality that Immich already handles well. Rather than building a separate photo platform from scratch, the long-term goal is to connect Igloo with Immich as part of a broader self-hosted media ecosystem.

## Project Status

APIs, features, playback workflows, and client applications are still evolving.

The current focus is the server platform and the React web client contained in this repository. Dedicated TV and mobile clients are planned as separate projects.

## Future Fixes

- Watch room WebSocket broadcast delivery should be made resilient to slow clients. The current server-side broadcast path writes to each socket serially, so one slow connection can delay room events for everyone else. A future version should move watch room clients to dedicated buffered outbound queues with a single writer loop per client and non-blocking broadcast fan-out.

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

## Prerequisites

- Go: version aligned with `server/go.mod`
- CGO: required for `github.com/mattn/go-sqlite3` (`CGO_ENABLED=1`)
- SQLite development libraries: required for CGO linking
- Bun: for installing and running the web client in `web/`
- sqlc: for generating database code from SQL
- `server/.env`: the server loads `server/.env` on startup and exits if it is missing

## FFmpeg and ffprobe

Large FFmpeg and ffprobe binaries are not committed to the repository. You need platform-specific binaries under `server/cmd/internal/ffmpeg/` and `server/cmd/internal/ffprobe/` that match the `//go:embed` files expected by the build tags, such as `ffmpeg_darwin_arm64.go` and `ffprobe_darwin_arm64.go`.

At the moment, this project uses FFmpeg binaries provided by the Jellyfin project because they include features that are useful for media transcoding. Add the appropriate binaries to those directories before building.

## Quick Start for Development

Development uses two processes:

- Backend: Go server on port `8080` by default
- Frontend: Vite development server on port `3000` by default

During development, Vite proxies `/api` requests to `http://localhost:8080`.

### 1. Create `server/.env`

Minimal example:

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

This runs sqlc generation, syncs the schema into `cmd/api`, and starts the server with `VITE_DEV_SERVER=http://localhost:3000` so the backend can hand browser requests off to the Vite app during development.

Before running this, make sure the required FFmpeg and ffprobe binaries are present under `server/cmd/internal/ffmpeg/` and `server/cmd/internal/ffprobe/` as described above.

### 3. Start the web client

From `web/` in another terminal:

```bash
bun install
bun run dev
```

Open the URL printed by Vite, usually `http://localhost:3000`.

### Default admin user

On a fresh database, if no admin exists, the server creates:

- Email: `admin@sample.com`
- Password: `AdminPassword`

Change this password immediately after first login. These credentials are only intended as a bootstrap account for local setup.

## Production Build

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

This is the recommended build for deployment when you want the server to deliver the web application directly.

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

- Engine: SQLite
- Database path: controlled by `DB_PATH`, default `igloo.db`
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

The server **requires** a valid `server/.env` file for normal startup: `server/cmd/api/main.go` calls `godotenv.Load()`, and if that call returns an error (for example when the file is missing or cannot be read), the process exits immediately with `log.Fatal(err)`. After a successful load, variables may still be overridden by the process environment where your deployment sets them.

| Variable                                 | Role                                                                     |
| ---------------------------------------- | ------------------------------------------------------------------------ |
| `PORT`                                   | HTTP listen port                                                         |
| `DB_PATH`                                | SQLite database file path                                                |
| `DEBUG`                                  | Enables debug-friendly behavior such as stdout logging                   |
| `STATIC_DIR`                             | Static file directory for avatars, downloaded images, and related assets |
| `LOGS_DIR`                               | Log directory when not running in debug mode                             |
| `TMDB_API_KEY`                           | TMDB API v3 key                                                          |
| `SPOTIFY_CLIENT_ID`                     | Spotify client ID for optional music metadata enrichment                 |
| `SPOTIFY_CLIENT_SECRET`                 | Spotify client secret for optional music metadata enrichment             |
| `JELLYFIN_TOKEN`                         | Optional Jellyfin integration token                                      |
| `MOVIES_DIR` / `SHOWS_DIR` / `MUSIC_DIR` | Media library root directories                                           |
| `DOWNLOAD_IMAGES`                        | Controls whether remote images are downloaded                            |
| `ENABLE_LOGGER` / `ENABLE_WATCHER`       | Feature flags for logging and watchers                                   |
| `HARDWARE_ACCELERATION_DEVICE`           | Transcoding target: `cpu`, `apple`, `nvidia`, or `intel`                 |
| `VITE_DEV_SERVER`                        | Development URL used to hand off browser requests to the Vite app        |

## Testing

From `server/`:

```bash
make test
```

This runs `go test -v ./...`.

Some tests may rely on fixtures or external APIs depending on the package being tested.
