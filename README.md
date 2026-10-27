# Igloo

Igloo is a self-hosted media center for the movies, TV shows, and music you keep on your own server. It scans your local libraries and brings them together in a web app where you can browse, search, and play your collection. Movie and episode progress is saved per user, making it easy to pick up where you left off. You can also create collaborative playlists and watch movies with friends in synchronized watch rooms. Built with a Go server and a React web client, Igloo is designed for home servers and small personal setups.

The goal is a library that feels like your own, whether you're finding an old favorite or settling in for movie night.

Accessibility is part of that experience from the start. Keyboard navigation, visible focus, screen reader labels, skip links, and reduced-motion support are part of the [design system](docs/design-system.md).

Igloo is **pre-production**. Its API, configuration, database schema, and interface may change before v1. There are no database migrations or compatibility guarantees yet.

The Android TV and Fire TV client lives in the separate [Igloo TV repository](https://github.com/jibanez74/IglooTV). It is also pre-production; see that repository for its current features and build instructions.

## What works today

- **Movies:** Scan local files, browse details and technical information, enrich metadata with TMDB if configured, edit metadata as an admin, watch with direct playback or HLS, choose audio and supported text subtitles, and keep watch progress. You can like movies and make collaborative movie playlists.
- **TV shows:** Scan a show library, browse shows by season and episode, and play episodes with the same direct and HLS playback options as movies. Episode progress and watched status are saved per user. An up-next prompt helps you continue to the following episode.
- **Music:** Scan albums and tracks, browse musicians, play music, like tracks, see listening statistics, and make collaborative playlists. Albums bought on the iTunes Store are matched on Apple Music by the catalog IDs in their files; Spotify can optionally enrich the other albums and supply artist photos.
- **Finding things:** Search across movies, shows, albums, musicians, and tracks. The home page brings together recent additions, continue watching, watch rooms, and movie listings from TMDB when available.
- **Sharing and accounts:** Watch rooms synchronize movie playback over WebSockets. Accounts include an initial administrator, user management, account settings, avatars, profile PINs, Quick Connect device pairing, device management, and playback preferences. Members can send movie or music requests to admins through in-app notifications.

The web app includes light and dark themes. The Go server serves it in a production build; Vite serves it during development.

## Current limits

- The server targets **Linux x64** and **macOS 13 Ventura or newer on ARM64**. Windows, Docker deployment, and other Linux architectures are not supported at present.
- HLS transcoding and decoding are still works in progress. What plays smoothly depends on the source file, browser, FFmpeg build, and host hardware. See the [media guide](docs/ffmpeg.md) for supported paths and troubleshooting.
- Watch rooms support movies, not TV episodes.
- Photos are a coming-soon page.
- The library watcher (**Settings → General**) applies added, changed, moved, and deleted files within seconds on local Linux disks; libraries on network shares or FUSE mounts, and every library on macOS, are polled every few minutes instead. With it off, use a startup or manual scan after adding files.
- TMDB and Spotify are optional. Local scanning still works without their credentials, with less descriptive metadata and artwork. Apple Music lookups need no credentials and are on by default; they contact Apple only for files that carry iTunes catalog IDs. Turn them off in Settings, or set `ENABLE_APPLE_LOOKUPS=false` before the first start.

## Get started from source

### Prerequisites

On a supported host, install Git, Make, [Go 1.27.3](server/go.mod), a C compiler for CGO/SQLite, `sqlc`, Bun, `ffmpeg`, and `ffprobe`. Put the command-line tools on your `PATH`. The development and backend test targets require host FFmpeg tools; production builds have [additional media payload requirements](#build-a-production-binary).

### Set up the checkout

```bash
git clone https://github.com/jibanez74/Igloo.git
cd Igloo
cp .env.example .env
cd web
bun install --frozen-lockfile
cd ..
```

Open the root `.env` and set `DEFAULT_ADMIN_NAME`, `DEFAULT_ADMIN_EMAIL`, and a nonempty `DEFAULT_ADMIN_PASSWORD`. Those values create the first administrator. Keep `PORT=8080` for the development commands below, and use `SESSION_COOKIE_SECURE=false` for local HTTP. Set it to `true` when you serve Igloo over HTTPS.

You can set `MOVIES_DIR`, `SHOWS_DIR`, and `MUSIC_DIR` now, or add the paths later in **Settings → Libraries**. Leave libraries you do not use blank. Each path must already exist on the server and be readable by the Igloo process.

### Run the app

Start the web client in one terminal from the repository root:

```bash
cd web
bun run dev
```

Start the Go server in a second terminal from the repository root:

```bash
make dev
```

Open [http://localhost:3000](http://localhost:3000) and sign in with your administrator email and password. Vite proxies `/api` to the Go server on port 8080. `make dev` generates database code, builds the development server, and runs it with Vite enabled.

In **Settings → Libraries**, save your movie, TV show, and music paths, then start a scan for each library you want to use. Configured libraries also scan when the server starts; with the watcher on, it walks them instead. A scan can take time, and files being written are deferred until they have been quiet for 60 seconds. Run another scan after adding or changing files, or turn on the library watcher to apply changes as they happen. Saving a path by itself does not start a scan unless the watcher is on.

For TV files, use a layout such as `Show Name/Season 1/S01E02 - Episode Title.mkv`. `Specials` is supported for season zero. Combined episode files are catalogued as separate episodes, but playback runs the whole file because Igloo does not guess episode boundaries. More filename rules are in the [media guide](docs/ffmpeg.md#tv-show-scanning).

## Configuration

Igloo reads an optional `.env` from its **current working directory**; process environment variables take precedence. Relative paths also resolve from that directory. The file is runtime configuration and is not embedded in the binary. The [example file](.env.example) lists the main options.

Some values apply every time the server starts:

- `PORT` defaults to `8080`; `DB_PATH` defaults to `db/igloo.db`; `LOGS_DIR` defaults to `logs`.
- `SESSION_COOKIE_SECURE` controls HTTPS-only session cookies. `DEBUG` and `LOG_TO_STDOUT` control logging.
- `HLS_MAX_CPU_TRANSCODES` limits concurrent HLS sessions that encode video on the CPU, and `HLS_MAX_HW_TRANSCODES` limits those that encode video with a hardware encoder; copy-video sessions count against neither. `HLS_MAX_SESSIONS_PER_USER` limits personal HLS sessions, including remuxing. Their defaults are based on CPU count, `3`, and `3`, respectively.
- `IGLOO_FFMPEG_PATH` and `IGLOO_FFPROBE_PATH` can override executable lookup in development or other `externalbin` builds. Embedded production builds use their bundled tools.

Other values seed **Settings only when the database has no Settings row**. After the first start, change them in the app:

- `MOVIES_DIR`, `SHOWS_DIR`, and `MUSIC_DIR` choose library paths. `STATIC_DIR` and `TRANSCODE_DIR` choose artwork/upload storage and temporary HLS storage.
- `TMDB_API_KEY` enables movie and TV metadata enrichment. `SPOTIFY_CLIENT_ID` and `SPOTIFY_CLIENT_SECRET` enable music enrichment and Spotify search. `ENABLE_APPLE_LOOKUPS` (default `true`) controls matching iTunes Store purchases on Apple Music, which needs no credentials.
- `HARDWARE_ACCELERATION_DEVICE` defaults to `cpu`; `apple`, `intel`, and `nvidia` select supported host acceleration when FFmpeg and drivers allow it.
- `ENABLE_WATCHER` turns on the library watcher for a new database; afterwards the Settings switch decides. `DOWNLOAD_IMAGES` does the same for downloading movie and TV artwork, cast and crew photos, and network logos into `STATIC_DIR`, which Igloo then serves instead of the TMDB images; turning it off deletes them.

The `DEFAULT_ADMIN_*` values are used when there is no administrator; changing them later does not reset an existing account. Igloo creates application storage directories as needed, but it does not create media library directories. Make sure the process can write its database, static files, logs, and transcode directory.

### Playback notes

When possible, Igloo serves the original movie or episode file directly. Otherwise, FFmpeg can remux or transcode it to HLS. Alternate audio tracks may require HLS. Supported text subtitles can be converted to WebVTT; bitmap subtitles cannot. CPU transcoding is the default, and selected hardware acceleration falls back to CPU when unavailable.

For codec behavior, session limits, subtitles, HDR, and operational details, read the [FFmpeg and playback guide](docs/ffmpeg.md).

## Build a production binary

Production builds embed the web app and FFmpeg/ffprobe. In addition to the source prerequisites, install `zstd` and provide matching **Jellyfin FFmpeg** and ffprobe payloads for your host platform. Follow the [binary strategy](docs/ffmpeg.md#binary-strategy); generic upstream binaries are not the expected payloads.

Place the payloads at:

- Linux x64: `server/cmd/internal/ffmpeg/ffmpeg_linux_amd64` and `server/cmd/internal/ffprobe/ffprobe_linux_amd64`
- macOS ARM64: `server/cmd/internal/ffmpeg/ffmpeg_darwin_arm64` and `server/cmd/internal/ffprobe/ffprobe_darwin_arm64`

From the repository root, build and run in the foreground:

```bash
make build
./server/dist/igloo-server
```

The app is then available at [http://localhost:8080](http://localhost:8080) unless you changed `PORT`. Run the binary from the directory containing your runtime `.env`, or supply environment variables through your shell or service manager. Igloo does not look for `.env` beside the executable.

To build and run it in the background from the repository root, use `make start`. Stop that process with `make stop`. The background PID and output log are written in `server/dist/`.

## For contributors

The main areas of the repository are [server/](server/) for the Go API, SQLite database, scanners, and playback; [web/](web/) for the React/Vite client; and [docs/](docs/) for the design system, playback notes, and API contract.

Start with the [repository guidelines](AGENTS.md) and the relevant [backend](server/AGENTS.md) or [web](web/AGENTS.md) guidelines. The [design system](docs/design-system.md), [FFmpeg guide](docs/ffmpeg.md), and manually maintained [OpenAPI contract](docs/openapi.json) describe the current rules.

Bug reports, pull requests, and accessibility feedback are welcome.

Useful commands from the repository root:

- `make dev` runs the development API. `make dev-profile` also enables admin-only profiling endpoints.
- `make generate` regenerates SQLite access code with sqlc after schema or query changes.
- `make check` runs API contract checks, backend lint and tests, frontend lint and tests, and a frontend build.
- `make test-server`, `make test-web`, `make lint-web`, and `make build-web` run narrower checks.
- `make test-openapi` and `make check-openapi` check route coverage and generated frontend API types. Use `make generate-openapi` after changing the API schema.
- `make clean` stops the background app and removes build artifacts; it preserves `.env`, the database, and runtime media data.

With `make dev-profile`, sign in as an administrator and open `/api/debug/pprof/goroutineleak?debug=1` to inspect goroutines the runtime identifies as permanently blocked. Go 1.27 provides this profile without an experimental flag. The existing `/api/debug/pprof/goroutine` profile remains useful for inspecting all goroutines.

The SQLite schema is in [server/sqlc/schema.sql](server/sqlc/schema.sql), with queries in [server/sqlc/queries/](server/sqlc/queries/). Igloo currently changes the schema directly during pre-production rather than adding migrations. Do not edit generated database or OpenAPI types by hand.

Frontend unit tests use Vitest. Browser tests use Playwright; install Chromium from `web/` with `bun x playwright install --with-deps chromium`, then run `bun run test:e2e`. By default, Playwright starts Vite and a mock API. Set `E2E_BASE_URL` to a running Igloo URL for live-server suites, along with `E2E_ADMIN_EMAIL` and `E2E_ADMIN_PASSWORD` where needed. Run one spec with `bun run test:e2e e2e/<name>.spec.ts`, and `bun run test:e2e:boot` to check a production web build with the mock API. CI runs the whole mocked suite against the production build.

The [CI workflow](.github/workflows/ci.yml) runs code, contract, and mocked browser checks. It does not publish release binaries.

## License

Copyright (C) 2023–2026 Jose Ibañez

Igloo is free software: you can redistribute it and/or modify it under the terms of the GNU General Public License as published by the Free Software Foundation, either version 3 of the License, or (at your option) any later version.

Igloo is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the [GNU General Public License](LICENSE) for more details.

Igloo builds on third-party software, including the Jellyfin FFmpeg build embedded in release binaries and the Inter font. Their licenses and notices are listed in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
