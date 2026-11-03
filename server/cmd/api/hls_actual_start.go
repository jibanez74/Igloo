package main

import (
	"context"
	"time"

	"igloo/cmd/internal/ffmpeg"
)

// hlsActualStartTimeout bounds the seek measurement behind a copy-video
// session's actual start. The measurement is advisory: running out of time
// costs the session nothing but an unknown start.
const hlsActualStartTimeout = 15 * time.Second

// hlsActualStartWait is how long a manifest response waits for a session's
// start measurement before answering without X-Igloo-Actual-Start. The
// measurement copies one packet, while a copy-video manifest has already
// waited for a whole segment, so the wait is normally nothing. A variable so
// tests can shorten it.
var hlsActualStartWait = 5 * time.Second

// measureHLSActualStart records where a copy-video session's media really
// begins: the source time of the first packet FFmpeg's input seek copies
// (see ffmpeg.SeekLanding). The session's own context bounds it, so a stop,
// seek or shutdown ends the measurement with the session. A failure leaves
// the start unknown and the client falls back to the requested start.
func (app *Application) measureHLSActualStart(ctx context.Context, session *HLSSession, sourcePath string, videoStreamIndex int) {
	defer close(session.actualStartReady)

	measureCtx, cancel := context.WithTimeout(ctx, hlsActualStartTimeout)
	defer cancel()

	landing, err := app.FFmpeg.SeekLanding(measureCtx, ffmpeg.SeekLandingParams{
		SourcePath:       sourcePath,
		VideoStreamIndex: videoStreamIndex,
		StartSec:         session.StartSec,
	})
	if err != nil {
		sessionEnded := ctx.Err() != nil
		if !sessionEnded {
			app.Logger.Warn("hls actual start measurement failed",
				"media", session.Media.String(),
				"requested_start_sec", session.StartSec,
				"error", err.Error(),
			)
		}
		return
	}

	// An MP4 seek picks keyframes by decode time and can land a few frames
	// after the start. The header promises a start at or before the requested
	// one, and the web player discards anything later, so report the start.
	actualStart := min(landing, session.StartSec)
	session.setActualStartSec(actualStart)
	app.Logger.Debug("hls actual start measured",
		"media", session.Media.String(),
		"requested_start_sec", session.StartSec,
		"landing_sec", landing,
	)
}
