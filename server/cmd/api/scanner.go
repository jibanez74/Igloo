package main

import (
	"context"
	"igloo/cmd/internal/scanner"
)

const scannerBatchSize = scanner.BatchSize

var (
	musicScanGuard scanner.ScanGuard
)

// scanContext returns the shutdown-aware context library scans run under, and
// falls back to a background context when the application has none configured.
func (app *Application) scanContext() context.Context {
	if app.ScanContext != nil {
		return app.ScanContext
	}

	return context.Background()
}
