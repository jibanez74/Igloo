package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"time"

	"igloo/cmd/internal/ffmpeg"
	"igloo/cmd/internal/helpers"
)

const (
	subtitleStreamContentType = "video/x-matroska"
	// subtitleStreamMaxOpen bounds the subtitle streams open at once across
	// the server; each holds an FFmpeg process while its player plays.
	subtitleStreamMaxOpen    = 8
	subtitleStreamRetryAfter = "2"
)

// Variables so tests can shorten them.
var (
	// subtitleStreamKeepalive is how long a stream may stay silent between
	// elements before a Void element keeps the connection alive. It is also
	// how long a client that cancelled its load can hold the stream: Media3
	// cancels by setting a flag, and its loader only notices, and closes the
	// connection, once the next bytes arrive. So it stays short; each Void is
	// 3 bytes.
	subtitleStreamKeepalive = 3 * time.Second
	// subtitleStreamSlotWait covers a TV starting at a position, which drops
	// one stream and opens the next at the same moment.
	subtitleStreamSlotWait = 3 * time.Second
)

// SubtitleStream streams one of a movie's subtitle tracks as Matroska.
func (app *Application) SubtitleStream(w http.ResponseWriter, r *http.Request) {
	app.serveSubtitleStream(w, r, mediaKindMovie)
}

// EpisodeSubtitleStream streams one of a TV episode's subtitle tracks as
// Matroska.
func (app *Application) EpisodeSubtitleStream(w http.ResponseWriter, r *http.Request) {
	app.serveSubtitleStream(w, r, mediaKindEpisode)
}

// serveSubtitleStream streams the original track, copied or, for text codecs
// Matroska cannot carry, converted to SubRip, with its packets rebased onto
// the HLS session that starts at `start`. Every error is answered as JSON
// before the first byte of the stream; a failure after it cuts the
// connection, so the player sees a broken stream rather than one that ended.
func (app *Application) serveSubtitleStream(w http.ResponseWriter, r *http.Request, kind mediaKind) {
	track, ok := app.resolveSubtitleTrackRequest(w, r, kind)
	if !ok {
		return
	}
	sub := track.Subtitle

	plan := helpers.SubtitleMatroskaPlanFor(sub.Codec, track.Source.Container)
	if plan == helpers.SubtitleMatroskaUnsupported {
		helpers.ErrorJSON(w, fmt.Errorf("%s subtitles cannot be streamed", sub.Codec), http.StatusUnsupportedMediaType)
		return
	}
	ref := subtitleTrackRef{
		Kind:            kind,
		FileID:          track.Source.FileID,
		StreamIndex:     sub.StreamIndex,
		SourcePath:      track.Source.FilePath,
		ConvertToSubRip: plan == helpers.SubtitleMatroskaSubRip,
	}

	if app.hlsShuttingDown() {
		writeSubtitleStreamUnavailable(w, errHLSShuttingDown)
		return
	}
	acquired := app.acquireSubtitleStreamSlot(r.Context())
	if !acquired {
		clientGone := r.Context().Err() != nil
		if clientGone {
			return
		}
		writeSubtitleStreamUnavailable(w, errors.New("too many subtitle streams are open"))
		return
	}
	defer func() { <-app.SubtitleStreamSlots }()

	// Shutdown ends the stream like a client leaving: FFmpeg goes, and the
	// response ends without an error the player would act on.
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	stopOnShutdown := context.AfterFunc(app.HLSShutdown, cancel)
	defer stopOnShutdown()

	opened, err := app.openSubtitleStream(ctx, ref, track.StartSec)
	if err != nil {
		clientGone := r.Context().Err() != nil
		if clientGone {
			return
		}
		if app.hlsShuttingDown() {
			writeSubtitleStreamUnavailable(w, errHLSShuttingDown)
			return
		}
		app.serverError(w, err, "subtitle stream failed to start", "failed to stream subtitle track",
			"media", track.Media.String(), "stream_index", sub.StreamIndex, "codec", sub.Codec)
		return
	}
	defer opened.cancel()
	fromCache := opened.cachePath != ""
	if !fromCache {
		app.scheduleSubtitleTrackCache(ctx, ref)
	}

	w.Header().Set("Content-Type", subtitleStreamContentType)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	controller := http.NewResponseController(w)
	relayErr := writeSubtitleStreamHeader(w, controller.Flush, opened.header)
	if relayErr == nil {
		relayErr = helpers.RelayMatroskaElements(opened.src, w, controller.Flush, subtitleStreamKeepalive)
	}
	// FFmpeg closes its output a moment before it exits: after a complete
	// stream it is left to exit on its own, since stopping it in that moment
	// would turn a good run into a failed one.
	if relayErr != nil {
		opened.cancel()
	}
	waitErr := opened.stream.Wait()
	// Checked once FFmpeg is gone: a player leaving in that last moment kills
	// it, and the run it cut short is no failure of the stream.
	endedByCaller := r.Context().Err() != nil || app.hlsShuttingDown()
	if endedByCaller {
		return
	}

	failure := relayErr
	if failure == nil {
		failure = waitErr
	}
	if failure != nil {
		app.Logger.Error("subtitle stream failed",
			"media", track.Media.String(),
			"stream_index", sub.StreamIndex,
			"codec", sub.Codec,
			"from_cache", fromCache,
			"error", failure,
			"ffmpeg_error", waitErr,
		)
		// A copy that breaks off would break every later stream the same
		// way, and serving it marks it used, so the sweep would keep it.
		// A player that stopped reading says nothing about the copy.
		readFailed := !errors.Is(relayErr, helpers.ErrMatroskaRelayWrite)
		if fromCache && readFailed {
			removeErr := os.Remove(opened.cachePath)
			if removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
				app.Logger.Warn("could not remove a broken cached subtitle track", "path", opened.cachePath, "error", removeErr)
			}
		}
		panic(http.ErrAbortHandler)
	}
}

