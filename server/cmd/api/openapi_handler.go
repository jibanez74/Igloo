package main

import (
	_ "embed"
	"net/http"
	"strconv"
	"sync"
)

// openAPIDocumentJSON is the copy of docs/openapi.json that make
// prepare-openapi-embed places in this package before every build.
//
//go:embed openapi/openapi.json
var openAPIDocumentJSON []byte

// openAPIDocumentGzip compresses the embedded document on first use. The route
// sits outside the session group, so helpers.WriteBody's per-request gzip never
// sees it; the document never changes, so one compressed copy serves every
// request. A compression error leaves it nil and the document goes out plain.
var openAPIDocumentGzip = sync.OnceValue(func() []byte {
	compressed, err := gzipFrontendAsset(openAPIDocumentJSON)
	if err != nil {
		return nil
	}
	return compressed
})

// ServeOpenAPIDocument answers with the embedded contract, the way FastAPI
// serves /openapi.json, gzip-compressed when the client accepts it.
func (app *Application) ServeOpenAPIDocument(w http.ResponseWriter, r *http.Request) {
	header := w.Header()
	header.Set("Content-Type", "application/json")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Cache-Control", "no-cache")
	header.Add("Vary", "Accept-Encoding")

	body := openAPIDocumentJSON
	if acceptsGzip(r) {
		compressed := openAPIDocumentGzip()
		if compressed != nil {
			body = compressed
			header.Set("Content-Encoding", "gzip")
		}
	}

	header.Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusOK)
	w.Write(body)
}
