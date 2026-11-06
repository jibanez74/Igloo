# Third-party notices

Igloo is licensed under the [GNU General Public License v3.0 or later](LICENSE). It includes or builds on the third-party software listed below, each under its own license. Those licenses continue to apply to their components.

## FFmpeg and ffprobe

Release binaries of Igloo embed `ffmpeg` and `ffprobe` executables. Igloo runs them as separate processes. They are not linked into Igloo's code. [`media-payloads/payloads.lock`](media-payloads/payloads.lock) pins each embedded executable. Both builds enable GPL components, so the executables are under the GNU General Public License. Run `ffmpeg -version` and check the `configuration:` line for the exact options: `--enable-gpl` means GPL version 2 or later, and adding `--enable-version3` means GPLv3.

- **Linux x64:** Igloo's own build of FFmpeg 8.1.3 (`8.1.3-Igloo`), made by [igloo-ffmpeg](https://github.com/jibanez74/igloo-ffmpeg). It is GPL version 2 or later.
  - It links x264 (GPL-2.0-or-later), dav1d (BSD-2-Clause), zimg (WTFPL), libvpl (MIT), libva and libdrm (MIT, loaded through [Implib.so](https://github.com/yugr/Implib.so) stubs, MIT), and zlib (zlib License). It also uses the nv-codec-headers (MIT).
  - It includes the `tonemap_cuda` patches from [Jellyfin FFmpeg](https://github.com/jellyfin/jellyfin-ffmpeg) (LGPL-2.1-or-later, like the FFmpeg files they change).
  - **Source code:** the [`media-8.1.3-igloo.3`](https://github.com/jibanez74/igloo-ffmpeg/releases/tag/media-8.1.3-igloo.3) release holds the complete corresponding source, `igloo-ffmpeg-8.1.3-sources.tar`: every source tarball, the patches and the build recipe.
- **macOS ARM64:** the [Jellyfin FFmpeg](https://github.com/jellyfin/jellyfin-ffmpeg) portable build from the [`v8.1.3-1`](https://github.com/jellyfin/jellyfin-ffmpeg/releases/tag/v8.1.3-1) release, which reports `8.1.3-Jellyfin`. The complete corresponding source is available from that release.
- FFmpeg is a trademark of Fabrice Bellard, originator of the FFmpeg project.

Development builds (`externalbin` tag) do not include FFmpeg. They use the `ffmpeg`/`ffprobe` on the host's `PATH`, or the pinned builds that `make media-bin` links.

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
| golang.org/x/crypto, x/sync, x/text | BSD-3-Clause |

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
