package logger

import (
	"path/filepath"
	"testing"
)

// Benchmarks the request-logging hot path: one JSON line per HTTP request,
// including the rotation cost once the line cap is reached.
func BenchmarkRotatingWriterWrite(b *testing.B) {
	path := filepath.Join(b.TempDir(), "bench.log")
	w, err := newRotatingWriter(path, loggerMaxLines)
	if err != nil {
		b.Fatal(err)
	}
	defer w.Close()

	line := []byte(`{"time":"2026-08-12T00:00:00Z","level":"INFO","msg":"request completed","method":"GET","path":"/api/movies/latest","status":200,"duration_ms":12}` + "\n")

	b.SetBytes(int64(len(line)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := w.Write(line)
		if err != nil {
			b.Fatal(err)
		}
	}
}
