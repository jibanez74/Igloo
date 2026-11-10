package helpers

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	// gzipMinBytes is about one TCP segment: a smaller body is a single packet
	// either way, so compressing it only costs CPU.
	gzipMinBytes = 1400
	// maxPooledBuffer keeps one unusually large response from staying in the
	// pool for the life of the process.
	maxPooledBuffer = 1 << 20
)

var responseBuffers = sync.Pool{New: func() any { return new(bytes.Buffer) }}

// BestSpeed shrinks the API's JSON to about a quarter in half the CPU of the
// default level, which gains only another tenth.
var gzipWriters = sync.Pool{New: func() any {
	writer, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
	return writer
}}

// gzipAccepter is a response writer that knows whether its client accepts
// gzip. WriteBody takes no request, so the middleware that wraps the writer
// records the answer on it.
type gzipAccepter interface {
	AcceptsGzip() bool
}

func getResponseBuffer() *bytes.Buffer {
	buf := responseBuffers.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

func putResponseBuffer(buf *bytes.Buffer) {
	if buf.Cap() <= maxPooledBuffer {
		responseBuffers.Put(buf)
	}
}

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
	buf := getResponseBuffer()
	defer putResponseBuffer(buf)

	// The encoder writes into a reused buffer where json.Marshal returns a
	// fresh copy of every body. It ends the value with a newline that Marshal
	// does not write.
	err := json.NewEncoder(buf).Encode(data)
	if err != nil {
		return err
	}
	buf.Truncate(buf.Len() - 1)

	if len(headers) > 0 {
		for key, value := range headers[0] {
			w.Header()[key] = value
		}
	}

	w.Header().Set("Content-Type", "application/json")

	return WriteBody(w, status, buf.Bytes())
}

// WriteBody writes a text body the handler has in full, gzip-compressed when
// the client accepts it and the body is large enough to gain. The caller sets
// Content-Type first. Files and media never come through here: they are
// streamed, and compressing them would take them off the sendfile path.
func WriteBody(w http.ResponseWriter, status int, body []byte) error {
	header := w.Header()

	accepter, negotiates := w.(gzipAccepter)
	isCompressible := negotiates && len(body) >= gzipMinBytes
	if isCompressible {
		header.Add("Vary", "Accept-Encoding")
	}

	compress := isCompressible && accepter.AcceptsGzip()
	if compress {
		compressed := getResponseBuffer()
		defer putResponseBuffer(compressed)

		writer := gzipWriters.Get().(*gzip.Writer)
		defer gzipWriters.Put(writer)
		writer.Reset(compressed)

		_, err := writer.Write(body)
		if err == nil {
			err = writer.Close()
		}
		if err != nil {
			return err
		}

		body = compressed.Bytes()
		header.Set("Content-Encoding", "gzip")
	}

	header.Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)

	_, err := w.Write(body)

	return err
}

// maxRequestBytes is the body limit ReadJSON applies to every request, a
// ceiling no endpoint is expected to approach.
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
