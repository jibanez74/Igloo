package helpers

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// negotiatingRecorder stands in for the writer the session group hands to
// handlers, which knows whether the client accepts gzip.
type negotiatingRecorder struct {
	*httptest.ResponseRecorder
	accepts bool
}

func (r *negotiatingRecorder) AcceptsGzip() bool {
	return r.accepts
}

type jsonTestCredit struct {
	Name      string  `json:"name"`
	Character string  `json:"character"`
	Order     int     `json:"order"`
	Profile   *string `json:"profile"`
}

func jsonTestCredits(count int) JSONResponse {
	profile := "/pB8BM7pdSp6B6Ih7QZ4DrQ3PmJK.jpg"
	credits := make([]jsonTestCredit, count)
	for i := range credits {
		credits[i] = jsonTestCredit{Name: "Person <" + strconv.Itoa(i) + "> & co", Character: "Role \"" + strconv.Itoa(i) + "\"", Order: i, Profile: &profile}
	}
	return JSONResponse{Data: map[string]any{"cast": credits}}
}

// headerRecorder reports whether WriteHeader was called, which the recorder's
// default 200 Code cannot tell apart from an explicit 200.
type headerRecorder struct {
	*httptest.ResponseRecorder
	wroteHeader bool
}

func (r *headerRecorder) WriteHeader(status int) {
	r.wroteHeader = true
	r.ResponseRecorder.WriteHeader(status)
}

