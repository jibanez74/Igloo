package main

import (
	"context"
	"time"
)

// sweepStaleDevices revokes devices whose last_used_at is older than the
// inactivity TTL, keeping the devices list free of dead entries whose owners
// never trip the lazy auth-time check.
func (app *Application) sweepStaleDevices(ctx context.Context) {
	deleted, err := app.Queries.DeleteDevicesUnusedSince(ctx, deviceInactivityCutoff(time.Now()))
	if err != nil {
		app.Logger.Error("failed to sweep stale devices", "error", err)
		return
	}

	if deleted > 0 {
		app.Logger.Info("revoked stale devices", "count", deleted)
	}
}

// runDeviceExpirySweeper re-runs the sweep daily for long-lived processes.
func (app *Application) runDeviceExpirySweeper() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		app.sweepStaleDevices(context.Background())
	}
}
