package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"igloo/cmd/internal/scanner"
)

func main() {
	log.Println("igloo server starting up...")

	loadedEnvFile, err := LoadRuntimeEnvFile()
	if err != nil {
		log.Printf("warning: %v", err)
	}

	if loadedEnvFile {
		log.Printf("loaded environment file %s", envFile)
	}

	app, err := InitApp()
	if err != nil {
		log.Fatal(err)
	}

	startLibraryWork(app)

	app.Server = &http.Server{
		Addr:    fmt.Sprintf(":%d", app.Config.Port),
		Handler: app.Router,
		// No WriteTimeout: it would cut off long-running direct-play and HLS
		// responses. No ReadTimeout either: it applies to the watch-room
		// WebSocket, whose read deadline survives the hijack, so it would kill
		// idle rooms. Request bodies are bounded per request instead, by
		// helpers.ReadJSON. ReadHeaderTimeout bounds header parsing so idle or
		// malicious connections cannot hold a goroutine open indefinitely.
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	deviceExpiryCtx, cancelDeviceExpiry := context.WithCancel(context.Background())
	app.DeviceExpiryCancel = cancelDeviceExpiry
	app.Wait.Go(func() {
		app.runDeviceExpirySweeper(deviceExpiryCtx)
	})

	startupSweepCtx, cancelStartupSweep := context.WithTimeout(deviceExpiryCtx, deviceExpirySweepTimeout)
	app.sweepStaleDevices(startupSweepCtx)
	cancelStartupSweep()

	log.Printf("server listening on port %d", app.Config.Port)

	err = app.serveUntilShutdown()
	if err != nil {
		// InitApp already extracted the ffmpeg/ffprobe binaries and opened the
		// database and logger, so a serve failure (e.g. port already in use) must
		// run cleanup rather than exit bare.
		app.Logger.Error("server failed to start", "error", err)
		app.DeviceExpiryCancel()
		app.ScanCancel()
		app.Wait.Wait()
		app.cleanupStartupResources()
		os.Exit(1)
	}
}

// startLibraryWork follows the stored enable_watcher setting, which the
// ENABLE_WATCHER environment value seeds only for a new database. With the
// watcher on, each library is walked once as soon as its watches are in place,
// so no change made during that walk is missed; otherwise the startup scans
// run as before and nothing watches the libraries. A startup scan is a
// catch-up, like the watcher's walks: nothing saw what changed while the
// server was down, and a disk that has not mounted yet must not read as
// deleted files.
func startLibraryWork(app *Application) {
	// A first artwork pass catches up with a setting changed, or a static
	// directory emptied, while the server was down.
	app.syncArtwork()
	enabled := app.CurrentSettings().EnableWatcher
	if enabled {
		app.Watcher.Sync(true)
		return
	}
	app.Logger.Info("library watcher is off; libraries change only through scans")
	startLibraryScansAtStartup(app)
}

func startLibraryScansAtStartup(app *Application) {
	catchUp := scanner.Changes{CatchUp: true}
	app.startScanAtStartup(movieLibrary, app.MovieScanner.StartChanges(catchUp))
	app.startScanAtStartup(showLibrary, app.ShowScanner.StartChanges(catchUp))
	app.startScanAtStartup(musicLibrary, app.MusicScanner.StartChanges(catchUp))
}

func (app *Application) startScanAtStartup(library scanLibrary, result scanner.StartResult) {
	switch result.Status {
	case scanner.StartNotConfigured:
		app.Logger.Info("skipping " + library.name + " library scan: " + library.directory + " directory is not configured")
	case scanner.StartAlreadyRunning:
		app.Logger.Warn(library.name + " library scan is already in progress")
	}
}
