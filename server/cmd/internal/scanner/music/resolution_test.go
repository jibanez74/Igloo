package music

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/scannertest"
)

func TestResolveTrackFileMapsAudioMetadata(t *testing.T) {
	app := setupMusicScanner(t)
	defer app.tx.DB.Close()

	trackPath := filepath.Join(t.TempDir(), "Mapped Track.flac")
	metadata := &ffprobe.FfprobeResult{
		Format: ffprobe.Format{
			Duration: "245.125",
			BitRate:  "1411200",
			Tags: ffprobe.FormatTags{
				Title:       "Mapped Title",
				Artist:      "Mapped Artist",
				AlbumArtist: "Mapped Album Artist",
				Composer:    "Mapped Composer",
				Album:       "Mapped Album",
				Genre:       "Mapped Genre",
				Track:       "7/12",
				Disc:        "2/3",
				Date:        "2024-02-03",
				Copyright:   "Mapped Copyright",
				SortName:    "Mapped Sort Title",
				SortAlbum:   "Mapped Sort Album",
				SortArtist:  "Mapped Sort Artist",
			},
		},
		Streams: []ffprobe.Stream{
			{
				Index:     0,
				CodecName: "mjpeg",
				CodecType: "video",
				Disposition: ffprobe.StreamDisposition{
					AttachedPic: 1,
				},
			},
			{
				Index:         1,
				CodecName:     "flac",
				CodecType:     "audio",
				Profile:       "Lossless",
				Channels:      6,
				ChannelLayout: "5.1",
				Tags: ffprobe.StreamTags{
					Language: "jpn",
				},
			},
		},
	}
	app.ffprobe = newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{
		trackPath: metadata,
	})

	resolved, err := app.resolveTrackFile(context.Background(), newMusicScanContext(nil), scanner.ScanFile{
		Path: trackPath,
		Ext:  "flac",
		Size: 42,
	})
	if err != nil {
		t.Fatalf("resolve track file: %v", err)
	}

	params := resolved.params
	if params.Title != "Mapped Title" || params.SortTitle != "Mapped Sort Title" {
		t.Fatalf("title/sort_title = %q/%q, want mapped tags", params.Title, params.SortTitle)
	}
	if params.Container != "flac" || params.MimeType != "audio/flac" {
		t.Fatalf("container/mime = %q/%q, want flac/audio/flac", params.Container, params.MimeType)
	}
	if params.Duration != 245125 || params.TrackIndex != 7 || params.Disc != 2 {
		t.Fatalf("duration/track/disc = %d/%d/%d, want 245125/7/2", params.Duration, params.TrackIndex, params.Disc)
	}
	if params.BitRate != 1411200 {
		t.Fatalf("bit rate = %d, want 1411200", params.BitRate)
	}
	if params.Codec != "flac" || params.Profile != "Lossless" || params.Channels != "5.1" || params.ChannelLayout != "5.1" {
		t.Fatalf("audio fields = codec %q profile %q channels %q layout %q, want flac/Lossless/5.1/5.1",
			params.Codec, params.Profile, params.Channels, params.ChannelLayout)
	}
	if !params.Language.Valid || params.Language.String != "jpn" {
		t.Fatalf("language = %#v, want jpn", params.Language)
	}
	if !params.ReleaseDate.Valid || params.ReleaseDate.String != "2024-02-03" {
		t.Fatalf("release date = %#v, want 2024-02-03", params.ReleaseDate)
	}
	if !params.Year.Valid || params.Year.Int64 != 2024 {
		t.Fatalf("year = %#v, want 2024", params.Year)
	}
	if !params.Composer.Valid || params.Composer.String != "Mapped Composer" {
		t.Fatalf("composer = %#v, want mapped composer", params.Composer)
	}
	if !params.Copyright.Valid || params.Copyright.String != "Mapped Copyright" {
		t.Fatalf("copyright = %#v, want mapped copyright", params.Copyright)
	}
	if resolved.genreTag != "Mapped Genre" {
		t.Fatalf("genre tag = %q, want Mapped Genre", resolved.genreTag)
	}
	if len(resolved.musicians) != 1 || resolved.musicians[0].name != "Mapped Artist" || resolved.musicians[0].sortName != "Mapped Sort Artist" {
		t.Fatalf("resolved musicians = %#v, want mapped artist and sort artist", resolved.musicians)
	}
	if resolved.album == nil {
		t.Fatal("expected resolved album")
	}
	if resolved.album.title != "Mapped Album" || resolved.album.sortTitle != "Mapped Sort Album" || resolved.album.albumArtist != "Mapped Album Artist" {
		t.Fatalf("resolved album = %#v, want mapped album tags", resolved.album)
	}
}

