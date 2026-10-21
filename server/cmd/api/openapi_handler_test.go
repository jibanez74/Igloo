package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestServeOpenAPIDocument(t *testing.T) {
	app := setupTestApp(t)
	app.InitRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/openapi.json", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	assertOpenAPIExchange(t, "getOpenAPIDocument", req, w)

	if got := w.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	if w.Header().Get("Set-Cookie") != "" {
		t.Fatal("the document route must not touch the session")
	}

	// The embedded copy is made by make prepare-openapi-embed; a stale copy
	// would serve a contract the repository no longer matches.
	documentPath, err := openAPIDocumentPath()
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(documentPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(w.Body.Bytes(), want) {
		t.Fatal("served document differs from docs/openapi.json; run make prepare-openapi-embed")
	}
}