// writeSubtitleStreamHeader sends the primed header. A failure is on the
// player's side, as one writing the elements that follow is.
func writeSubtitleStreamHeader(w io.Writer, flush func() error, header []byte) error {
	_, err := w.Write(header)
	if err == nil {
		err = flush()
	}
	if err != nil {
		return fmt.Errorf("%w: %w", helpers.ErrMatroskaRelayWrite, err)
	}
	return nil
}

// writeSubtitleStreamUnavailable answers a retryable refusal: every slot is
// taken, or the server is shutting down.
func writeSubtitleStreamUnavailable(w http.ResponseWriter, err error) {
	w.Header().Set("Retry-After", subtitleStreamRetryAfter)
	helpers.ErrorJSON(w, err, http.StatusServiceUnavailable)
}

func (app *Application) acquireSubtitleStreamSlot(ctx context.Context) bool {
	select {
	case app.SubtitleStreamSlots <- struct{}{}:
		return true
	default:
	}

	timer := time.NewTimer(subtitleStreamSlotWait)
	defer timer.Stop()
	select {
	case app.SubtitleStreamSlots <- struct{}{}:
		return true
	case <-timer.C:
		return false
	case <-ctx.Done():
		return false
	}
}

// openedSubtitleStream is a running FFmpeg whose header has been read and
// primed; src holds the rest of its output.
type openedSubtitleStream struct {
	stream ffmpeg.SubtitleStream
	src    *bufio.Reader
	header []byte
	// cachePath is the cached copy FFmpeg reads, or "" for the media file.
	cachePath string
	// cancel ends this FFmpeg alone.
	cancel context.CancelFunc
}

// openSubtitleStream streams the track's cached copy when there is one, and
// the media file otherwise, or when the copy cannot be read.
func (app *Application) openSubtitleStream(ctx context.Context, ref subtitleTrackRef, startSec float64) (openedSubtitleStream, error) {
	cachePath := app.cachedSubtitleTrack(ref)
	if cachePath != "" {
		opened, err := app.startSubtitleStream(ctx, ffmpeg.SubtitleStreamParams{
			InputPath: cachePath,
			FromCache: true,
			StartSec:  startSec,
		})
		if err == nil {
			opened.cachePath = cachePath
			return opened, nil
		}
		if ctx.Err() != nil {
			return openedSubtitleStream{}, err
		}
		app.Logger.Warn("cached subtitle track could not be streamed; reading the media file",
			"track", ref.String(), "error", err)
		_ = os.Remove(cachePath)
	}

	return app.startSubtitleStream(ctx, ffmpeg.SubtitleStreamParams{
		InputPath:       ref.SourcePath,
		StreamIndex:     ref.StreamIndex,
		ConvertToSubRip: ref.ConvertToSubRip,
		StartSec:        startSec,
	})
}

// startSubtitleStream starts FFmpeg and reads its header through Tracks,
// which FFmpeg writes before it reads any packet. A process that ends first,
// or writes something the primer cannot follow, is reported with its stderr.
func (app *Application) startSubtitleStream(ctx context.Context, params ffmpeg.SubtitleStreamParams) (openedSubtitleStream, error) {
	processCtx, cancel := context.WithCancel(ctx)
	stream, err := app.FFmpeg.StreamSubtitleMatroska(processCtx, params)
	if err != nil {
		cancel()
		return openedSubtitleStream{}, err
	}

	src := bufio.NewReader(stream)
	header, err := helpers.ReadMatroskaHeader(src)
	if err != nil {
		cancel()
		waitErr := stream.Wait()
		return openedSubtitleStream{}, fmt.Errorf("read subtitle stream header: %w (ffmpeg: %v)", err, waitErr)
	}

	return openedSubtitleStream{stream: stream, src: src, header: header, cancel: cancel}, nil
}
