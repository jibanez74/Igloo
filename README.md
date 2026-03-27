# Igloo

Igloo is a self-hosted media center server inspired by Plex and Jellyfin, built for accessible, high-quality playback of personal media libraries.

This project is currently in active development and has not yet reached its first stable release. Igloo is being built as a focused media server platform for movies, TV shows, personal videos, and music, with multiple clients planned over time.

This repository contains the Igloo server, including the Go backend, APIs, media indexing and management logic, playback and transcoding workflows, and the React-based web client that the server builds and serves in production. The web client is part of this repository, but it is only one client of the platform. Dedicated TV and mobile clients are planned as separate applications that will connect to the same server.

Igloo exists in part because current media servers, while powerful, still leave important accessibility gaps. The goal is to build a more modern and more inclusive system that works especially well for people who value accessible interfaces and dependable local media playback.

Igloo is intentionally focused on personal media libraries and local playback. Rather than trying to reproduce every feature found in larger media platforms, it focuses on a smaller set of core capabilities done well. Features such as live TV, torrent integrations, and large plugin ecosystems are outside the current scope.

For video playback, Igloo supports direct streaming when a file can be played in its original format, preserving full quality. When direct playback is not possible or not ideal for the device or network, Igloo can transcode to HLS and offer multiple bitrate options to adapt to different connection speeds.

For photos, Igloo is intended to integrate with Immich instead of duplicating functionality that Immich already handles well. Rather than building a separate photo platform from scratch, the long-term goal is to connect Igloo with Immich as part of a broader self-hosted media ecosystem.

---

## Project status

Igloo is currently in full development and has not yet reached its first stable release. APIs, features, playback workflows, and client applications are still evolving.

The current focus is the server platform and the React web client contained in this repository. Dedicated TV and mobile clients are planned as separate projects.

Expect a full tv client to be available by end of April, with a mobile client comming later this year (2026). Once they are available, this readme file will be updated with more information about this clients and URLs on how to get them.

---

## Features

- **Movies and TV Shows** — Library scanning, metadata enrichment, artwork, trailers where available, technical media details, watch progress, direct streaming, and HLS playback with hardware-accelerated transcoding where supported.
- **Music** — Albums, tracks, musicians, playlists with collaborators, liked tracks, listening statistics, and cover art sourced via MusicBrainz or Spotify where configured.
- **Accounts and settings** — Session-based authentication with SQLite-backed users and application settings.
- **Multi-client platform** — The server exposes APIs used by the built-in web client in this repository and by future dedicated TV and mobile clients.

---

## Repository layout

| Path           | Purpose                                                                            |
| -------------- | ---------------------------------------------------------------------------------- |
| `server/`      | Go server, API, embedded schema, sqlc queries, and media tooling wrappers          |
| `server/sqlc/` | SQL schema and queries; generated Go code lives in `server/cmd/internal/database/` |
| `web/`         | React-based web client built and served by the Igloo server                        |

Large **FFmpeg** and **ffprobe** binaries are **not** committed to the repository. You need platform-specific binaries under `server/cmd/internal/ffmpeg/` and `server/cmd/internal/ffprobe/` that match the `//go:embed` files expected by the build tags, such as `ffmpeg_darwin_arm64.go` and `ffprobe_darwin_arm64.go`. Follow your normal workflow for placing those binaries before building.

Currently this project uses the ffmpeg binaries provided by the Jellyfin project because they have some very nice features that are nice to have for media transcoding. You can find them at:

---

## Prerequisites

- **Go** — Version aligned with `server/go.mod`
- **CGO** — Required for `github.com/mattn/go-sqlite3` (`CGO_ENABLED=1`)
- **SQLite** — Development libraries for CGO linking
- **Bun** — For installing and running the web client in `web/`
- **sqlc** — For generating database code from SQL
- **`.env` file** — The server loads `server/.env` on startup and exits if it is missing

---

## Quick start (development)

Development uses two processes:

- **Backend** — Go server, default port `8080`
- **Frontend** — Vite development server, default port `3000`

