# Igloo FFmpeg build

This recipe builds Igloo's own `ffmpeg` and `ffprobe` for Linux x64. Igloo's Linux release binaries embed them, and CI runs the server tests against them. macOS ARM64 still embeds Jellyfin's portable build.

[`payloads.lock`](payloads.lock) pins every embedded payload, for both platforms, by download URL and sha256. `make media-payloads` runs [`fetch-payloads.sh`](fetch-payloads.sh), which installs the current platform's pair under `server/cmd/internal/{ffmpeg,ffprobe}/` and refuses a file that does not match its pin. `make build` runs it first. `make media-bin` also links the pair as `ffmpeg` and `ffprobe` in `server/dist/media-bin/`, for `PATH`.

## Build

```bash
media-build/build.sh          # needs only Docker; JOBS=<n> limits parallelism
media-build/build.sh --clean  # rebuild everything except the source cache
```

The build writes `out/linux_amd64/` with these files:

| File | Contents |
| --- | --- |
| `ffmpeg`, `ffprobe` | The binaries |
| `SHA256SUMS` | Checksums of both binaries |
| `BUILDINFO` | Toolchain, configure line, sources and patch digests |

It also writes `out/igloo-ffmpeg-<version>-sources.tar`. That file is the GPL corresponding source: every source tarball, the patches and this recipe. `cache/`, `work/` and `out/` are gitignored.

The build is reproducible: a clean build on a workstation and one on a GitHub runner gave byte-identical binaries. (`media-8.1.3-igloo.1` did not, because x264 embedded its OpenCL kernels in directory-listing order; x264 is now built without OpenCL.)

## Release

[`.github/workflows/media-build.yml`](../.github/workflows/media-build.yml) builds and checks the recipe on every push that changes it. Its run keeps the files as a workflow artifact for 14 days. Pushing a tag named `media-<FFmpeg version>-igloo.<n>`, such as `media-8.1.3-igloo.1`, also publishes them as a GitHub release:

| Release file | Contents |
| --- | --- |
| `ffmpeg_linux_amd64`, `ffprobe_linux_amd64` | The binaries |
| `BUILDINFO_linux_amd64` | The build's `BUILDINFO` |
| `igloo-ffmpeg-<version>-sources.tar` | The corresponding source |
| `SHA256SUMS` | Checksums of all of the above |

The workflow refuses a tag whose FFmpeg version differs from `sources.lock`. A release is never replaced: a rebuild of the same FFmpeg version gets the next `igloo.<n>`.

## What is built

