package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
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
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		app.ListenForShutdown()
	}()

	err := app.Server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	<-shutdownDone
	return nil
}

// ListenForShutdown blocks until SIGINT or SIGTERM, then stops the HTTP
// server and releases every resource the process owns. It returns rather than
// exiting so the caller decides when the process ends.
func (app *Application) ListenForShutdown() {
	quit := make(chan os.Signal, 1)

	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit

	signal.Stop(quit)

	app.Logger.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	app.shutdownHTTPServer(ctx)

	app.Logger.Info("running clean up tasks...")

	app.shutdownWatchRoomHub()
	app.cleanupHLSSessions()
	if app.DeviceExpiryCancel != nil {
		app.DeviceExpiryCancel()
	}
	app.cancelScans()

	// Background tasks may still need database and logger access.
	app.Wait.Wait()

	app.flushRuntimeCaches()
	app.clearMediaClientCaches()
	app.cleanupMediaBinaries()
	app.closeDatabase()
	app.closeLogger()
}

func (app *Application) cancelScans() {
	if app.ScanCancel != nil {
		app.ScanCancel()
	}
}

func (app *Application) shutdownHTTPServer(ctx context.Context) {
	if app.Server == nil {
		return
	}

	err := app.Server.Shutdown(ctx)
	if err != nil {
		app.Logger.Error("failed to shutdown server", "error", err)
	}
}

func (app *Application) shutdownWatchRoomHub() {
	if app.WatchRoomHub != nil {
		app.WatchRoomHub.Shutdown()
	}
}

func (app *Application) cleanupHLSSessions() {
	// Stop FFmpeg sessions before cleaning up the FFmpeg binary.
	if app.HLSSessionCache != nil {
		count := 0
		for _, item := range app.HLSSessionCache.Items() {
			session, ok := item.Object.(*HLSSession)
			if ok {
				cleanupHLSSession(session)
				count++
			}
		}
		if count > 0 {
			app.Logger.Info("cleaned up HLS sessions", "count", count)
		}
		app.HLSSessionCache.Flush()
	}
}

func (app *Application) flushRuntimeCaches() {
	if app.SubtitleVTTCache != nil {
		app.SubtitleVTTCache.Flush()
	}
	if app.RoomHLSTombstone != nil {
		app.RoomHLSTombstone.Flush()
	}
}

func (app *Application) clearMediaClientCaches() {
	if app.Spotify != nil {
		app.Spotify.ClearAllCaches()
	}
	if app.Tmdb != nil {
		app.Tmdb.ClearCache()
	}
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
	// Fold this run's accumulated statistics back into sqlite_stat1 so the next
	// startup plans against them. Best effort: a failure here must not block
	// shutdown.
	if app.DB != nil {
		err := app.RefreshQueryPlannerStats()
		if err != nil {
			app.Logger.Error("failed to refresh query planner statistics", "error", err)
		}
	}

	// Close prepared statements before the connection that owns them.
	if app.Queries != nil {
		err := app.Queries.Close()
		if err != nil {
			app.Logger.Error("failed to close prepared statements", "error", err)
		}
	}

	if app.DB != nil {
		err := app.DB.Close()
		if err != nil {
			app.Logger.Error("failed to close database", "error", err)
		}
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
