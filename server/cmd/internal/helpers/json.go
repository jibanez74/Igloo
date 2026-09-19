package helpers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
)

// readJSONTimeout bounds how long a handler will wait for a request body.
// http.MaxBytesReader caps the body's size but not the time taken to send it,
// so a slow client could otherwise hold a goroutine open indefinitely. This is
// applied per request in ReadJSON rather than as http.Server.ReadTimeout,
// which would also apply to the hijacked watch-room WebSocket.
const readJSONTimeout = 15 * time.Second

// setReadDeadline bounds (deadline) or clears (zero deadline) the time allowed
// to read the rest of the request. Responses that do not support deadlines —
// httptest.ResponseRecorder in the handler tests — are left alone; nothing else
// is worth failing a request over, since the deadline is only a guard rail.
func setReadDeadline(w http.ResponseWriter, deadline time.Time) {
	_ = http.NewResponseController(w).SetReadDeadline(deadline)
}

type JSONResponse struct {
	Error   bool   `json:"error"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

// WriteJSON marshals data to JSON and writes it to the response with the given status code.
// Optional headers can be provided to add custom response headers.
func WriteJSON(w http.ResponseWriter, status int, data any, headers ...http.Header) error {
	out, err := json.Marshal(data)
	if err != nil {
		return err
	}

	if len(headers) > 0 {
		for key, value := range headers[0] {
			w.Header()[key] = value
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_, err = w.Write(out)

	return err
}

// maxRequestBytes is the body limit ReadJSON applies to every request. It is a
// ceiling on one JSON request body, not a payload size any endpoint is expected
// to approach. No handler has ever needed a different limit, so ReadJSON does
// not take one.
const maxRequestBytes int64 = 1024 * 1024

// ReadJSON decodes JSON from the request body into data.
// It enforces a maximum body size, a read deadline, and rejects unknown fields.
func ReadJSON(w http.ResponseWriter, r *http.Request, data any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	defer setReadDeadline(w, time.Time{})
	setReadDeadline(w, time.Now().Add(readJSONTimeout))

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	err := dec.Decode(data)
	if err != nil {
		return err
	}

	err = dec.Decode(&struct{}{})
	if !errors.Is(err, io.EOF) {
		return errors.New("body must only contain a single JSON value")
	}

	return nil
}

// ErrorJSON writes an error response with the given status code (defaults to 500).
func ErrorJSON(w http.ResponseWriter, err error, status ...int) error {
	statusCode := http.StatusInternalServerError
	if len(status) > 0 {
		statusCode = status[0]
	}

	return WriteJSON(w, statusCode, JSONResponse{
		Error:   true,
		Message: err.Error(),
	})
}
