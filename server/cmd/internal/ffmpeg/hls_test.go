package ffmpeg

import (
  "strings"
  "testing"

  "igloo/cmd/internal/helpers"
)

func TestBuildHLSArgsForTest(t *testing.T) {
  sourcePath := "/path/to/movie.mkv"
  outDir := "/tmp/igloo-hls-abc"

  t.Run("valid profile and cpu device", func(t *testing.T) {
    args, err := BuildHLSArgsForTest(sourcePath, outDir, helpers.HLSProfile1080p4Mbps, 0, 0, helpers.HARDWARE_ACCELERATION_DEVICE_CPU, false)
    if err != nil {
      t.Fatal(err)
    }
    argStr := strings.Join(args, " ")
    // Input and paths present
    if !strings.Contains(argStr, sourcePath) {
      t.Errorf("args should contain source path %q", sourcePath)
    }
    if !strings.Contains(argStr, outDir) {
      t.Errorf("args should contain out dir %q", outDir)
    }
    // HLS fMP4 options (main plan §5.11)
    for _, want := range []string{"-f", "hls", "-hls_segment_type", "fmp4", "-hls_playlist_type", "vod", "-hls_list_size", "0", "playlist.m3u8", "init.mp4", "segment_"} {
      if !strings.Contains(argStr, want) {
        t.Errorf("args should contain %q", want)
      }
    }
    // CPU: libx264, no hwaccel
    if !strings.Contains(argStr, "libx264") {
      t.Errorf("expected libx264 for cpu device")
    }
    // Map for video and audio track 0
    if !strings.Contains(argStr, "-map") {
      t.Errorf("expected -map for video/audio")
    }
  })

  t.Run("fast path uses copy", func(t *testing.T) {
    args, err := BuildHLSArgsForTest(sourcePath, outDir, helpers.HLSProfile720p3Mbps, 0, 0, helpers.HARDWARE_ACCELERATION_DEVICE_CPU, true)
    if err != nil {
      t.Fatal(err)
    }
    argStr := strings.Join(args, " ")
    if !strings.Contains(argStr, "-c:v") || !strings.Contains(argStr, "copy") {
      t.Errorf("fast path should use -c:v copy")
    }
    if !strings.Contains(argStr, "-c:a") || !strings.Contains(argStr, "copy") {
      t.Errorf("fast path should use -c:a copy")
    }
  })

  t.Run("apple device uses videotoolbox", func(t *testing.T) {
    args, err := BuildHLSArgsForTest(sourcePath, outDir, helpers.HLSProfile1080p8Mbps, 0, 0, helpers.HARDWARE_ACCELERATION_DEVICE_APPLE, false)
    if err != nil {
      t.Fatal(err)
    }
    argStr := strings.Join(args, " ")
    if !strings.Contains(argStr, "videotoolbox") || !strings.Contains(argStr, "h264_videotoolbox") {
      t.Errorf("apple device should use videotoolbox and h264_videotoolbox")
    }
  })

  t.Run("invalid profile returns error", func(t *testing.T) {
    _, err := BuildHLSArgsForTest(sourcePath, outDir, "4k_10mbps", 0, 0, helpers.HARDWARE_ACCELERATION_DEVICE_CPU, false)
    if err == nil {
      t.Error("expected error for invalid profile")
    }
  })

  t.Run("source path is single argument no injection", func(t *testing.T) {
    // Path with spaces/special chars should appear as one argument, not split.
    dangerPath := "/path/with spaces; rm -rf /"
    args, err := BuildHLSArgsForTest(dangerPath, outDir, helpers.HLSProfile1080p4Mbps, 0, 0, helpers.HARDWARE_ACCELERATION_DEVICE_CPU, false)
    if err != nil {
      t.Fatal(err)
    }
    found := false
    for _, a := range args {
      if a == dangerPath {
        found = true
        break
      }
    }
    if !found {
      t.Errorf("source path should appear as single argument; got args: %q", args)
    }
    // No raw "rm" or ";" as separate args
    for _, a := range args {
      if a == "rm" || a == ";" {
        t.Errorf("dangerous token should not appear as separate arg: %q", a)
      }
    }
  })
}

func TestIsAllowedHLSProfile(t *testing.T) {
  for _, profile := range helpers.HLSAllowedProfiles {
    if !helpers.IsAllowedHLSProfile(profile) {
      t.Errorf("IsAllowedHLSProfile(%q) should be true", profile)
    }
  }
  if helpers.IsAllowedHLSProfile("4k_10mbps") {
    t.Error("4k profile should not be allowed in v1")
  }
  if helpers.IsAllowedHLSProfile("") {
    t.Error("empty profile should not be allowed")
  }
}
