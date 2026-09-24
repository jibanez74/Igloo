# Third-party notices

Igloo is licensed under the [GNU General Public License v3.0 or later](LICENSE). It includes or builds on the third-party software listed below, each under its own license. Those licenses continue to apply to their components.

## FFmpeg and ffprobe (Jellyfin FFmpeg)

Release binaries of Igloo embed `ffmpeg` and `ffprobe` executables from the [Jellyfin FFmpeg](https://github.com/jellyfin/jellyfin-ffmpeg) project. Igloo runs them as separate processes. They are not linked into Igloo's code.

- **License:** GNU General Public License (Jellyfin FFmpeg is built with GPL components enabled). Run `ffmpeg -version` and check the `configuration:` line for the exact options: `--enable-gpl` means GPL, and adding `--enable-version3` means GPLv3.
- **Source code:** the complete corresponding source for each embedded build is available from the matching tagged release at <https://github.com/jellyfin/jellyfin-ffmpeg/releases>. The current Linux x64 and macOS ARM64 payloads are both from the [`v8.1.2-5`](https://github.com/jellyfin/jellyfin-ffmpeg/releases/tag/v8.1.2-5) release, which reports version `8.1.2-Jellyfin`.
- FFmpeg is a trademark of Fabrice Bellard, originator of the FFmpeg project.

Development builds (`externalbin` tag) do not include FFmpeg. They use whatever `ffmpeg`/`ffprobe` is installed on the host.

## Inter typeface

The web app ships `web/public/fonts/InterVariable.woff2`.

- Copyright (c) 2016 The Inter Project Authors (<https://github.com/rsms/inter>)
- **License:** SIL Open Font License, Version 1.1. The full text is in [web/public/fonts/LICENSE.txt](web/public/fonts/LICENSE.txt).

## Go server dependencies

Compiled into the Igloo server binary. Module versions are pinned in [server/go.mod](server/go.mod).

| Module | License |
| --- | --- |
| github.com/alexedwards/scs/v2, scs/sqlite3store | MIT |
| github.com/go-chi/chi/v5 | MIT |
| github.com/gorilla/websocket | BSD-2-Clause |
| github.com/klauspost/compress | BSD-3-Clause |
| github.com/mattn/go-sqlite3 (bundles SQLite, public domain) | MIT |
| github.com/patrickmn/go-cache | MIT |
| github.com/zmb3/spotify/v2 | Apache-2.0 |
| golang.org/x/crypto, x/oauth2, x/sync, x/text | BSD-3-Clause |

`github.com/getkin/kin-openapi` (MIT) and its dependencies are used only by tests and are not part of the server binary.

## Web client dependencies

Bundled into the web app that the server embeds and serves. Versions are pinned in [web/bun.lock](web/bun.lock).

| Package | License |
| --- | --- |
| react, react-dom | MIT |
| @tanstack/react-query, @tanstack/react-router, @tanstack/react-virtual | MIT |
| @dnd-kit/core, @dnd-kit/modifiers, @dnd-kit/sortable, @dnd-kit/utilities | MIT |
| radix-ui | MIT |
| hls.js | Apache-2.0 |
| class-variance-authority | Apache-2.0 |
| lucide-react | ISC |
| clsx, sonner, tailwind-merge, zod | MIT |

These tables list direct runtime dependencies only. Binary releases must also include the full license texts and copyright notices of every component that ships in them.
