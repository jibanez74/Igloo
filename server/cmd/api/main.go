package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
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

	app.Server = &http.Server{
		Addr:    fmt.Sprintf(":%d", app.Config.Port),
		Handler: app.Router,
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
