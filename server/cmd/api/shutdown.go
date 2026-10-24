package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"igloo/cmd/internal/ffmpeg"
	"igloo/cmd/internal/ffprobe"
)

// serveUntilShutdown runs the HTTP server and returns once ListenForShutdown
// has finished cleaning up. ListenAndServe returns the moment Shutdown closes
// the listener, before any cleanup has run; returning from main on that alone
// ended the process mid-teardown, with FFmpeg children orphaned, temp
// directories left behind, and the database and logger never closed.
func (app *Application) serveUntilShutdown() error {
	// Subscribe before the listener binds: a signal landing after the port is
	// open but before the goroutine below had registered took the default
	// action and ended the process mid-startup with no cleanup at all.
	quit := subscribeShutdownSignals()
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		app.ListenForShutdown(quit)
	}()

	err := app.Server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	<-shutdownDone
	return nil
}

// subscribeShutdownSignals returns a channel that receives the first SIGINT
// or SIGTERM; ListenForShutdown stops the subscription once it fires.
func subscribeShutdownSignals() chan os.Signal {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	return quit
}

// ListenForShutdown blocks until quit delivers a signal, then stops the HTTP
// server and releases every resource the process owns. It returns rather than
// exiting so the caller decides when the process ends.
func (app *Application) ListenForShutdown(quit chan os.Signal) {
	<-quit

	signal.Stop(quit)

	app.Logger.Info("shutting down server...")

	// Before the HTTP server: see stopHLSSessions.
	app.stopHLSSessions()

	ctx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
	defer cancel()

	err := app.Server.Shutdown(ctx)
	if err != nil {
		// What outlives the grace period is a response that ends only when its
		// client stops reading, such as a direct-play stream, which the
		// process exit would cut anyway.
		app.Logger.Warn("closing connections still open after the shutdown grace period", "error", err)
		closeErr := app.Server.Close()
		if closeErr != nil {
			app.Logger.Error("failed to close server connections", "error", closeErr)
		}
	}

	app.Logger.Info("running clean up tasks...")

	app.WatchRoomHub.Shutdown()
	app.DeviceExpiryCancel()
	app.ScanCancel()

	// Background tasks may still need database and logger access.
	app.Wait.Wait()

	app.SubtitleVTTCache.Flush()
	app.RoomHLSTombstone.Flush()
	app.clearMediaClientCaches()
	app.cleanupMediaBinaries()
	app.closeDatabase()
	app.closeLogger()
}

// shutdownGracePeriod bounds how long in-flight requests get to finish once
// HLS is stopped. API requests take milliseconds; what is still open past it
// is a stream that would otherwise hold the shutdown for its full length.
const shutdownGracePeriod = 10 * time.Second

// stopHLSSessions stops every FFmpeg session, which must happen before the
// FFmpeg binary is cleaned up, and it runs before the HTTP server's graceful
// shutdown: a segment request long-polls for up to two minutes and a manifest
// for up to 45 seconds, Shutdown waits for every one of them, and only a
// stopped session answers them, with the 404 clients rebase on once the server
// is back. From here on no session starts or is published; the cache is
// emptied under both locks the publishers take, so a creation that raced the
// shutdown either is in this snapshot or sees the refusal.
func (app *Application) stopHLSSessions() {
	app.beginHLSShutdown()

	app.PersonalHLSMu.Lock()
	app.RoomHLSMu.Lock()
	var sessions []*HLSSession
	for _, item := range app.HLSSessionCache.Items() {
		session, ok := item.Object.(*HLSSession)
		if ok {
			sessions = append(sessions, session)
		}
	}
	app.HLSSessionCache.Flush()
	app.RoomHLSMu.Unlock()
	app.PersonalHLSMu.Unlock()

	// Each stop can wait seconds for FFmpeg to exit, so they run together.
	var stops sync.WaitGroup
	for _, session := range sessions {
		stops.Go(func() {
			cleanupHLSSession(session)
		})
	}
	stops.Wait()

	if len(sessions) > 0 {
		app.Logger.Info("cleaned up HLS sessions", "count", len(sessions))
	}
}

func (app *Application) clearMediaClientCaches() {
	if app.Tmdb != nil {
		app.Tmdb.ClearCache()
	}
	app.PrerollTheaters.Reset()
}

func (app *Application) cleanupMediaBinaries() {
	err := ffprobe.Cleanup()
	if err != nil {
		app.Logger.Error("failed to cleanup ffprobe", "error", err)
	}

	err = ffmpeg.Cleanup()
	if err != nil {
		app.Logger.Error("failed to cleanup ffmpeg", "error", err)
	}
}

func (app *Application) cleanupStartupResources() {
	if app.Ffprobe != nil || app.FFmpeg != nil {
		app.cleanupMediaBinaries()
	}
	app.closeDatabase()
	app.closeLogger()
}

func (app *Application) closeDatabase() {
	if app.DB == nil {
		return
	}

	// Fold this run's accumulated statistics back into sqlite_stat1 so the next
	// startup plans against them. Best effort: a failure here must not block
	// shutdown.
	err := app.RefreshQueryPlannerStats()
	if err != nil {
		app.Logger.Error("failed to refresh query planner statistics", "error", err)
	}

	// Close prepared statements before the connection that owns them.
	if app.Queries != nil {
		err = app.Queries.Close()
		if err != nil {
			app.Logger.Error("failed to close prepared statements", "error", err)
		}
	}

	err = app.DB.Close()
	if err != nil {
		app.Logger.Error("failed to close database", "error", err)
	}
}

func (app *Application) closeLogger() {
	// Close the logger last so prior cleanup failures can still be logged.
	if app.LoggerCloser != nil {
		err := app.LoggerCloser()
		if err != nil {
			log.Printf("failed to close logger: %v", err)
		}
	}
}
