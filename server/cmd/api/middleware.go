package main

import (
	"context"
	"database/sql"
	"errors"
	"igloo/cmd/internal/helpers"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type contextKey string

const deviceAuthKey contextKey = "deviceAuth"

type deviceAuth struct {
	UserID   int64
	DeviceID int64
}

func deviceAuthFrom(ctx context.Context) *deviceAuth {
	auth, ok := ctx.Value(deviceAuthKey).(*deviceAuth)
	if !ok {
		return nil
	}
	return auth
}

// DeviceTokenAuth resolves "Authorization: Bearer igd_..." device tokens.
// It is attached only to routes where device auth is part of the contract
// (protected API routes, the watch-room WebSocket, and /api/auth/user), so
// public pairing and login routes stay reachable with a stale token.
// Requests without such a header pass through untouched; an invalid or
// revoked token is rejected immediately rather than falling back to cookies.
// The session is never written to, so bearer requests do not create session
// rows.
//
// Resolved tokens are cached because a TV client re-authenticates on every HLS
// segment, and the database runs on a single shared connection. Revocation
// evicts the entry so a revoked token stops working immediately.
func (app *Application) DeviceTokenAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if header == "" {
			next.ServeHTTP(w, r)
			return
		}

		token, found := strings.CutPrefix(header, "Bearer ")
		if !found || !strings.HasPrefix(token, deviceTokenPrefix) {
			next.ServeHTTP(w, r)
			return
		}

		tokenHash := hashDeviceToken(token)

		cached, hit := app.DeviceAuthCache.Get(tokenHash)
		if hit {
			auth, ok := cached.(*deviceAuth)
			if ok {
				app.touchDeviceLastUsed(r.Context(), auth.DeviceID)
				next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), deviceAuthKey, auth)))
				return
			}
		}

		device, err := app.Queries.GetDeviceByTokenHash(r.Context(), tokenHash)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
				return
			}

			if app.Logger != nil {
				app.Logger.Error("failed to look up device token", "error", err)
			}
			helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
			return
		}

		// A device that has been idle past the inactivity TTL is revoked on
		// the spot; it must pair again (public pairing routes stay reachable).
		if device.LastUsedAt < deviceInactivityCutoff(time.Now()) {
			err = app.Queries.DeleteDevice(r.Context(), device.ID)
			if err != nil && app.Logger != nil {
				app.Logger.Error("failed to delete stale device", "error", err, "device_id", device.ID)
			}
			app.forgetDevice(device.ID)
			helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
			return
		}

		app.touchDeviceLastUsed(r.Context(), device.ID)

		auth := &deviceAuth{UserID: device.UserID, DeviceID: device.ID}
		app.DeviceAuthCache.SetDefault(tokenHash, auth)

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), deviceAuthKey, auth)))
	})
}

// LoadAndSaveSession wraps the scs session middleware.
func (app *Application) LoadAndSaveSession(next http.Handler) http.Handler {
	return app.SessionManager.LoadAndSave(next)
}

// LoadSessionReadOnly loads the session into the request context without
// committing changes or wrapping the response writer.
func (app *Application) LoadSessionReadOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Cookie")

		var token string
		cookie, err := r.Cookie(app.SessionManager.Cookie.Name)
		if err == nil {
			token = cookie.Value
		}

		ctx, err := app.SessionManager.Load(r.Context(), token)
		if err != nil {
			if app.Logger != nil {
				app.Logger.Error("failed to load read-only session", "error", err)
			}
			helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
			return
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// IsAuth rejects unauthenticated requests with 401.
func (app *Application) IsAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if deviceAuthFrom(r.Context()) != nil {
			next.ServeHTTP(w, r)
			return
		}

		if !app.SessionManager.Exists(r.Context(), cookieUserID) {
			helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// userIDFromRequest resolves the authenticated user from either a device
// token (context) or the session cookie. Returns 0 when unauthenticated.
func (app *Application) userIDFromRequest(r *http.Request) int64 {
	auth := deviceAuthFrom(r.Context())
	if auth != nil {
		return auth.UserID
	}
	return app.SessionManager.GetInt64(r.Context(), cookieUserID)
}

func (app *Application) currentUserID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	userID := app.userIDFromRequest(r)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
		return 0, false
	}
	return userID, true
}

// requireSessionUserID resolves the user from the session cookie only.
// Device bearer tokens are rejected so a stolen device token cannot manage
// (enumerate, rename, revoke) devices or approve pairing codes to mint
// further devices.
func (app *Application) requireSessionUserID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	if deviceAuthFrom(r.Context()) != nil {
		helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
		return 0, false
	}

	userID := app.SessionManager.GetInt64(r.Context(), cookieUserID)
	if userID == 0 {
		helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
		return 0, false
	}

	return userID, true
}

// touchDeviceLastUsed updates devices.last_used_at, throttled through the
// DeviceLastSeen cache so each device writes at most once per TTL.
func (app *Application) touchDeviceLastUsed(ctx context.Context, deviceID int64) {
	key := strconv.FormatInt(deviceID, 10)
	_, fresh := app.DeviceLastSeen.Get(key)
	if fresh {
		return
	}

	err := app.Queries.UpdateDeviceLastUsed(ctx, deviceID)
	if err != nil {
		if app.Logger != nil {
			app.Logger.Error("failed to update device last_used_at", "error", err)
		}
		return
	}

	app.DeviceLastSeen.SetDefault(key, struct{}{})
}

// forgetDeviceAuth evicts every cached bearer resolution the predicate matches.
// The cache is keyed by token hash, which no revocation path carries, so its
// entries are found by what they resolved to.
func (app *Application) forgetDeviceAuth(match func(*deviceAuth) bool) {
	for key, item := range app.DeviceAuthCache.Items() {
		auth, ok := item.Object.(*deviceAuth)
		if ok && match(auth) {
			app.DeviceAuthCache.Delete(key)
		}
	}
}

// forgetDevice drops the caches keyed on a device so a revoked device is
// rejected on its next request.
func (app *Application) forgetDevice(deviceID int64) {
	app.DeviceLastSeen.Delete(strconv.FormatInt(deviceID, 10))
	app.forgetDeviceAuth(func(auth *deviceAuth) bool {
		return auth.DeviceID == deviceID
	})
}

// forgetUserDevices drops cached bearer auth for every device belonging to a
// user. Deleting the user cascades its device rows away, so without this a
// token could keep authenticating until the cache entry expired.
func (app *Application) forgetUserDevices(userID int64) {
	app.forgetDeviceAuth(func(auth *deviceAuth) bool {
		return auth.UserID == userID
	})
}

// RequireAdmin rejects requests from non-admin users with 403.
func (app *Application) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := app.currentUserID(w, r)
		if !ok {
			return
		}

		isAdmin, err := app.Queries.GetUserIsAdmin(r.Context(), userID)
		if err != nil {
			helpers.ErrorJSON(w, errors.New(notAuthorizedMessage), http.StatusUnauthorized)
			return
		}

		if !isAdmin {
			helpers.ErrorJSON(w, errors.New("admin access required"), http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
