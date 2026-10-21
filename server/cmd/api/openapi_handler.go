package main

import (
	_ "embed"
	"net/http"
)

// openAPIDocumentJSON is the copy of docs/openapi.json that make
// prepare-openapi-embed places in this package before every build.
//
//go:embed openapi/openapi.json
var openAPIDocumentJSON []byte

// ServeOpenAPIDocument answers with the embedded contract, the way FastAPI
// serves /openapi.json.
func (app *Application) ServeOpenAPIDocument(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	w.Write(openAPIDocumentJSON)
}
