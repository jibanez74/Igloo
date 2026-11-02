package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestServeOpenAPIDocument(t *testing.T) {
	app := setupSessionTestApp(t)
	app.InitRouter()

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
	for _, databaseAvailable := range []bool{true, false} {
		if !databaseAvailable {
			err = app.DB.Close()
			if err != nil {
				t.Fatal(err)
			}
		}
		for _, credentials := range []string{"none", "cookie", "bearer", "both", "malformed"} {
			t.Run(fmt.Sprintf("database=%t/%s", databaseAvailable, credentials), func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, "/api/openapi.json", nil)
				if credentials == "cookie" || credentials == "both" {
					req.AddCookie(&http.Cookie{Name: app.SessionManager.Cookie.Name, Value: "unknown-session"})
				}
				if credentials == "bearer" || credentials == "both" {
					req.Header.Set("Authorization", "Bearer igd_invalid")
				}
				if credentials == "malformed" {
					req.Header.Set("Authorization", "invalid")
					req.Header.Set("Cookie", "session=malformed!")
				}
				w := httptest.NewRecorder()
				app.Router.ServeHTTP(w, req)
				if w.Code != http.StatusOK {
					t.Fatalf("status = %d: %s", w.Code, w.Body)
				}
				assertOpenAPIExchange(t, "getOpenAPIDocument", req, w)
				for header, value := range map[string]string{
					"Content-Type": "application/json", "Cache-Control": "no-cache",
					"X-Content-Type-Options": "nosniff", "Set-Cookie": "", "Vary": "",
				} {
					if w.Header().Get(header) != value {
						t.Errorf("%s = %q, want %q", header, w.Header().Get(header), value)
					}
				}
				if !bytes.Equal(w.Body.Bytes(), want) {
					t.Fatal("served document differs from docs/openapi.json; run make prepare-openapi-embed")
				}
			})
		}
	}
}
