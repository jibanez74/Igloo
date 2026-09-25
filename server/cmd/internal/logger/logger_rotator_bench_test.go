package logger

import "testing"

// Benchmarks the request-logging hot path: one JSON line per HTTP request,
// including the rotation cost each time the byte cap is reached.
func BenchmarkRotatingWriterWrite(b *testing.B) {
	w, _ := newTestWriter(b, loggerMaxBytes, "", loggerFlushInterval)

	line := []byte(`{"time":"2026-08-12T00:00:00Z","level":"INFO","msg":"request completed","method":"GET","path":"/api/movies/latest","status":200,"duration_ms":12}` + "\n")

	b.SetBytes(int64(len(line)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := w.Write(line)
		if err != nil {
			b.Fatal(err)
		}
	}
	// Close flushes the buffer and closes the file, so keep it out of the
	// per-line measurement.
	b.StopTimer()

	err := w.Close()
	if err != nil {
		b.Fatal(err)
	}
}
