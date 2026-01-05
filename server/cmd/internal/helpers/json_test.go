package helpers

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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

	t.Run("returns error for unmarshalable data", func(t *testing.T) {
		w := httptest.NewRecorder()
		// Channels cannot be marshaled to JSON
		data := make(chan int)

		err := WriteJSON(w, http.StatusOK, data)
		if err == nil {
			t.Error("expected error for unmarshalable data, got nil")
		}
	})

	t.Run("writes JSONResponse struct correctly", func(t *testing.T) {
		w := httptest.NewRecorder()
		data := JSONResponse{
			Error:   false,
			Message: "success",
			Data:    map[string]int{"count": 42},
		}

		err := WriteJSON(w, http.StatusOK, data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		body := w.Body.String()
		if !strings.Contains(body, `"error":false`) {
			t.Errorf("expected error:false in body, got %s", body)
		}
		if !strings.Contains(body, `"message":"success"`) {
			t.Errorf("expected message in body, got %s", body)
		}
		if !strings.Contains(body, `"count":42`) {
			t.Errorf("expected data in body, got %s", body)
		}
	})

	t.Run("omits empty fields with omitempty", func(t *testing.T) {
		w := httptest.NewRecorder()
		data := JSONResponse{
			Error: true,
			// Message and Data are empty
		}

		err := WriteJSON(w, http.StatusOK, data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		body := w.Body.String()
		if strings.Contains(body, `"message"`) {
			t.Errorf("expected message to be omitted, got %s", body)
		}
		if strings.Contains(body, `"data"`) {
			t.Errorf("expected data to be omitted, got %s", body)
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

		err := ReadJSON(w, r, &data, 0)
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

		err := ReadJSON(w, r, &data, 0)
		if err == nil {
			t.Error("expected error for unknown field, got nil")
		}
	})

	t.Run("enforces max bytes limit", func(t *testing.T) {
		// Create a body larger than the limit
		largeBody := `{"data":"` + strings.Repeat("x", 100) + `"}`
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(largeBody))
		w := httptest.NewRecorder()

		var data struct {
			Data string `json:"data"`
		}

		// Set a small limit
		err := ReadJSON(w, r, &data, 50)
		if err == nil {
			t.Error("expected error for body exceeding max bytes, got nil")
		}
	})

	t.Run("rejects multiple JSON values", func(t *testing.T) {
		body := strings.NewReader(`{"name":"first"}{"name":"second"}`)
		r := httptest.NewRequest(http.MethodPost, "/", body)
		w := httptest.NewRecorder()

		var data struct {
			Name string `json:"name"`
		}

		err := ReadJSON(w, r, &data, 0)
		if err == nil {
			t.Error("expected error for multiple JSON values, got nil")
		}
		if err != nil && err.Error() != "body must only contain a single JSON value" {
			t.Errorf("expected specific error message, got: %v", err)
		}
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		body := strings.NewReader(`{invalid json}`)
		r := httptest.NewRequest(http.MethodPost, "/", body)
		w := httptest.NewRecorder()

		var data struct {
			Name string `json:"name"`
		}

		err := ReadJSON(w, r, &data, 0)
		if err == nil {
			t.Error("expected error for invalid JSON, got nil")
		}
	})

	t.Run("uses default max bytes when zero", func(t *testing.T) {
		body := strings.NewReader(`{"name":"test"}`)
		r := httptest.NewRequest(http.MethodPost, "/", body)
		w := httptest.NewRecorder()

		var data struct {
			Name string `json:"name"`
		}

		// Pass 0 for maxBytes, should use 1MB default
		err := ReadJSON(w, r, &data, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if data.Name != "test" {
			t.Errorf("expected name 'test', got '%s'", data.Name)
		}
	})

	t.Run("handles empty body", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
		w := httptest.NewRecorder()

		var data struct {
			Name string `json:"name"`
		}

		err := ReadJSON(w, r, &data, 0)
		if err == nil {
			t.Error("expected error for empty body, got nil")
		}
		if !errors.Is(err, io.EOF) {
			t.Errorf("expected EOF error, got: %v", err)
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

	t.Run("writes error with custom status code", func(t *testing.T) {
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

	t.Run("writes error with bad request status", func(t *testing.T) {
		w := httptest.NewRecorder()
		testErr := errors.New("invalid input")

		err := ErrorJSON(w, testErr, http.StatusBadRequest)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
		}
	})

	t.Run("sets correct content type", func(t *testing.T) {
		w := httptest.NewRecorder()
		testErr := errors.New("error")

		err := ErrorJSON(w, testErr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		contentType := w.Header().Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", contentType)
		}
	})
}
