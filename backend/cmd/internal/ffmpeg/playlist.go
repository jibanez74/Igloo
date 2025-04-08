package ffmpeg

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/fsnotify/fsnotify"
)

type monitorParams struct {
	vodPath   string
	eventPath string
	ctx       context.Context
}

func (f *ffmpeg) monitorAndUpdatePlaylists(opts *monitorParams) error {
	if opts == nil {
		return errors.New("unable to monitor event playlist because opts is nil")
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("unable to create fsnotify watcher: %w", err)
	}
	defer watcher.Close()

	err = watcher.Add(opts.eventPath)
	if err != nil {
		return fmt.Errorf("unable to add event path to watcher: %w", err)
	}

	vodFile, err := os.Create(opts.vodPath)
	if err != nil {
		return fmt.Errorf("unable to create VOD playlist: %w", err)
	}
	defer vodFile.Close()

	for {
		select {
		case <-opts.ctx.Done():
			f.finalizeVODPlaylist(opts.vodPath)
			return nil

		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
				vodContent, err := f.getEventPlaylistContent(opts.eventPath)
				if err != nil {
					return fmt.Errorf("unable to get event playlist content: %w", err)
				}

				_, err = vodFile.Write(vodContent)
				if err != nil {
					return fmt.Errorf("unable to write to VOD playlist: %w", err)
				}
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			return fmt.Errorf("watcher error: %w", err)
		}
	}
}

func (f *ffmpeg) getEventPlaylistContent(eventPath string) ([]byte, error) {
	content, err := os.ReadFile(eventPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read event playlist: %w", err)
	}

	lines := bytes.Split(content, []byte("\n"))
	var updatedLines [][]byte

	for _, line := range lines {
		if bytes.Contains(line, []byte("#EXT-X-PLAYLIST-TYPE:EVENT")) {
			updatedLines = append(updatedLines, []byte("#EXT-X-PLAYLIST-TYPE:VOD"))
			continue
		}

		updatedLines = append(updatedLines, line)
	}

	vodContent := bytes.Join(updatedLines, []byte("\n"))

	return vodContent, nil
}

func (f *ffmpeg) finalizeVODPlaylist(vodPath string) error {
	content, err := os.ReadFile(vodPath)
	if err != nil {
		return fmt.Errorf("unable to read VOD playlist: %w", err)
	}

	if bytes.Contains(content, []byte("#EXT-X-ENDLIST")) {
		return nil
	}

	if !bytes.HasSuffix(content, []byte("\n")) {
		content = append(content, '\n')
	}
	content = append(content, []byte("#EXT-X-ENDLIST\n")...)

	return os.WriteFile(vodPath, content, 0644)
}
