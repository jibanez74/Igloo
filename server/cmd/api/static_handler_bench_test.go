package main

import (
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"
)

// Benchmarks the SPA hot path, which currently re-reads the embedded asset on
// every request. Skips when webdist holds only the test placeholder.
func BenchmarkServeFrontend(b *testing.B) {
	_, err := FrontendFS.Open("webdist/index.html")
	if err != nil {
		b.Skip("webdist is not populated; run make prepare-web first")
	}

	b.Setenv("VITE_DEV_SERVER", "")
	app := &Application{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/index.html", nil)
		app.ServeFrontend(w, r)
		if w.Code != 200 {
			b.Fatalf("unexpected status %d", w.Code)
		}
	}
}
