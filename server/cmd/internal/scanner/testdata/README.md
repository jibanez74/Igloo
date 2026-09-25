# Video probe fixtures

Synthetic fixtures generated with the repository's Jellyfin FFmpeg `7.1.4-Jellyfin` Linux x64 payload. No third-party media is included.

- `moving.avi`: 0.6-second moving MJPEG video at absolute stream index 1, with PCM audio at index 0.
- `artwork-with-video.mp4`: H.264 video and AAC audio, plus attached MJPEG artwork. Only the moving video is persisted as movie video.
- `artwork-only.mp4`: one attached MJPEG picture and no accepted video. Import must roll back all movie state.

Regenerate from the repository root, setting `IGLOO_FFMPEG_PATH` to the matching Jellyfin payload for Linux x64 or macOS ARM64:

```sh
"$IGLOO_FFMPEG_PATH" -v error -y -f lavfi -i sine=frequency=440:sample_rate=8000 -f lavfi -i testsrc2=size=64x48:rate=5 -map 0:a -map 1:v -t 0.6 -c:a pcm_s16le -c:v mjpeg -threads 1 server/cmd/internal/scanner/testdata/moving.avi
"$IGLOO_FFMPEG_PATH" -v error -y -f lavfi -i color=red:size=32x32 -frames:v 1 -threads 1 /tmp/igloo-cover.jpg
"$IGLOO_FFMPEG_PATH" -v error -y -f lavfi -i testsrc2=size=64x48:rate=5 -f lavfi -i sine=frequency=440:sample_rate=8000 -i /tmp/igloo-cover.jpg -map 0:v -map 1:a -map 2:v -t 0.6 -c:v:0 libx264 -threads 1 -c:a aac -c:v:1 copy -disposition:v:1 attached_pic server/cmd/internal/scanner/testdata/artwork-with-video.mp4
"$IGLOO_FFMPEG_PATH" -v error -y -i /tmp/igloo-cover.jpg -map 0:v -c:v copy -disposition:v attached_pic server/cmd/internal/scanner/testdata/artwork-only.mp4
```

`TestMovieRealProbeVideoAndArtwork` uses the real ffprobe wrapper and the scanner database transaction. Run from `server/` with `IGLOO_FFPROBE_PATH` set to the matching Jellyfin payload:

```sh
CGO_ENABLED=1 go test -count=1 -tags 'sqlite_fts5 externalbin' ./cmd/internal/scanner/movie -run TestMovieRealProbeVideoAndArtwork -v
```

Validation on 2026-09-08: Linux x64 passed using Jellyfin ffprobe `7.1.4-Jellyfin`. macOS ARM64 execution is unavailable in this Linux environment and remains unverified. These tests cover probing and import, not browser playback or hardware transcoding.
