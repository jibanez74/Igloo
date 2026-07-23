package main

import (
	"context"
	"igloo/cmd/internal/helpers"
)

// scannerBatchSize is how many files a library scan buffers before flushing them
// to the database.
const scannerBatchSize = 54

// Library scans are single-flighted across the startup scan and the
// admin-triggered scan endpoints.
var (
	movieScanGuard helpers.ScanGuard
	musicScanGuard helpers.ScanGuard
)

// scanContext returns the shutdown-aware context library scans run under, and
// falls back to a background context when the application has none configured.
func (app *Application) scanContext() context.Context {
	if app.ScanContext != nil {
		return app.ScanContext
	}

	return context.Background()
}
