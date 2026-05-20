# Igloo

Igloo is a self-hosted media center for personal movie and music libraries. It is built for people who want to own their media, run their own server, and enjoy a polished playback experience without depending on a managed cloud platform.

Accessibility is one of Igloo's core design values, especially strong screen reader support, but Igloo is not meant to be a media platform only for blind users. The goal is to build a media center that feels excellent for everyone: fast, attractive, reliable, comfortable to navigate, and usable whether someone is browsing visually, using a keyboard, navigating with a remote, or relying on assistive technology.

The project started from a practical need: blind users should be able to manage and enjoy a media library independently, without inaccessible admin screens, missing navigation paths, or playback features that only work well for sighted users. That experience shapes Igloo's priorities, but the broader vision is universal: accessibility should raise the quality of the product for all users, not narrow its audience.

Igloo is intended to run on user-managed hardware, usually inside a private network or Tailscale tailnet. It is not designed around managed cloud hosting or public exposure.

Igloo is pre-v1 software. Expect API, database, configuration, and client behavior to change before a stable v1 release.

## What This Repository Contains

This repository contains the Igloo server and browser-based web client.

- `server/`: Go backend, chi API, SQLite startup schema, media scanning, playback endpoints, HLS support, database access, and FFmpeg/ffprobe integration.
- `web/`: React web client for browser-based library management, administration, and playback.
- `docs/`: OpenAPI documentation and project notes.
- `compose.yaml`: Docker Compose deployment with optional NVIDIA and Intel transcoding snippets.
- `Dockerfile`: Multi-stage production image build with the web client embedded into the server binary.

Native TV clients, including the planned Android TV / Google TV app, are not part of this repository.

## Platform Support

Current primary client:

- Web browser client, served by the Go server in production.

Planned clients:

- Native Android TV / Google TV client.
- Apple TV support is a future nice-to-have.

The current repository focuses on the server and web client. Future TV clients may live in separate repositories and consume the same documented HTTP API.

## Development Principles

Igloo prioritizes:

- Accessibility as a core requirement that improves the experience for everyone, not a final pass or a niche-only feature.
- Local ownership of media and metadata.
- Reliable playback before visual polish.
- Clear API contracts between server and clients.
- Self-hosted deployment over managed cloud assumptions.
- Practical maintainability over unnecessary abstraction.

## Current Status

Igloo is in active development and has not reached a stable v1 release. The server, API, database schema, and web client are all evolving.

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

## Repository Layout

| Path | Purpose |
| --- | --- |
| `server/` | Go server, chi API, SQLite startup schema, media scanning, playback, HLS, and FFmpeg/ffprobe integration |
| `server/sqlc/` | SQL schema and sqlc query files; generated Go code lives in `server/cmd/internal/database/` |
| `web/` | React 19 web client built with Vite, TanStack Router, TanStack Query, and Bun |
| `docs/` | OpenAPI documentation and project notes |
| `compose.yaml` | Docker Compose deployment with optional NVIDIA and Intel transcoding snippets |
| `Dockerfile` | Multi-stage production image build with the web client embedded into the server binary |

## Quick Start With Docker

Docker Compose is the recommended way to run Igloo.

If you only want to run the published image, download the compose file and example environment file:

```bash
curl -fsSLO https://raw.githubusercontent.com/jibanez74/Igloo/main/compose.yaml
curl -fsSLO https://raw.githubusercontent.com/jibanez74/Igloo/main/.env.example
cp .env.example .env
```

Edit `.env` before first start:

- Set `DEFAULT_ADMIN_EMAIL` and `DEFAULT_ADMIN_PASSWORD`.
- Set `SESSION_COOKIE_SECURE=false` if you are testing over plain HTTP, such as `http://localhost:8080`.
- Keep `SESSION_COOKIE_SECURE=true` when running behind HTTPS, including Tailscale Serve or a reverse proxy.
- Set `MOVIES_DIR`, `SHOWS_DIR`, and `MUSIC_DIR` to your host media paths, or leave the defaults in place until you are ready to point Igloo at real media folders.

Prepare writable directories for the container user:

```bash
mkdir -p ./config ./transcode
chown -R 1000:1000 ./config ./transcode
```

Pull and start the service:

```bash
docker compose pull
docker compose up -d
```

Igloo listens on port `8080` by default. Change the host port with `HOST_PORT` in `.env`.

The compose file also includes a local build definition. If you cloned the repository instead of downloading only `compose.yaml`, Docker Compose can build the image locally from the included `Dockerfile`.

## Hardware Acceleration

The default Compose service uses CPU software transcoding:

```bash
docker compose up -d
```

NVIDIA transcoding requires `nvidia-container-toolkit` on the host. See NVIDIA's [Container Toolkit installation guide](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html) for Docker setup instructions. To use it, set `HARDWARE_ACCELERATION_DEVICE=nvidia` in `.env`, uncomment the NVIDIA environment lines and runtime block in `compose.yaml`, then run:

```bash
docker compose up -d
```

Intel QSV transcoding requires access to `/dev/dri/renderD128` and the host render group ID. To use it, set `HARDWARE_ACCELERATION_DEVICE=intel`, set `RENDER_GROUP_ID`, uncomment the Intel block in `compose.yaml`, then run:

```bash
getent group render | cut -d: -f3
# Add the result to .env:
# RENDER_GROUP_ID=<number>

docker compose up -d
```

