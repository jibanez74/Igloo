package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"igloo/cmd/internal/scanner"
)

func main() {
	log.Println("igloo server starting up...")

	envFile, loadedEnvFile, err := LoadRuntimeEnvFile()
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

	startLibraryScansAtStartup(app)

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
	app.Wait.Add(1)
	go func() {
		defer app.Wait.Done()
		app.runDeviceExpirySweeper(deviceExpiryCtx)
	}()

	go app.ListenForShutdown()

	startupSweepCtx, cancelStartupSweep := context.WithTimeout(deviceExpiryCtx, deviceExpirySweepTimeout)
	app.sweepStaleDevices(startupSweepCtx)
	cancelStartupSweep()

	log.Printf("server listening on port %d", app.Config.Port)

	err = app.Server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		// InitApp already extracted the ffmpeg/ffprobe binaries and opened the
		// database and logger, so a serve failure (e.g. port already in use) must
		// run cleanup rather than exit bare.
		app.Logger.Error("server failed to start", "error", err)
		app.DeviceExpiryCancel()
		app.cancelScans()
		app.Wait.Wait()
		app.cleanupStartupResources()
		os.Exit(1)
	}
}

func startLibraryScansAtStartup(app *Application) {
	app.startScanAtStartup(movieLibrary, app.MovieScanner.Start())
	app.startScanAtStartup(musicLibrary, app.MusicScanner.Start())
}

func (app *Application) startScanAtStartup(library scanLibrary, result scanner.StartResult) {
	switch result.Status {
	case scanner.StartNotConfigured:
		app.Logger.Info("skipping " + library.name + " library scan: " + library.directory + " directory is not configured")
	case scanner.StartAlreadyRunning:
		app.Logger.Warn(library.name + " library scan is already in progress")
	}
}
