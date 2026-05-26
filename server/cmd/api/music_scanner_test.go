package main

import (
	"context"
	"path/filepath"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
)

func testMusicMetadata() *ffprobe.FfprobeResult {
	return &ffprobe.FfprobeResult{
		Format: ffprobe.Format{
			Duration:   "180",
			Size:       "5",
			BitRate:    "256000",
			FormatName: "mov,mp4,m4a,3gp,3g2,mj2",
			Tags: ffprobe.FormatTags{
				Title:  "Test Track",
				Artist: "Test Artist",
				Album:  "Test Album",
				Track:  "1/10",
			},
		},
		Streams: []ffprobe.Stream{
			{
				Index:         0,
				CodecName:     "aac",
				CodecType:     "audio",
				Channels:      2,
				ChannelLayout: "stereo",
			},
		},
	}
}

func TestProcessMusicBatchWritesScanStatusAndSkipsUnchanged(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.Ffprobe = &stubMovieScannerFfprobe{result: testMusicMetadata()}

	file := trackFile{
		path:  filepath.Join(t.TempDir(), "Test Track.m4a"),
		ext:   "m4a",
		size:  5,
		mtime: 123456789,
	}

	scanned, skipped, errCount, processed := app.processMusicBatch(context.Background(), []trackFile{file})
	if scanned != 1 || skipped != 0 || errCount != 0 || len(processed) != 1 {
		t.Fatalf("first scan result scanned=%d skipped=%d errors=%d processed=%d, want 1/0/0/1", scanned, skipped, errCount, len(processed))
	}

	var statusCount int
	err := app.DB.QueryRow("SELECT COUNT(*) FROM track_scan_status WHERE file_path = ? AND size = ? AND file_mtime = ? AND scan_error IS NULL", file.path, file.size, file.mtime).Scan(&statusCount)
	if err != nil {
		t.Fatalf("count track scan status: %v", err)
	}
	if statusCount != 1 {
		t.Fatalf("track scan status count = %d, want 1", statusCount)
	}

	scanned, skipped, errCount, processed = app.processMusicBatch(context.Background(), []trackFile{file})
	if scanned != 0 || skipped != 1 || errCount != 0 || len(processed) != 0 {
		t.Fatalf("second scan result scanned=%d skipped=%d errors=%d processed=%d, want 0/1/0/0", scanned, skipped, errCount, len(processed))
	}
}

func TestRunMusicScanClearsInProgressFlagWhenDirectoryMissing(t *testing.T) {
	finishMusicScan()

	app := setupTestApp(t)
	defer app.DB.Close()
	app.Settings = &database.Setting{}

	if !tryBeginMusicScan() {
		t.Fatal("expected to reserve music scan")
	}

	app.runMusicScan()

	if !tryBeginMusicScan() {
		t.Fatal("music scan flag was not cleared")
	}
	finishMusicScan()
}
