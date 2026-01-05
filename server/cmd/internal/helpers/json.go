package helpers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

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

// ReadJSON decodes JSON from the request body into data.
// It enforces a maximum body size and rejects unknown fields.
func ReadJSON(w http.ResponseWriter, r *http.Request, data any, maxBytes int64) error {
	if maxBytes == 0 {
		maxBytes = 1024 * 1024 // 1 MB default
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

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