During development, Vite proxies `/api` requests to `http://localhost:8080`.

### 1. Create `server/.env`

Minimal example:

```env
PORT=8080
DEBUG=true
DB_PATH=igloo.db
STATIC_DIR=static
LOGS_DIR=logs
Optional but commonly useful:
TMDB_API_KEY=your_tmdb_v3_key
MOVIES_DIR=/path/to/movies
MUSIC_DIR=/path/to/music
SHOWS_DIR=/path/to/shows
DOWNLOAD_IMAGES=true
ENABLE_LOGGER=true
ENABLE_WATCHER=false
HARDWARE_ACCELERATION_DEVICE=cpu
Notes:
	•	HARDWARE_ACCELERATION_DEVICE — cpu, apple, nvidia, or intel
	•	TMDB_API_KEY — Enables movie matching, in-theaters data, and background movie scanning when MOVIES_DIR is configured
	•	JELLYFIN_TOKEN — Optional and only relevant if you are using Jellyfin-related integrations
2. Start the backend
From server/:
make dev
This runs sqlc generation, syncs the schema into cmd/api, and starts the server with VITE_DEV_SERVER=http://localhost:3000 so the server can hand off browser requests to the Vite app during development.
3. Start the web client
From web/ in another terminal:
bun install
bun run dev
Open the URL printed by Vite, usually http://localhost:3000.
Default admin user
On a fresh database, if no admin exists, the server creates:
	•	Email: admin@sample.com
	•	Password: AdminPassword

Production build
In production, the web client is built and embedded into the server binary.
Build the full server with embedded web assets:
cd server
make build-full
This process:
	1	Generates sqlc code
	2	Builds the web client into web/dist
	3	Copies the built assets into server/cmd/api/webdist/
	4	Builds the igloo-server binary
This is the recommended build for deployment when you want the server to deliver the web application directly.
Backend-only build:
cd server
make build
Use this only if you are handling web assets separately or copying them yourself.

Useful Make targets
Target
Description
make dev
Generate sqlc code and run the API with Vite development URL support
make generate
Run sqlc generate and sync schema.sql into cmd/api
make build
Build the igloo-server binary for the current platform
make build-web
Build the web client into web/dist
make build-full
Build the web client and embed it into the server binary
make build-mac / make build-linux
Cross-compile the backend
make test
Run go test -v ./...
make clean
Remove generated binaries and web/dist

Database and SQL
	•	Engine: SQLite
	•	Database path: Controlled by DB_PATH, default igloo.db
	•	Schema source of truth: server/sqlc/schema.sql
	•	Embedded schema copy: server/cmd/api/schema.sql
	•	Query files: server/sqlc/queries/*.sql
After changing schema or query files, run:
cd server
make generate

Frontend scripts
Script
Description
bun run dev
Start the Vite development server
bun run build
Type-check and build the production bundle
bun run lint
Run ESLint
bun run preview
Preview the production build

Configuration reference
Variables are read from server/.env when present and may also be provided through the process environment.
Variable
Role
PORT
HTTP listen port
DB_PATH
SQLite database file path
DEBUG
Enables debug-friendly behavior such as stdout logging
STATIC_DIR
Static file directory for avatars, downloaded images, and related assets
LOGS_DIR
Log directory when not running in debug mode
TMDB_API_KEY
TMDB API v3 key
JELLYFIN_TOKEN
Optional Jellyfin integration token
MOVIES_DIR / SHOWS_DIR / MUSIC_DIR
Media library root directories
DOWNLOAD_IMAGES
Controls whether remote images are downloaded
ENABLE_LOGGER / ENABLE_WATCHER
Feature flags for logging and watchers
HARDWARE_ACCELERATION_DEVICE
Transcoding target: cpu, apple, nvidia, or intel
VITE_DEV_SERVER
Development URL used to hand off browser requests to the Vite app

Testing
cd server
go test ./...
Some tests may rely on fixtures or external APIs depending on the package being tested.
```
