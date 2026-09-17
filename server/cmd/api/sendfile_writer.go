package main

import (
	"io"
	"net/http"
)

// restoreSendfile re-exposes io.ReaderFrom on the response writer so
// http.ServeContent and http.ServeFile can reach the kernel's sendfile(2)
// path when they stream a file.
//
// scs.LoadAndSave wraps the writer in a type that implements only Write,
// WriteHeader and Unwrap, and it is the innermost wrapper a handler sees.
// io.Copy therefore finds no io.ReaderFrom and falls back to a 32 KiB
// userspace copy loop for every byte of every movie, track and HLS segment
// (audit finding D6). Unwrapping to the outermost io.ReaderFrom — chi's
// wrapped writer rather than the raw response beneath it — keeps the request
// logger's byte counter accurate.
func restoreSendfile(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, alreadyCapable := w.(io.ReaderFrom)
		if alreadyCapable {
			next.ServeHTTP(w, r)
			return
		}

		readerFrom, found := unwrapReaderFrom(w)
		if !found {
			next.ServeHTTP(w, r)
			return
		}

		next.ServeHTTP(&sendfileResponseWriter{ResponseWriter: w, readerFrom: readerFrom}, r)
	})
}

// unwrapReaderFrom walks the Unwrap chain and returns the first writer that
// implements io.ReaderFrom.
func unwrapReaderFrom(w http.ResponseWriter) (io.ReaderFrom, bool) {
	for {
		unwrapper, ok := w.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			return nil, false
		}

		w = unwrapper.Unwrap()

		readerFrom, ok := w.(io.ReaderFrom)
		if ok {
			return readerFrom, true
		}
	}
}

type sendfileResponseWriter struct {
	http.ResponseWriter
	readerFrom  io.ReaderFrom
	wroteHeader bool
}

func (w *sendfileResponseWriter) Write(b []byte) (int, error) {
	w.wroteHeader = true
	return w.ResponseWriter.Write(b)
}

func (w *sendfileResponseWriter) WriteHeader(code int) {
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(code)
}

// ReadFrom hands the body to the unwrapped writer, so the response headers and
// the session must be committed through the wrapped chain first.
func (w *sendfileResponseWriter) ReadFrom(r io.Reader) (int64, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}

	return w.readerFrom.ReadFrom(r)
}

func (w *sendfileResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
