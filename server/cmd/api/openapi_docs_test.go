package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

type openAPIDocument struct {
	Paths map[string]map[string]json.RawMessage `json:"paths"`
}

func TestOpenAPIDocumentsRegisteredAPIRoutes(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to locate test file")
	}

	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
	docPath := filepath.Join(repoRoot, "docs", "openapi.json")

	raw, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read OpenAPI document: %v", err)
	}

	var doc openAPIDocument
	err = json.Unmarshal(raw, &doc)
	if err != nil {
		t.Fatalf("parse OpenAPI document: %v", err)
	}

	documented := make(map[string]map[string]bool, len(doc.Paths))
	for rawPath, methods := range doc.Paths {
		path := normalizeRouteForOpenAPI(rawPath)
		if documented[path] == nil {
			documented[path] = make(map[string]bool, len(methods))
		}
		for method := range methods {
			if isOpenAPIHTTPMethod(method) {
				documented[path][strings.ToUpper(method)] = true
			}
		}
	}

	app := &Application{}
	app.InitRouter()

	registered := make(map[string]map[string]bool)
	var missing []string
	err = chi.Walk(app.Router, func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		if !strings.HasPrefix(route, "/api") {
			return nil
		}

		path := normalizeRouteForOpenAPI(route)
		if registered[path] == nil {
			registered[path] = make(map[string]bool)
		}
		registered[path][method] = true

		if !documented[path][method] {
			missing = append(missing, method+" "+path)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("walk router: %v", err)
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("docs/openapi.json is missing registered API routes:\n%s", strings.Join(missing, "\n"))
	}

	var stale []string
	for path, methods := range documented {
		if !strings.HasPrefix(path, "/api") {
			continue
		}

		for method := range methods {
			if !registered[path][method] {
				stale = append(stale, method+" "+path)
			}
		}
	}

	if len(stale) > 0 {
		sort.Strings(stale)
		t.Fatalf("docs/openapi.json documents API routes that are not registered:\n%s", strings.Join(stale, "\n"))
	}
}

func normalizeRouteForOpenAPI(route string) string {
	if route == "/api/static/*" {
		return "/api/static/{path}"
	}

	if strings.HasPrefix(route, "/api/") && route != "/api/" {
		return strings.TrimRight(route, "/")
	}

	return route
}

func isOpenAPIHTTPMethod(method string) bool {
	switch strings.ToLower(method) {
	case "get", "put", "post", "delete", "options", "head", "patch", "trace":
		return true
	default:
		return false
	}
}
