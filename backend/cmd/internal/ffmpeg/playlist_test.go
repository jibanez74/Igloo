package ffmpeg

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPlaylistMonitoring(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "playlist_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test event playlist content
	eventContent := `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-TARGETDURATION:4
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-PLAYLIST-TYPE:EVENT
#EXTINF:4.000000,
segment0.ts
#EXTINF:4.000000,
segment1.ts
`

	// Create test event playlist file
	eventPath := filepath.Join(tempDir, DefaultPlaylistName)
	if err := os.WriteFile(eventPath, []byte(eventContent), 0644); err != nil {
		t.Fatalf("Failed to create event playlist: %v", err)
	}

	// Create ffmpeg instance
	f := &ffmpeg{}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start monitoring in a goroutine
	errChan := make(chan error)
	go func() {
		errChan <- f.monitorAndUpdatePlaylists(ctx, tempDir)
	}()

	// Wait a bit for the initial VOD playlist to be created
	time.Sleep(100 * time.Millisecond)

	// Verify initial VOD playlist was created
	vodPath := filepath.Join(tempDir, VodPlaylistName)
	if _, err := os.Stat(vodPath); err != nil {
		t.Fatalf("VOD playlist was not created: %v", err)
	}

	// Read and verify VOD playlist content
	vodContent, err := os.ReadFile(vodPath)
	if err != nil {
		t.Fatalf("Failed to read VOD playlist: %v", err)
	}

	expectedContent := `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-TARGETDURATION:4
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-PLAYLIST-TYPE:VOD
#EXTINF:4.000000,
segment0.ts
#EXTINF:4.000000,
segment1.ts
`

	if string(vodContent) != expectedContent {
		t.Errorf("VOD playlist content mismatch:\nGot:\n%s\nExpected:\n%s", vodContent, expectedContent)
	}

	// Update event playlist with new segment
	updatedEventContent := eventContent + `#EXTINF:4.000000,
segment2.ts
`
	if err := os.WriteFile(eventPath, []byte(updatedEventContent), 0644); err != nil {
		t.Fatalf("Failed to update event playlist: %v", err)
	}

	// Wait for the update to be processed
	time.Sleep(100 * time.Millisecond)

	// Verify VOD playlist was updated
	vodContent, err = os.ReadFile(vodPath)
	if err != nil {
		t.Fatalf("Failed to read updated VOD playlist: %v", err)
	}

	expectedUpdatedContent := `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-TARGETDURATION:4
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-PLAYLIST-TYPE:VOD
#EXTINF:4.000000,
segment0.ts
#EXTINF:4.000000,
segment1.ts
#EXTINF:4.000000,
segment2.ts
`

	if string(vodContent) != expectedUpdatedContent {
		t.Errorf("Updated VOD playlist content mismatch:\nGot:\n%s\nExpected:\n%s", vodContent, expectedUpdatedContent)
	}

	// Cancel context and wait for monitoring to stop
	cancel()
	select {
	case err := <-errChan:
		if err != context.Canceled {
			t.Errorf("Unexpected error from monitoring: %v", err)
		}
	case <-time.After(time.Second):
		t.Error("Monitoring did not stop after context cancellation")
	}
}

func TestCreateInitialVodPlaylist(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "playlist_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test event playlist content
	eventContent := `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-TARGETDURATION:4
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-PLAYLIST-TYPE:EVENT
#EXTINF:4.000000,
segment0.ts
`

	// Create test event playlist file
	eventPath := filepath.Join(tempDir, DefaultPlaylistName)
	if err := os.WriteFile(eventPath, []byte(eventContent), 0644); err != nil {
		t.Fatalf("Failed to create event playlist: %v", err)
	}

	// Create ffmpeg instance
	f := &ffmpeg{}

	// Test creating initial VOD playlist
	vodPath := filepath.Join(tempDir, VodPlaylistName)
	if err := f.createInitialVodPlaylist(eventPath, vodPath); err != nil {
		t.Fatalf("Failed to create initial VOD playlist: %v", err)
	}

	// Verify VOD playlist was created
	vodContent, err := os.ReadFile(vodPath)
	if err != nil {
		t.Fatalf("Failed to read VOD playlist: %v", err)
	}

	expectedContent := `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-TARGETDURATION:4
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-PLAYLIST-TYPE:VOD
#EXTINF:4.000000,
segment0.ts
`

	if string(vodContent) != expectedContent {
		t.Errorf("VOD playlist content mismatch:\nGot:\n%s\nExpected:\n%s", vodContent, expectedContent)
	}
}

func TestUpdateVodPlaylist(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "playlist_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test event playlist content
	eventContent := `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-TARGETDURATION:4
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-PLAYLIST-TYPE:EVENT
#EXTINF:4.000000,
segment0.ts
#EXTINF:4.000000,
segment1.ts
`

	// Create test event playlist file
	eventPath := filepath.Join(tempDir, DefaultPlaylistName)
	if err := os.WriteFile(eventPath, []byte(eventContent), 0644); err != nil {
		t.Fatalf("Failed to create event playlist: %v", err)
	}

	// Create ffmpeg instance
	f := &ffmpeg{}

	// Test updating VOD playlist
	vodPath := filepath.Join(tempDir, VodPlaylistName)
	if err := f.updateVodPlaylist(eventPath, vodPath); err != nil {
		t.Fatalf("Failed to update VOD playlist: %v", err)
	}

	// Verify VOD playlist was updated
	vodContent, err := os.ReadFile(vodPath)
	if err != nil {
		t.Fatalf("Failed to read VOD playlist: %v", err)
	}

	expectedContent := `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-TARGETDURATION:4
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-PLAYLIST-TYPE:VOD
#EXTINF:4.000000,
segment0.ts
#EXTINF:4.000000,
segment1.ts
`

	if string(vodContent) != expectedContent {
		t.Errorf("VOD playlist content mismatch:\nGot:\n%s\nExpected:\n%s", vodContent, expectedContent)
	}
}