- **Toolchain.** `Dockerfile` pins an AlmaLinux 8 image by digest, so the binaries need glibc 2.28 or newer, the same baseline as Jellyfin's portable builds. GCC comes from gcc-toolset-15 and clang (CUDA kernels to PTX) from AppStream.
- **Linking.** Everything is static except glibc. libva and libdrm are linked through [Implib.so](https://github.com/yugr/Implib.so) stubs that `dlopen` the host's libraries on first use. A host without VA-API can still run ffmpeg, and Intel QSV uses the host's VA-API stack and drivers. NVIDIA (ffnvcodec) and the oneVPL runtime are loaded by FFmpeg and libvpl at runtime anyway.
- **Sources.** `sources.lock` pins every source. The FFmpeg release tarball is pinned by sha256 and git dependencies by full commit. The dependency commits match Jellyfin's portable builder at `jellyfin-ffmpeg` tag `v8.1.3-1`, so a comparison with Jellyfin isolates the FFmpeg configuration.
- **Configure.** `scripts/build-linux.sh` holds the configure line. `--disable-autodetect` makes every external feature explicit. All native FFmpeg components stay enabled, because the library scanner and the test suite need broad decoder, demuxer and filter coverage. The external libraries are limited to what Igloo uses:

  | Library | Used for |
  | --- | --- |
  | x264 | CPU encode |
  | zimg | `zscale` HDR tone-mapping |
  | dav1d | software AV1 decode |
  | nv-codec-headers | NVENC/NVDEC and CUDA filters, via `cuda-llvm` |
  | libvpl, VA-API, libdrm | Intel QSV |
  | zlib | |

  Jellyfin enables more libraries that Igloo never calls: fdk-aac, x265, SVT-AV1, libvpx, OpenSSL, libass and fonts, OpenCL, Vulkan, libplacebo and others. This build leaves all of them out.
- **nv-codec-headers stays on the SDK 12.1-era commit.** Newer SDKs (13.x) require NVIDIA driver 610 or later.

## Vendored patches

`patches/` holds Jellyfin's CUDA tone-mapping filter, `tonemap_cuda`. Igloo's NVIDIA HDR chain uses it (`server/cmd/internal/ffmpeg/ffmpeg_hls.go`), and upstream FFmpeg has no equivalent. Patches 0002 to 0004 come from [jellyfin-ffmpeg](https://github.com/jellyfin/jellyfin-ffmpeg) tag `v8.1.3-1`, `debian/patches/` (still unchanged at `v8.1.3-2`). Patch 0005 is Igloo's own and fixes bugs in them, so 0002 to 0004 stay byte-identical to Jellyfin's:

| File | Source | sha256 |
| --- | --- | --- |
| `0002-update-cuda-func-header.patch` | unchanged | `ac3ddd01…f0a820` |
| `0003-dither-matrix-from-add-enhanced-cuda-pixfmt-converter-impl.patch` | only the `libavfilter/dither_matrix.h` section of `0003-add-enhanced-cuda-pixfmt-converter-impl.patch`, which `tonemap_cuda` includes; the rest of 0003 replaces upstream's `scale_cuda` and is left out | `39349e5e…b92119` |
| `0004-add-cuda-tonemap-impl.patch` | unchanged | `867aed32…c414f` |
| `0005-igloo-fix-tonemap-cuda-error-handling.patch` | Igloo's own fixes on top of 0002–0004: a failed kernel launch or buffer allocation fails the graph instead of sending an empty frame to the encoder, recompiles no longer leak the Dolby Vision buffers and LUT or drop the dither texture, and a Dolby Vision peak of 0 falls back to the default | `907d96ed…498c8d` |

Like the files they patch, all four are FFmpeg code under the LGPL. The build applies them with `--fuzz=0`, so a patch that no longer fits fails the build instead of being applied loosely.

## Verify

```bash
python3 -I media-build/verify/check-features.py media-build/verify/required-features.txt \
  media-build/out/linux_amd64/ffmpeg server/cmd/internal/ffmpeg/ffmpeg_linux_amd64
python3 -I media-build/verify/ffprobe-parity.py <reference ffprobe> media-build/out/linux_amd64/ffprobe \
  <report.json> <media dir>...
```

- `required-features.txt` lists every component Igloo's production code and `make test-server` use. `check-features.py` fails when one is missing, and with a reference build it also prints the full component diff.
- `ffprobe-parity.py` runs Igloo's exact metadata probes with both binaries and diffs only the fields Igloo reads.
- `check-linkage.sh` fails when a binary links anything outside glibc or needs a glibc newer than 2.28.
- `nvidia-parity.sh` runs on a host with an NVIDIA GPU. It runs Igloo's NVENC and CUDA capability probes, then times its SDR (`scale_cuda`) and HDR (`tonemap_cuda`) session chains with both builds and samples each one's picture. It stops if NVENC does not open, and it skips the benchmarks while NVENC is in use.

To run the server test suite on a local build before it is released, point `IGLOO_FFMPEG_PATH` and `IGLOO_FFPROBE_PATH` at `out/linux_amd64/`. Also put a directory of `ffmpeg`/`ffprobe` symlinks first on `PATH`, because the Makefile checks for both commands on `PATH`.

## Bumping

1. Update the FFmpeg line in `sources.lock`. Verify the new tarball's signature before you record its sha256:

   ```bash
   gpg --verify ffmpeg-<v>.tar.xz.asc ffmpeg-<v>.tar.xz
   ```

   The key is FFmpeg's release key, fingerprint `FCF9 86EA 15E6 E293 A564 4F10 B432 2F04 D676 58D8`, from <https://ffmpeg.org/ffmpeg-devel.asc>.
2. Refresh 0002 to 0004 in `patches/` from the jellyfin-ffmpeg tag for the same upstream release, and update the table above. Then check that 0005 still applies; if Jellyfin changed the code it touches, regenerate it against the new tree, or drop the fixes Jellyfin made itself.
3. Rebuild, then run the verify scripts and `make check` against the new build. On a host with an NVIDIA GPU, run `nvidia-parity.sh` with the previous release as the reference.
4. Commit, push, and push the next `media-<version>-igloo.<n>` tag. The workflow publishes the release.
5. Point the Linux lines of `payloads.lock` at the new release and its sha256s, which match `SHA256SUMS`. Then run `make media-payloads` and `make check`, and update the version references in `docs/ffmpeg.md` and `THIRD_PARTY_NOTICES.md`.