func TestResolveTrackFileFallsBackToFilenameAndNumericDefaults(t *testing.T) {
	app := setupMusicScanner(t)
	defer app.tx.DB.Close()

	trackPath := filepath.Join(t.TempDir(), "No Tags.mp3")
	app.ffprobe = newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{
		trackPath: {
			Format: ffprobe.Format{
				Duration: "not-a-duration",
				BitRate:  "not-a-bitrate",
			},
			Streams: []ffprobe.Stream{
				{
					CodecName: "mp3",
					CodecType: "audio",
					Channels:  2,
				},
			},
		},
	})

	resolved, err := app.resolveTrackFile(context.Background(), newMusicScanContext(nil), scanner.ScanFile{
		Path: trackPath,
		Ext:  "mp3",
		Size: 7,
	})
	if err != nil {
		t.Fatalf("resolve track file: %v", err)
	}

	params := resolved.params
	if params.Title != "No Tags.mp3" || params.SortTitle != "No Tags.mp3" {
		t.Fatalf("title/sort_title = %q/%q, want filename fallback", params.Title, params.SortTitle)
	}
	if params.MimeType != "audio/mpeg" {
		t.Fatalf("mime type = %q, want audio/mpeg", params.MimeType)
	}
	if params.Duration != 0 || params.BitRate != 0 {
		t.Fatalf("duration/bitrate = %d/%d, want zero defaults", params.Duration, params.BitRate)
	}
	if params.Channels != "2" || params.ChannelLayout != "2" {
		t.Fatalf("channels/layout = %q/%q, want numeric fallback", params.Channels, params.ChannelLayout)
	}
	if len(resolved.musicians) != 0 {
		t.Fatalf("musicians = %#v, want none without artist tag", resolved.musicians)
	}
	if resolved.album != nil {
		t.Fatalf("album = %#v, want none without album tag", resolved.album)
	}
}

func TestAudioStreamRequiredBeforeResolution(t *testing.T) {
	musicDir := t.TempDir()
	for _, streams := range [][]ffprobe.Stream{nil, {{CodecType: "video", CodecName: "mjpeg"}}, {{CodecType: "subtitle"}}} {
		t.Run(fmt.Sprint(streams), func(t *testing.T) {
			s := setupMusicScanner(t)
			defer s.tx.DB.Close()
			metadata := testMusicMetadata()
			metadata.Streams = streams
			s.ffprobe = &countingMusicScannerFfprobe{result: metadata}
			spotify := &musicScannerSpotifyStub{}
			s.spotify = spotify
			scanned, _, failures := s.processMusicFixtureBatch(t, context.Background(), newMusicScanContext(nil), []scanner.ScanFile{{Path: musicDir + "/no-audio.m4a", Ext: "m4a", Size: 1}})
			if scanned != 0 || failures != 1 || spotify.artistCalls != 0 || spotify.albumCalls != 0 {
				t.Fatalf("scanned=%d errors=%d Spotify=%+v", scanned, failures, spotify)
			}
			for _, table := range []string{"tracks", "musicians", "albums", "music_spotify_matches"} {
				count := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM "+table)
				if count != 0 {
					t.Fatalf("%s has %d rows", table, count)
				}
			}
			logs := s.logger.(*scannertest.Logger)
			if len(logs.WarnEntries) != 1 || !strings.Contains(fmt.Sprint(logs.WarnEntries[0].Args), musicDir+"/no-audio.m4a") || !strings.Contains(fmt.Sprint(logs.WarnEntries[0].Args), "no audio stream") {
				t.Fatalf("failure logs: %+v", logs.WarnEntries)
			}
		})
	}
}

func TestAudioWithArtworkSelectsFirstAudioStream(t *testing.T) {
	musicDir := t.TempDir()
	s := setupMusicScanner(t)
	defer s.tx.DB.Close()
	metadata := testMusicMetadata()
	metadata.Streams = []ffprobe.Stream{{CodecType: "video", CodecName: "mjpeg"}, {CodecType: "audio", CodecName: "aac", Channels: 2}, {CodecType: "audio", CodecName: "mp3", Channels: 1}}
	s.ffprobe = &countingMusicScannerFfprobe{result: metadata}
	resolved, err := s.resolveTrackFile(context.Background(), newMusicScanContext(nil), scanner.ScanFile{Path: musicDir + "/art.m4a", Ext: "m4a", Size: 1})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.params.Codec != "aac" || resolved.params.Channels != "2" {
		t.Fatalf("audio selection: %+v", resolved.params)
	}
	prepareMusicFixtures(t, []scanner.ScanFile{{Path: resolved.params.FilePath, Size: resolved.params.Size}})
	resolved.inspection, err = scanner.InspectFile(context.Background(), resolved.params.FilePath, nil, s.now)
	if err != nil {
		t.Fatal(err)
	}
	defer resolved.inspection.Close()
	_, err = s.persistResolvedTrack(context.Background(), newMusicScanContext(nil), resolved)
	if err != nil {
		t.Fatal(err)
	}
}
