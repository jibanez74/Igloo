package music

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/itunes"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/scannertest"
	spotifyapi "igloo/cmd/internal/spotify"
)

func TestResolveTrackFileMapsAudioMetadata(t *testing.T) {
	app := setupMusicScanner(t)

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

	resolved, err := app.resolveTrackFile(context.Background(), newMusicScanContext(nil), "", scanner.ScanFile{
		Path: trackPath,
		Ext:  "flac",
		Size: 42,
	}, itunes.CatalogIDs{})
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

	resolved, err := app.resolveTrackFile(context.Background(), newMusicScanContext(nil), "", scanner.ScanFile{
		Path: trackPath,
		Ext:  "mp3",
		Size: 7,
	}, itunes.CatalogIDs{})
	if err != nil {
		t.Fatalf("resolve track file: %v", err)
	}

	params := resolved.params
	if params.Title != "No Tags" || params.SortTitle != "No Tags" {
		t.Fatalf("title/sort_title = %q/%q, want the file stem", params.Title, params.SortTitle)
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
	if !app.logger.(*scannertest.Logger).DebugMentions("unreadable audio tag", "not-a-duration") {
		t.Fatalf("unparsable duration was not logged: %+v", app.logger.(*scannertest.Logger).DebugEntries)
	}
}

func TestAudioStreamRequiredBeforeResolution(t *testing.T) {
	musicDir := t.TempDir()
	for _, streams := range [][]ffprobe.Stream{nil, {{CodecType: "video", CodecName: "mjpeg"}}, {{CodecType: "subtitle"}}} {
		t.Run(fmt.Sprint(streams), func(t *testing.T) {
			s := setupMusicScanner(t)
			metadata := testMusicMetadata()
			metadata.Streams = streams
			s.ffprobe = &scannertest.CountingProbe{Default: metadata}
			spotify := &musicScannerSpotifyStub{}
			s.spotify = spotify
			scanned, _, failures := s.processMusicFixtureBatch(t, context.Background(), newMusicScanContext(nil), []scanner.ScanFile{{Path: musicDir + "/no-audio.m4a", Ext: "m4a", Size: 1}})
			if scanned != 0 || failures != 1 || spotify.artistCalls != 0 || spotify.albumCalls != 0 {
				t.Fatalf("scanned=%d errors=%d Spotify=%+v", scanned, failures, spotify)
			}
			for _, table := range []string{"tracks", "musicians", "albums", "music_provider_matches"} {
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
	metadata := testMusicMetadata()
	metadata.Streams = []ffprobe.Stream{{CodecType: "video", CodecName: "mjpeg"}, {CodecType: "audio", CodecName: "aac", Channels: 2}, {CodecType: "audio", CodecName: "mp3", Channels: 1}}
	s.ffprobe = &scannertest.CountingProbe{Default: metadata}
	resolved, err := s.resolveTrackFile(context.Background(), newMusicScanContext(nil), "", scanner.ScanFile{Path: musicDir + "/art.m4a", Ext: "m4a", Size: 1}, itunes.CatalogIDs{})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.params.Codec != "aac" || resolved.params.Channels != "2" {
		t.Fatalf("audio selection: %+v", resolved.params)
	}
	prepareMusicFixtures(t, []scanner.ScanFile{{Path: resolved.params.FilePath, Size: resolved.params.Size}})
	resolved.inspection, err = scanner.InspectFile(context.Background(), resolved.params.FilePath, nil, s.now, scanner.FileQuietPeriod)
	if err != nil {
		t.Fatal(err)
	}
	defer resolved.inspection.Close()
	_, err = s.persistResolvedTrack(context.Background(), newMusicScanContext(nil), resolved)
	if err != nil {
		t.Fatal(err)
	}
}

func TestTrackLanguageImportAndChange(t *testing.T) {
	cases := []struct{ stream, format, want string }{
		{"eng", "spa", "eng"}, {"", "spa", "spa"}, {"UnD", "spa", "spa"}, {"zxx", "spa", "zxx"}, {"und", "zxx", "zxx"}, {"", "UND", ""}, {"und", "", ""}, {"und", "und", ""},
	}
	for _, tc := range cases {
		t.Run(tc.stream+"/"+tc.format, func(t *testing.T) {
			s := setupMusicScanner(t)
			path := filepath.Join(t.TempDir(), "track.m4a")
			scan := newMusicScanContext(nil)
			var id int64
			for pass := 0; pass < 2; pass++ {
				metadata := testMusicMetadataWithTags(ffprobe.FormatTags{Title: "Track", Language: tc.format})
				metadata.Streams[0].Tags.Language = tc.stream
				metadata.Streams = append([]ffprobe.Stream{{CodecType: "video", Tags: ffprobe.StreamTags{Language: "ita"}}}, metadata.Streams...)
				metadata.Streams = append(metadata.Streams, ffprobe.Stream{CodecType: "audio", Tags: ffprobe.StreamTags{Language: "deu"}})
				s.ffprobe = &scannertest.CountingProbe{Default: metadata}
				n, _, failures := s.processMusicFixtureBatch(t, context.Background(), scan, []scanner.ScanFile{{Path: path, Ext: "m4a", Size: int64(pass + 1)}})
				if n != 1 || failures != 0 {
					t.Fatalf("scan %d: %d/%d", pass, n, failures)
				}
				var got sql.NullString
				var currentID int64
				err := s.tx.DB.QueryRow("SELECT id,language FROM tracks WHERE file_path=?", path).Scan(&currentID, &got)
				if err != nil || got.String != tc.want || got.Valid != (tc.want != "") {
					t.Fatalf("language %+v want %q: %v", got, tc.want, err)
				}
				if pass == 1 && currentID != id {
					t.Fatal("changed track ID")
				}
				id = currentID
				// Ensure changed-file processing replaces an existing explicit language.
				_, err = s.tx.DB.Exec("UPDATE tracks SET language='fra' WHERE id=?", id)
				if err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestSpotifyRecoveryCreditSelection(t *testing.T) {
	cases := []struct {
		name    string
		credits []spotifyapi.SimpleArtist
		target  string
		matched bool
		pending string
	}{
		{name: "first", credits: []spotifyapi.SimpleArtist{{ID: "artist", Name: "Test Artist"}}, target: "artist"},
		{name: "second", credits: []spotifyapi.SimpleArtist{{ID: "other", Name: "Other"}, {ID: "artist", Name: "Test Artist"}}, target: "artist"},
		{name: "later skips empty IDs", credits: []spotifyapi.SimpleArtist{{Name: "Test Artist"}, {ID: "other", Name: "Other"}, {ID: "artist", Name: "Test Artist"}, {ID: "last", Name: "Test Artist"}}, target: "artist"},
		{name: "unrelated", credits: []spotifyapi.SimpleArtist{{ID: "other", Name: "Other"}}},
		{name: "empty ID", credits: []spotifyapi.SimpleArtist{{Name: "Test Artist"}}},
		{name: "various", credits: []spotifyapi.SimpleArtist{{ID: "various", Name: "Various Artists"}}},
		{name: "already matched", credits: []spotifyapi.SimpleArtist{{ID: "artist", Name: "Test Artist"}}, matched: true},
		{name: "pending wins", credits: []spotifyapi.SimpleArtist{{ID: "replacement", Name: "Test Artist"}}, pending: "artist", target: "artist"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := setupMusicScanner(t)
			scanTaggedTrack(t, s, newMusicScanContext(nil), filepath.Join(t.TempDir(), "track.m4a"), 1, albumTags("Song", "1"))
			if tc.matched {
				_, err := s.tx.DB.Exec("UPDATE musicians SET spotify_id='known'")
				if err != nil {
					t.Fatal(err)
				}
			}
			if tc.pending != "" {
				_, err := s.tx.DB.Exec("INSERT INTO music_spotify_adoptions SELECT id,? FROM musicians", tc.pending)
				if err != nil {
					t.Fatal(err)
				}
			}
			var steps []spotifyStep
			if tc.target != "" {
				steps = append(steps, spotifyStep{operation: "artist", target: tc.target, artist: testArtist()})
			}
			s.spotify = scriptedSpotify(t, steps...)
			album := testAlbum()
			album.Artists = tc.credits
			adopted, err := s.adoptAlbumArtist(context.Background(), newMusicScanContext(nil), "Test Artist", album, false)
			if err != nil {
				t.Fatal(err)
			}
			if (adopted != nil) != (tc.target != "") {
				t.Fatalf("adoption=%+v", adopted)
			}
		})
	}
}

// Opus and Ogg Vorbis keep their tags on the audio stream, and Vorbis
// comments keep the track and disc totals in keys of their own.
func TestResolveTrackFileReadsAudioStreamTagsAndSeparateTotals(t *testing.T) {
	app := setupMusicScanner(t)
	trackPath := filepath.Join(t.TempDir(), "02 Hit.opus")
	app.ffprobe = newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{
		trackPath: {
			Format: ffprobe.Format{Duration: "3.0"},
			Streams: []ffprobe.Stream{{
				CodecName: "opus", CodecType: "audio", Channels: 2, ChannelLayout: "stereo",
				Tags: ffprobe.StreamTags{Music: ffprobe.FormatTags{
					Title: "Hit", Artist: "Singer Alpha", AlbumArtist: "Various Artists", Album: "Summer Hits 2010",
					Track: "2", TrackTotal: "3", Disc: "1", DiscTotal: "1", Compilation: "1",
				}},
			}},
		},
	})

	resolved, err := app.resolveTrackFile(context.Background(), newMusicScanContext(nil), "", scanner.ScanFile{Path: trackPath, Ext: "opus", Size: 1}, itunes.CatalogIDs{})
	if err != nil {
		t.Fatalf("resolve track file: %v", err)
	}

	params := resolved.params
	if params.Title != "Hit" || params.TrackIndex != 2 || params.Disc != 1 {
		t.Fatalf("title, track, disc = %q, %d, %d, want Hit, 2, 1", params.Title, params.TrackIndex, params.Disc)
	}
	if params.Container != "opus" || params.MimeType != "audio/ogg" {
		t.Fatalf("container, mime = %q, %q, want opus, audio/ogg", params.Container, params.MimeType)
	}
	if resolved.album == nil || resolved.album.title != "Summer Hits 2010" || resolved.album.albumArtist != "Various Artists" {
		t.Fatalf("album = %+v, want Summer Hits 2010 by Various Artists", resolved.album)
	}
	if resolved.tags.trackTotal != 3 || resolved.tags.discTotal != 1 || !resolved.tags.compilation {
		t.Fatalf("tags = %+v, want totals 3 and 1 and the compilation flag", resolved.tags)
	}
}

func TestResolveTrackFileTakesCreditsFromAMultiValueArtistsTag(t *testing.T) {
	app := setupMusicScanner(t)
	trackPath := filepath.Join(t.TempDir(), "Wait For It.flac")
	app.ffprobe = newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{
		trackPath: testMusicMetadataWithTags(ffprobe.FormatTags{
			Title: "Wait For It", Album: "Hamilton",
			// ARTIST alone would split into a bogus "Jr." credit.
			Artist:  "Leslie Odom, Jr., Lin-Manuel Miranda & Ensemble",
			Artists: "Leslie Odom, Jr.;Lin-Manuel Miranda;Ensemble",
		}),
	})

	resolved, err := app.resolveTrackFile(context.Background(), newMusicScanContext(nil), "", scanner.ScanFile{Path: trackPath, Ext: "flac", Size: 1}, itunes.CatalogIDs{})
	if err != nil {
		t.Fatalf("resolve track file: %v", err)
	}

	names := make([]string, 0, len(resolved.musicians))
	for _, musician := range resolved.musicians {
		names = append(names, musician.name)
	}
	want := []string{"Leslie Odom, Jr.", "Lin-Manuel Miranda", "Ensemble"}
	if strings.Join(names, "|") != strings.Join(want, "|") {
		t.Fatalf("credits = %q, want %q", names, want)
	}
	if resolved.album == nil || resolved.album.albumArtist != "Leslie Odom, Jr., Lin-Manuel Miranda & Ensemble" {
		t.Fatalf("album = %+v, want the ARTIST string as the fallback album artist", resolved.album)
	}
}

// Only Opus and Ogg read tags off the audio stream: an MP4 track's own name
// is not a song title.
func TestResolveTrackFileReadsStreamTagsOnlyForOggFamilyFiles(t *testing.T) {
	app := setupMusicScanner(t)
	trackPath := filepath.Join(t.TempDir(), "07 Untitled.m4a")
	app.ffprobe = newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{
		trackPath: {
			Format: ffprobe.Format{Duration: "3.0", Tags: ffprobe.FormatTags{Disc: "1/-2", DiscTotal: "2", Track: "7/-1"}},
			Streams: []ffprobe.Stream{{
				CodecName: "aac", CodecType: "audio", Channels: 2,
				Tags: ffprobe.StreamTags{Title: "Stereo", Music: ffprobe.FormatTags{Title: "Stereo"}},
			}},
		},
	})

	resolved, err := app.resolveTrackFile(context.Background(), newMusicScanContext(nil), "", scanner.ScanFile{Path: trackPath, Ext: "m4a", Size: 1}, itunes.CatalogIDs{})
	if err != nil {
		t.Fatalf("resolve track file: %v", err)
	}
	if resolved.params.Title == "Stereo" {
		t.Fatalf("title = %q, want the file name, not the stream's own name", resolved.params.Title)
	}
	// A negative total after the slash is no total, so the separate key counts.
	if resolved.tags.discTotal != 2 || resolved.tags.trackTotal != 0 {
		t.Fatalf("disc total, track total = %d, %d, want 2, 0", resolved.tags.discTotal, resolved.tags.trackTotal)
	}
}
