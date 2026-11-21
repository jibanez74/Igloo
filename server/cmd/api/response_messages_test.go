package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServerErrorLogLevel(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantLevel string
	}{
		{name: "cancelled request", err: fmt.Errorf("read version: %w", context.Canceled), wantLevel: "DEBUG"},
		{name: "deadline exceeded", err: fmt.Errorf("read version: %w", context.DeadlineExceeded), wantLevel: "ERROR"},
		{name: "ordinary failure", err: errors.New("disk I/O error"), wantLevel: "ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logs bytes.Buffer
			app := &Application{Logger: slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))}
			rec := httptest.NewRecorder()

			app.serverError(rec, tt.err, "failed to read session version", internalServerErrorMessage, "user_id", int64(7))

			line := strings.TrimSpace(logs.String())
			if strings.Count(line, "\n") != 0 {
				t.Fatalf("want one log line, got %q", line)
			}
			if !strings.Contains(line, "level="+tt.wantLevel) {
				t.Fatalf("want level %s, got %q", tt.wantLevel, line)
			}
			for _, want := range []string{`msg="failed to read session version"`, "user_id=7", "error="} {
				if !strings.Contains(line, want) {
					t.Fatalf("log line %q lacks %q", line, want)
				}
			}

			if rec.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want 500", rec.Code)
			}
			var body struct {
				Error   bool   `json:"error"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if !body.Error || body.Message != internalServerErrorMessage {
				t.Fatalf("body = %+v, want the client message only", body)
			}
		})
	}
}
