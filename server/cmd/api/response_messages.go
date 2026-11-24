package main

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"igloo/cmd/internal/helpers"
)

// Messages said by more than one handler file. A handler that needs to say
// something different keeps its own local constant, and a message owned by a
// single feature lives in that feature's file instead.
const (
	internalServerErrorMessage = "The server encountered an unexpected error"
	notAuthorizedMessage       = "not authorized"
	invalidRequestBodyMessage  = "invalid request body"
	accessDeniedMessage        = "access denied"
	tooManyAttemptsMessage     = "too many attempts, please try again later"

	// Said by the movie and the episode technical-details endpoints.
	fetchTechnicalDetailsMessage = "failed to fetch technical details"
)

// tooManyAttempts answers 429 for a rate-limited attempt. Retry-After names the
// wait in whole seconds, rounded up so a client that waits exactly that long
// lands in the next window, and never below one second.
func tooManyAttempts(w http.ResponseWriter, retryAfter time.Duration) {
	seconds := max(int64((retryAfter+time.Second-1)/time.Second), 1)
	w.Header().Set("Retry-After", strconv.FormatInt(seconds, 10))
	helpers.ErrorJSON(w, errors.New(tooManyAttemptsMessage), http.StatusTooManyRequests)
}

// Log lines repeated across handlers that share no feature file.
const beginTransactionLogMessage = "failed to begin transaction"

// serverError logs err under logMessage, with any extra attrs, and answers 500
// with clientMessage; the client never sees err itself. A cancelled request
// (the client went away, or the server is shutting down) is logged at Debug:
// nothing failed, and nobody reads the answer. A deadline that ran out is
// still an error.
func (app *Application) serverError(w http.ResponseWriter, err error, logMessage, clientMessage string, attrs ...any) {
	args := append([]any{"error", err}, attrs...)
	if errors.Is(err, context.Canceled) {
		app.Logger.Debug(logMessage, args...)
	} else {
		app.Logger.Error(logMessage, args...)
	}
	helpers.ErrorJSON(w, errors.New(clientMessage))
}
