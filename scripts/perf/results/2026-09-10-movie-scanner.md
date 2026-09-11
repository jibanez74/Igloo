# Movie scanner measurements — 2026-09-10

Linux x64, Jellyfin FFmpeg/ffprobe **7.1.4-Jellyfin**, 418 movie files on the configured CIFS library. Media access was read-only. SQLite databases were disposable, created for these measurements; no application database was opened or reset. TMDB was disabled. OS caches were left intact, and other validation ran in the workspace during part of the measurement.

| Measurement | Fresh import | Recovery rescan | Fully indexed rescan |
|---|---:|---:|---:|
| Total elapsed | 24m 22.436s | 1m 44.072s | 41.639s |
| Discovery | 8.863s | 7.240s | 7.093s |
| Local processing | 24m 13.425s | 1m 36.611s | 34.344s |
| Missing-file cleanup | 0.147s | 0.220s | 0.201s |
| Enrichment phase, provider disabled | <1ms | <1ms | <1ms |
| Files accounted for | 418 | 418 | 418 |
| Imported | 400 | 18 | 0 |
| Unchanged/skipped | 0 | 400 | 418 |
| Failed | 18 | 0 | 0 |
| Deferred / deleted | 0 / 0 | 0 / 0 | 0 / 0 |
| Logical read bytes, process and reaped probes | 1,418,963,236 | 67,872,503 | 156 |
| Physical read bytes reported by Linux | 1,150,976 | 0 | 0 |
| Physical write bytes, disposable DB and journals | 75,878,400 | 3,891,200 | 0 |
| Go allocations during pass | 78.57 MiB | 4.65 MiB | 1.37 MiB |
| Go live heap after pass | 1.10 MiB | 1.14 MiB | 1.93 MiB |
| Scanner RSS at end | 24.49 MiB | 24.55 MiB | 18.25 MiB |

All 18 initial failures imported successfully on the recovery scan without modifying the media. The first two passes used the same fresh database. The fully indexed measurement used a disposable copy of a snapshot made during recovery (407 movies), completed its remaining 11 imports, then scanned again. This follow-up was necessary because the first measurement harness stopped after two passes. The reusable benchmark now performs fresh, recovery, and unchanged passes and prints individual warnings/errors.

The fully indexed pass skipped all 418 files, ran no probes, and wrote no database pages. Its 156 logical read bytes are measurement overhead, not movie content. The sparse-file test additionally verifies metadata inspection leaves the descriptor offset at zero for files up to 1 TiB. Linux process I/O includes reaped ffprobe children; logical reads during imports therefore include probing and executable/library reads. CIFS network traffic is not represented fully by Linux block-device `read_bytes`. RSS is the scanner process, not the combined peak of both probe processes.

The slowest combined inspection/probe operations recorded in the initial run were:

| File | Elapsed |
|---|---:|
| The.Cabin.In.The.Woods.2011.mkv | 2m 17.151s |
| The.Evil.Dead.1981.mkv | 2m 16.926s |
| Talk.To.Me.2023.mkv | 1m 4.783s |
| The.Blair.Witch.Project.1999.mp4 | 1m 0.294s |
| Captain.America.Civil.War.2016.mkv | 27.906s |

These timings include filesystem inspection and catalog reads as well as subprocess work; they are not ffprobe runtime measurements alone. Exact first-pass error messages were captured only in the test logger's memory and were not retained in the original output, so no specific cause is assigned to those failures. Recovery was verified by subsequent successful imports and persisted baselines.

Validation included `make generate`, `make generate-openapi`, OpenAPI lint/route coverage, `make check`, scanner/TMDB/API race tests, and 11 Chromium Settings/playback tests. Real Jellyfin fixtures covered moving MJPEG, accepted video with artwork, and rejection of artwork-only files. Linux x64 was exercised; macOS ARM64 and hardware transcoding paths were unavailable in this session. No previous-implementation benchmark was captured.

Reproduction instructions are in [the performance README](../README.md#disposable-movie-scanner-benchmark).