func TestWriteJSON(t *testing.T) {
	t.Run("writes valid JSON with status code", func(t *testing.T) {
		w := httptest.NewRecorder()
		data := map[string]string{"message": "hello"}

		err := WriteJSON(w, http.StatusOK, data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		contentType := w.Header().Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", contentType)
		}

		expected := `{"message":"hello"}`
		if w.Body.String() != expected {
			t.Errorf("expected body %s, got %s", expected, w.Body.String())
		}
	})

	t.Run("writes with custom headers", func(t *testing.T) {
		w := httptest.NewRecorder()
		data := map[string]string{"status": "ok"}
		customHeaders := http.Header{
			"X-Custom-Header": []string{"custom-value"},
		}

		err := WriteJSON(w, http.StatusCreated, data, customHeaders)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if w.Code != http.StatusCreated {
			t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
		}

		if w.Header().Get("X-Custom-Header") != "custom-value" {
			t.Errorf("expected custom header value, got %s", w.Header().Get("X-Custom-Header"))
		}
	})

	t.Run("writes the bytes json.Marshal produces", func(t *testing.T) {
		for _, count := range []int{0, 1, 400} {
			data := jsonTestCredits(count)
			want, err := json.Marshal(data)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			w := httptest.NewRecorder()
			err = WriteJSON(w, http.StatusOK, data)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !bytes.Equal(w.Body.Bytes(), want) {
				t.Errorf("%d credits: body differs from json.Marshal", count)
			}
			if w.Header().Get("Content-Length") != strconv.Itoa(len(want)) {
				t.Errorf("%d credits: Content-Length = %q, want %d", count, w.Header().Get("Content-Length"), len(want))
			}
			if w.Header().Get("Content-Encoding") != "" || w.Header().Get("Vary") != "" {
				t.Errorf("%d credits: a writer that does not negotiate got %v", count, w.Header())
			}
		}
	})

	t.Run("compresses a large body for a client that accepts gzip", func(t *testing.T) {
		data := jsonTestCredits(400)
		want, _ := json.Marshal(data)

		w := &negotiatingRecorder{ResponseRecorder: httptest.NewRecorder(), accepts: true}
		err := WriteJSON(w, http.StatusOK, data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if w.Header().Get("Content-Encoding") != "gzip" || w.Header().Get("Vary") != "Accept-Encoding" {
			t.Fatalf("headers = %v, want gzip and Vary: Accept-Encoding", w.Header())
		}
		if w.Header().Get("Content-Length") != strconv.Itoa(w.Body.Len()) {
			t.Errorf("Content-Length = %q, want the compressed length %d", w.Header().Get("Content-Length"), w.Body.Len())
		}
		if w.Body.Len() >= len(want)/2 {
			t.Errorf("compressed body is %d bytes of %d", w.Body.Len(), len(want))
		}

		reader, err := gzip.NewReader(w.Body)
		if err != nil {
			t.Fatalf("body is not gzip: %v", err)
		}
		got, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("read gzip body: %v", err)
		}
		if !bytes.Equal(got, want) {
			t.Error("decompressed body differs from json.Marshal")
		}
	})

	t.Run("leaves a large body alone for a client that refuses gzip", func(t *testing.T) {
		data := jsonTestCredits(400)
		want, _ := json.Marshal(data)

		w := &negotiatingRecorder{ResponseRecorder: httptest.NewRecorder()}
		err := WriteJSON(w, http.StatusOK, data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if w.Header().Get("Content-Encoding") != "" {
			t.Errorf("Content-Encoding = %q for a client that refused gzip", w.Header().Get("Content-Encoding"))
		}
		// The other representation exists, so a cache must still key on it.
		if w.Header().Get("Vary") != "Accept-Encoding" {
			t.Errorf("Vary = %q, want Accept-Encoding", w.Header().Get("Vary"))
		}
		if !bytes.Equal(w.Body.Bytes(), want) {
			t.Error("body differs from json.Marshal")
		}
	})

	t.Run("leaves a body of one packet alone", func(t *testing.T) {
		w := &negotiatingRecorder{ResponseRecorder: httptest.NewRecorder(), accepts: true}
		err := WriteJSON(w, http.StatusOK, jsonTestCredits(3))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if w.Body.Len() >= gzipMinBytes {
			t.Fatalf("test body is %d bytes, want one under %d", w.Body.Len(), gzipMinBytes)
		}
		if w.Header().Get("Content-Encoding") != "" || w.Header().Get("Vary") != "" {
			t.Errorf("small body got %v", w.Header())
		}
	})

	// Marshalling happens before anything touches the response, so a handler
	// that fails here can still write its own error response.
	t.Run("writes nothing for unmarshalable data", func(t *testing.T) {
		w := &headerRecorder{ResponseRecorder: httptest.NewRecorder()}
		data := make(chan int)

		err := WriteJSON(w, http.StatusOK, data)
		if err == nil {
			t.Fatal("expected error for unmarshalable data, got nil")
		}

		if w.wroteHeader {
			t.Error("status was written despite the marshal failure")
		}
		if w.Header().Get("Content-Type") != "" {
			t.Errorf("Content-Type was set despite the marshal failure: %q", w.Header().Get("Content-Type"))
		}
		if w.Body.Len() != 0 {
			t.Errorf("body was written despite the marshal failure: %q", w.Body.String())
		}
	})
}

func TestReadJSON(t *testing.T) {
	t.Run("reads valid JSON", func(t *testing.T) {
		body := strings.NewReader(`{"name":"test","value":123}`)
		r := httptest.NewRequest(http.MethodPost, "/", body)
		w := httptest.NewRecorder()

		var data struct {
			Name  string `json:"name"`
			Value int    `json:"value"`
		}

		err := ReadJSON(w, r, &data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if data.Name != "test" {
			t.Errorf("expected name 'test', got '%s'", data.Name)
		}
		if data.Value != 123 {
			t.Errorf("expected value 123, got %d", data.Value)
		}
	})

	t.Run("rejects unknown fields", func(t *testing.T) {
		body := strings.NewReader(`{"name":"test","unknown_field":"value"}`)
		r := httptest.NewRequest(http.MethodPost, "/", body)
		w := httptest.NewRecorder()

		var data struct {
			Name string `json:"name"`
		}

		err := ReadJSON(w, r, &data)
		if err == nil {
			t.Error("expected error for unknown field, got nil")
		}
	})

	t.Run("enforces the request body limit", func(t *testing.T) {
		largeBody := `{"data":"` + strings.Repeat("x", int(maxRequestBytes)+1) + `"}`
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(largeBody))
		w := httptest.NewRecorder()

		var data struct {
			Data string `json:"data"`
		}

		err := ReadJSON(w, r, &data)
		if err == nil {
			t.Error("expected error for body exceeding the request limit, got nil")
		}
	})

	t.Run("rejects multiple JSON values", func(t *testing.T) {
		body := strings.NewReader(`{"name":"first"}{"name":"second"}`)
		r := httptest.NewRequest(http.MethodPost, "/", body)
		w := httptest.NewRecorder()

		var data struct {
			Name string `json:"name"`
		}

		err := ReadJSON(w, r, &data)
		if err == nil {
			t.Fatal("expected error for multiple JSON values, got nil")
		}
		if err.Error() != "body must only contain a single JSON value" {
			t.Errorf("expected specific error message, got: %v", err)
		}
	})
}

func TestErrorJSON(t *testing.T) {
	t.Run("writes error with default status 500", func(t *testing.T) {
		w := httptest.NewRecorder()
		testErr := errors.New("something went wrong")

		err := ErrorJSON(w, testErr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
		}

		body := w.Body.String()
		if !strings.Contains(body, `"error":true`) {
			t.Errorf("expected error:true in body, got %s", body)
		}
		if !strings.Contains(body, `"message":"something went wrong"`) {
			t.Errorf("expected error message in body, got %s", body)
		}
	})

	t.Run("writes error with the given status code", func(t *testing.T) {
		w := httptest.NewRecorder()
		testErr := errors.New("not found")

		err := ErrorJSON(w, testErr, http.StatusNotFound)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
		}

		body := w.Body.String()
		if !strings.Contains(body, `"message":"not found"`) {
			t.Errorf("expected error message in body, got %s", body)
		}
	})
}

// BenchmarkWriteJSON compares the pooled writer with the json.Marshal and
// Write it replaced, on a body the size of a movie's details.
func BenchmarkWriteJSON(b *testing.B) {
	data := jsonTestCredits(600)

	b.Run("marshal", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			w.Body = nil
			out, _ := json.Marshal(data)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(out)
		}
	})

	b.Run("pooled", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			w.Body = nil
			_ = WriteJSON(w, http.StatusOK, data)
		}
	})

	b.Run("pooled gzip", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			w := &negotiatingRecorder{ResponseRecorder: httptest.NewRecorder(), accepts: true}
			w.Body = nil
			_ = WriteJSON(w, http.StatusOK, data)
		}
	})
}
