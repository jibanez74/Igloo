package helpers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
