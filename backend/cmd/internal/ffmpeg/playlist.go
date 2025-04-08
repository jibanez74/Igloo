package ffmpeg

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

func (f *ffmpeg) monitorAndUpdatePlaylists(ctx context.Context, outputDir string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer watcher.Close()

	eventPath := filepath.Join(outputDir, DefaultPlaylistName)
	vodPath := filepath.Join(outputDir, VodPlaylistName)

	// Start watching the output directory
	err = watcher.Add(outputDir)
	if err != nil {
		return fmt.Errorf("failed to watch directory: %w", err)
	}

	// Wait for the event playlist to be created
	playlistCreated := make(chan struct{})
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Name == eventPath && (event.Op&fsnotify.Create == fsnotify.Create) {
					close(playlistCreated)
					return
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				fmt.Printf("watcher error: %v", err)
			}
		}
	}()

	// Wait for playlist creation or context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-playlistCreated:
		// Create initial VOD playlist
		if err := f.createInitialVodPlaylist(eventPath, vodPath); err != nil {
			return fmt.Errorf("failed to create initial VOD playlist: %w", err)
		}
	}

	// Continue monitoring for updates
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case event, ok := <-watcher.Events:
			if !ok {
				return fmt.Errorf("watcher channel closed")
			}

			if event.Name != eventPath {
				continue
			}

			if event.Op&fsnotify.Write == fsnotify.Write {
				if err := f.updateVodPlaylist(eventPath, vodPath); err != nil {
					return fmt.Errorf("failed to update VOD playlist: %w", err)
				}
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return fmt.Errorf("watcher error channel closed")
			}
			return fmt.Errorf("watcher error: %w", err)
		}
	}
}

func (f *ffmpeg) createInitialVodPlaylist(eventPath, vodPath string) error {
	for i := 0; i < 10; i++ {
		_, err := os.Stat(eventPath)
		if err == nil {
			break
		}

		time.Sleep(500 * time.Millisecond)
	}

	content, err := os.ReadFile(eventPath)
	if err != nil {
		return fmt.Errorf("failed to read event playlist: %w", err)
	}

	vodContent := strings.Replace(
		string(content),
		"#EXT-X-PLAYLIST-TYPE:EVENT",
		"#EXT-X-PLAYLIST-TYPE:VOD",
		1,
	)

	return os.WriteFile(vodPath, []byte(vodContent), 0644)
}

func (f *ffmpeg) updateVodPlaylist(eventPath, vodPath string) error {
	eventContent, err := os.ReadFile(eventPath)
	if err != nil {
		return fmt.Errorf("failed to read event playlist: %w", err)
	}

	vodContent, err := os.ReadFile(vodPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read VOD playlist: %w", err)
	}

	eventLines := strings.Split(string(eventContent), "\n")
	vodLines := strings.Split(string(vodContent), "\n")

	lastSegment := ""

	for i := len(vodLines) - 1; i >= 0; i-- {
		if strings.HasPrefix(vodLines[i], "#EXTINF") {
			lastSegment = vodLines[i+1]
			break
		}
	}

	var newSegments []string

	foundLastSegment := lastSegment == ""

	for _, line := range eventLines {
		if foundLastSegment {
			if strings.HasPrefix(line, "#EXTINF") || strings.HasPrefix(line, "#EXT-X-") {
				newSegments = append(newSegments, line)
			}
		} else if line == lastSegment {
			foundLastSegment = true
		}
	}

	if len(newSegments) > 0 {
		vodContentStr := strings.TrimSpace(string(vodContent))
		if vodContentStr != "" {
			vodContentStr += "\n"
		}
		vodContentStr += strings.Join(newSegments, "\n")

		return os.WriteFile(vodPath, []byte(vodContentStr), 0644)
	}

	return nil
}

func (f *ffmpeg) finalizeVODPlaylist(vodPath string) error {
	content, err := os.ReadFile(vodPath)
	if err != nil {
		return fmt.Errorf("failed to read VOD playlist: %w", err)
	}

	if strings.Contains(string(content), "#EXT-X-ENDLIST") {
		return nil
	}

	vodContent := string(content)
	if !strings.HasSuffix(vodContent, "\n") {
		vodContent += "\n"
	}

	vodContent += "#EXT-X-ENDLIST\n"

	return os.WriteFile(vodPath, []byte(vodContent), 0644)
}
