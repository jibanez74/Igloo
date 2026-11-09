# FFmpeg fixtures

Synthetic fixtures; no third-party media is included.

- `hevc-hdr10-open-gop.mkv`: 9 seconds of 256x144 HEVC Main 10 (`yuv420p10le`) tagged HDR10 (BT.2020, PQ, mastering display and content light level SEI), with stereo AAC. x265's open GOPs make every keyframe after the first a CRA picture (NAL type 21), the way Blu-ray HEVC is coded, so the HEVC remux test proves the copy validates on CRA sync samples, not only on IDR ones.

Igloo's own FFmpeg build has no HEVC encoder, so this fixture was generated once with a libx265-enabled FFmpeg (Ubuntu 22.04's `4.4.2-0ubuntu0.22.04.1`), from the repository root:

```sh
ffmpeg -v error -y -f lavfi -i testsrc2=size=256x144:rate=24 -f lavfi -i sine=frequency=440:sample_rate=48000 -t 9 \
  -map 0:v -map 1:a -c:v libx265 -pix_fmt yuv420p10le \
  -x265-params "log-level=error:crf=40:keyint=48:min-keyint=48:open-gop=1:scenecut=0:colorprim=bt2020:transfer=smpte2084:colormatrix=bt2020nc:hdr10=1:master-display=G(13250,34500)B(7500,3000)R(34000,16000)WP(15635,16450)L(10000000,50):max-cll=1000,400:frame-threads=1:pools=1" \
  -c:a aac -b:a 32k -ac 2 server/cmd/internal/ffmpeg/testdata/hevc-hdr10-open-gop.mkv
```

Check the picture types with the pinned FFmpeg; expect one type 20 (IDR_N_LP) and four type 21 (CRA):

```sh
"$IGLOO_FFMPEG_PATH" -v trace -i server/cmd/internal/ffmpeg/testdata/hevc-hdr10-open-gop.mkv -map 0:v -c copy \
  -bsf:v trace_headers -f null - 2>&1 | grep -E 'nal_unit_type +[0-9]+ = (1[6-9]|2[01])'
```