Apple VideoToolbox is supported for local server development builds on macOS, not through the Linux Docker image.

For implementation details, hardware acceleration behavior, and operational notes, see [docs/ffmpeg.md](docs/ffmpeg.md).

## Configuration

Igloo reads configuration from environment variables. Docker passes them through `compose.yaml`; local development loads `.env` from the repository root, with a fallback to `server/.env`.

The most important variables are:

| Variable | Purpose |
| --- | --- |
| `DEFAULT_ADMIN_NAME`, `DEFAULT_ADMIN_EMAIL`, `DEFAULT_ADMIN_PASSWORD` | Bootstrap admin account, used only when the database has no admin user |
| `HOST_PORT` | Host port for Docker, default `8080` |
| `SESSION_COOKIE_SECURE` | Set `true` behind HTTPS; set `false` for plain HTTP development |
| `CONFIG_DIR` | Docker host directory for the database, static assets, and logs |
| `TRANSCODE_DIR` | Docker host directory for temporary HLS transcode output |
| `MOVIES_DIR`, `SHOWS_DIR`, `MUSIC_DIR` | Host media directories mounted read-only into the container |
| `TMDB_API_KEY` | Optional TMDB API key for movie metadata and in-theaters data |
| `SPOTIFY_CLIENT_ID`, `SPOTIFY_CLIENT_SECRET` | Optional Spotify credentials for music metadata enrichment |
| `ENABLE_LOGGER`, `ENABLE_WATCHER`, `DOWNLOAD_IMAGES` | Runtime feature flags |
| `HARDWARE_ACCELERATION_DEVICE` | Transcode target: `cpu`, `apple`, `nvidia`, or `intel`; Docker defaults to `cpu` unless overridden |

See `.env.example` for the full reference and defaults.

## Development Setup

Prerequisites:

- Go `1.26.2`, matching `server/go.mod`
- CGO enabled, with a working C compiler
- SQLite support with FTS5 enabled; use the existing Make targets so the `sqlite_fts5` build tag is applied
- `sqlc` on your `PATH`
- Bun for the web client
- Platform-specific FFmpeg and ffprobe binaries for local non-Docker builds, or a `systembin` build that points at Jellyfin FFmpeg

Create your environment file:

```bash
cp .env.example .env
```

For local development, uncomment or set at least:

```env
DEBUG=true
PORT=8080
DB_PATH=/path/to/igloo.db
SESSION_COOKIE_SECURE=false
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
cd server
make dev
```

`make dev` runs sqlc generation, syncs the embedded schema copy, and starts the API with `VITE_DEV_SERVER=http://localhost:3000` so non-API browser requests are handed to Vite.

## Building Without Docker

Build the web client and embed it into the server binary:

```bash
cd server
make build-full
```

Build only the backend:

```bash
cd server
make build
```

Backend-only builds do not include a fresh web bundle. Use them when you are running the Vite client separately or handling web assets yourself.

## Useful Commands

From `server/`:

| Command | Description |
| --- | --- |
| `make dev` | Generate sqlc code and run the API for local development |
| `make generate` | Run `sqlc generate` and sync `server/sqlc/schema.sql` into `server/cmd/api/schema.sql` |
| `make build-web` | Build the web client into `web/dist` |
| `make build` | Build the backend for the current platform |
| `make build-full` | Build the web client and embed it into the server binary |
| `make test` | Run backend tests with the `sqlite_fts5` build tag |
| `make test-ci` | Run the deterministic backend suite used by GitHub Actions |
| `make clean` | Remove built binaries and `web/dist` |

From `web/`:

| Command | Description |
| --- | --- |
| `bun run dev` | Start Vite on port `3000` |
| `bun run build` | Build the production bundle and run TypeScript checking |
| `bun run lint` | Run ESLint |
| `bun run test` | Run Vitest |
| `bun run preview` | Preview the production build |

## API Documentation

The OpenAPI document lives at [docs/openapi.json](docs/openapi.json). It covers the registered `/api` routes, including JSON endpoints, static files, media streams, HLS playlists and segments, subtitles, and the watch-room WebSocket.

When adding or changing an API route, update the OpenAPI file and run the route coverage test:

```bash
cd server
go test -tags sqlite_fts5 ./cmd/api -run TestOpenAPIDocumentsRegisteredAPIRoutes -count=1
```

See [docs/openapi-maintenance.md](docs/openapi-maintenance.md) for the maintenance workflow.

## Database and SQL

- SQLite is the database engine.
- WAL mode is enabled at startup.
- `DB_PATH` controls the database file path; Docker defaults to `/config/igloo.db`.
- `server/sqlc/schema.sql` is the schema source of truth.
- `server/cmd/api/schema.sql` is the embedded startup schema copy.
- Query files live under `server/sqlc/queries/`.

After changing schema or query files:

```bash
cd server
make generate
```

## Testing

Backend:

```bash
cd server
make test
```

CI-equivalent backend suite:

```bash
cd server
make test-ci
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

GitHub Actions runs two main workflows:

- `ci.yml`: backend tests, frontend linting, and frontend build checks.
- `docker-publish.yml`: runs CI, then builds and publishes `ghcr.io/jibanez74/igloo` when a `v*` tag is pushed or the workflow is triggered manually.

Tagging a release such as `v0.1.0` produces semver image tags and updates `latest` for tagged releases.

```bash
git tag v0.1.0
git push origin v0.1.0
```

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
