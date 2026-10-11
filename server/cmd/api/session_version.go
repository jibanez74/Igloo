package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
)

// A cookie session records the user's session_version at login. Changing the
// password bumps the version, so every other session stops matching, and a
// deleted user has no version at all; IsAuth checks both on every cookie
// request. Device tokens are revoked by deleting their rows instead.
const cookieSessionVersion = "session_version"

const (
	// sessionVersionCacheTTL bounds how long a version read stays in memory.
	// Every write in this process updates the cache directly, so the TTL only
	// limits the window in which a read that raced a write could linger.
	sessionVersionCacheTTL   = 30 * time.Second
	sessionVersionCacheSweep = time.Minute

	// deletedUserSessionVersion marks a deleted user in the cache, so a read
	// that started before the delete cannot re-admit their sessions.
	deletedUserSessionVersion int64 = -1
)

var errSessionUserGone = errors.New("session user no longer exists")

func sessionVersionKey(userID int64) string {
	return strconv.FormatInt(userID, 10)
}

// currentSessionVersion returns the user's session_version, from the cache
// when it holds one. errSessionUserGone means the user was deleted.
func (app *Application) currentSessionVersion(ctx context.Context, userID int64) (int64, error) {
	key := sessionVersionKey(userID)
	cached, found := app.SessionVersionCache.Get(key)
	if found {
		version := cached.(int64)
		if version == deletedUserSessionVersion {
			return 0, errSessionUserGone
		}
		return version, nil
	}

	version, err := app.Queries.GetUserSessionVersion(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, errSessionUserGone
	}
	if err != nil {
		return 0, err
	}

	// Add, not Set: a password change or delete that landed after this read
	// already wrote the newer value, which must win.
	_ = app.SessionVersionCache.Add(key, version, sessionVersionCacheTTL)
	return version, nil
}

// rememberSessionVersion records a version this process just wrote.
func (app *Application) rememberSessionVersion(userID, version int64) {
	app.SessionVersionCache.Set(sessionVersionKey(userID), version, sessionVersionCacheTTL)
}

// forgetDeletedSessionUser ends every cookie session of a deleted user: the
// marker outlives any read that raced the delete, and once it expires the
// database has no row to find.
func (app *Application) forgetDeletedSessionUser(userID int64) {
	app.rememberSessionVersion(userID, deletedUserSessionVersion)
}

// requireCurrentCookieSession lets a cookie session through only while its
// user exists and its recorded version is current. Otherwise it ends the
// session and answers 401, or 500 when the version cannot be read. It reports
// whether the request may continue.
func (app *Application) requireCurrentCookieSession(w http.ResponseWriter, r *http.Request, userID int64) bool {
	current, err := app.currentSessionVersion(r.Context(), userID)
	if err == nil && current == app.SessionManager.GetInt64(r.Context(), cookieSessionVersion) {
		return true
	}
	if err != nil && !errors.Is(err, errSessionUserGone) {
		app.Logger.Error("failed to read session version", "error", err, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return false
	}

	destroyErr := app.SessionManager.Destroy(r.Context())
	if destroyErr != nil {
		app.Logger.Error("failed to destroy a revoked session", "error", destroyErr, "user_id", userID)
	}
	helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
	return false
}

// changePassword stores a new password hash, bumps the session version, and
// revokes every device token the user holds, in one transaction. Every cookie
// session that recorded the old version stops matching; the caller decides
// whether its own session carries on with the returned version.
func (app *Application) changePassword(ctx context.Context, userID int64, passwordHash string) (int64, error) {
	tx, err := app.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	qtx := app.Queries.WithTx(tx)
	version, err := qtx.UpdateUserPassword(ctx, database.UpdateUserPasswordParams{
		Password: passwordHash,
		ID:       userID,
	})
	if err != nil {
		return 0, err
	}
	err = qtx.DeleteDevicesForUser(ctx, userID)
	if err != nil {
		return 0, err
	}
	err = tx.Commit()
	if err != nil {
		return 0, err
	}

	app.rememberSessionVersion(userID, version)
	app.forgetUserDevices(userID)
	return version, nil
}

// keepSessionAfterPasswordChange moves the caller's own cookie session to the
// new version, under a fresh token, so the change signs out every session but
// the one that made it.
func (app *Application) keepSessionAfterPasswordChange(r *http.Request, userID, version int64) {
	err := app.SessionManager.RenewToken(r.Context())
	if err != nil {
		app.Logger.Error("failed to renew session token after password change", "error", err, "user_id", userID)
	}
	app.SessionManager.Put(r.Context(), cookieSessionVersion, version)
}
