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

	fmt.Printf("Watching event playlist at: %s\n", eventPath)
	fmt.Printf("Updating VOD playlist at: %s\n", vodPath)

	// Start watching the specific event playlist file
	err = watcher.Add(eventPath)
	if err != nil {
		return fmt.Errorf("failed to watch event playlist: %w", err)
	}

	// Check if event playlist already exists
	if _, err := os.Stat(eventPath); err == nil {
		fmt.Printf("Event playlist already exists at %s\n", eventPath)
		// Create initial VOD playlist if event playlist exists
		if err := f.createInitialVodPlaylist(eventPath, vodPath); err != nil {
			return fmt.Errorf("failed to create initial VOD playlist: %w", err)
		}
	} else {
		fmt.Printf("Waiting for event playlist to be created at %s\n", eventPath)
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
					fmt.Printf("Received event: %s %s\n", event.Op, event.Name)
					if event.Name == eventPath && (event.Op&fsnotify.Create == fsnotify.Create) {
						fmt.Printf("Event playlist created at %s\n", eventPath)
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

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-playlistCreated:
			if err := f.createInitialVodPlaylist(eventPath, vodPath); err != nil {
				return fmt.Errorf("failed to create initial VOD playlist: %w", err)
			}
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

			fmt.Printf("Received event for file: %s, operation: %s\n", event.Name, event.Op)
			if event.Name == eventPath && event.Op&fsnotify.Write == fsnotify.Write {
				fmt.Printf("Event playlist updated, updating VOD playlist\n")
				// Read current event playlist content
				eventContent, err := os.ReadFile(eventPath)
				if err != nil {
					return fmt.Errorf("failed to read event playlist: %w", err)
				}
				fmt.Printf("Event playlist content:\n%s\n", string(eventContent))

				// Update VOD playlist
				if err := f.updateVodPlaylist(eventPath, vodPath); err != nil {
					return fmt.Errorf("failed to update VOD playlist: %w", err)
				}

				// Verify VOD playlist was updated
				vodContent, err := os.ReadFile(vodPath)
				if err != nil {
					return fmt.Errorf("failed to read updated VOD playlist: %w", err)
				}
				fmt.Printf("VOD playlist content after update:\n%s\n", string(vodContent))
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
	// Read the event playlist content
	content, err := os.ReadFile(eventPath)
	if err != nil {
		return fmt.Errorf("failed to read event playlist: %w", err)
	}

	// Split the content into lines
	lines := strings.Split(string(content), "\n")
	var updatedLines []string

	// Process each line
	for _, line := range lines {
		// Replace EVENT with VOD in the playlist type
		if strings.Contains(line, "#EXT-X-PLAYLIST-TYPE:EVENT") {
			updatedLines = append(updatedLines, "#EXT-X-PLAYLIST-TYPE:VOD")
			continue
		}

		// Keep all other lines as is, including segment URLs
		updatedLines = append(updatedLines, line)
	}

	// Join the lines back together
	vodContent := strings.Join(updatedLines, "\n")

	// Write the updated content to the VOD playlist
	return os.WriteFile(vodPath, []byte(vodContent), 0644)
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
