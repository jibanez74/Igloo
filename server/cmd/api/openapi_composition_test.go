package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// A schema with "additionalProperties": false cannot be used as an allOf branch.
// JSON Schema evaluates every branch against the whole object, so a closed branch
// rejects each property its siblings contribute and the composed schema can never
// match a real response. This is invisible to the handler contract tests whenever
// the operation is only exercised against an empty collection, so it is checked
// here against the document itself.
func TestOpenAPISchemasDoNotComposeOverClosedBranches(t *testing.T) {
	document := loadRawOpenAPIDocument(t)

	schemas, ok := document["components"].(map[string]any)["schemas"].(map[string]any)
	if !ok {
		t.Fatal("OpenAPI document has no components.schemas object")
	}

	resolve := func(node any) map[string]any {
		object, ok := node.(map[string]any)
		if !ok {
			return nil
		}
		ref, isRef := object["$ref"].(string)
		if !isRef {
			return object
		}
		name := strings.TrimPrefix(ref, "#/components/schemas/")
		target, _ := schemas[name].(map[string]any)
		return target
	}

	var violations []string
	var walk func(schemaName, path string, node any)
	walk = func(schemaName, path string, node any) {
		switch typed := node.(type) {
		case map[string]any:
			branches, hasAllOf := typed["allOf"].([]any)
			if hasAllOf {
				for index, branch := range branches {
					resolved := resolve(branch)
					if resolved == nil || resolved["additionalProperties"] != false {
						continue
					}
					known := propertyNames(resolved)
					var rejected []string
					for siblingIndex, sibling := range branches {
						if siblingIndex == index {
							continue
						}
						for name := range propertyNames(resolve(sibling)) {
							if !known[name] {
								rejected = append(rejected, name)
							}
						}
					}
					if len(rejected) > 0 {
						sort.Strings(rejected)
						label, _ := branch.(map[string]any)["$ref"].(string)
						if label == "" {
							label = fmt.Sprintf("allOf[%d]", index)
						}
						violations = append(violations, fmt.Sprintf(
							"%s%s: closed branch %s rejects %v contributed by sibling branches",
							schemaName, path, label, rejected,
						))
					}
				}
			}
			for key, value := range typed {
				walk(schemaName, path+"/"+key, value)
			}
		case []any:
			for index, value := range typed {
				walk(schemaName, fmt.Sprintf("%s/%d", path, index), value)
			}
		}
	}

	for name, schema := range schemas {
		walk(name, "", schema)
	}

	if len(violations) > 0 {
		sort.Strings(violations)
		t.Fatalf("docs/openapi.json composes allOf over closed schemas, so these can never match a response:\n  %s",
			strings.Join(violations, "\n  "))
	}
}

// propertyNames returns the property keys a schema declares directly, as a set.
func propertyNames(schema map[string]any) map[string]bool {
	names := map[string]bool{}
	if schema == nil {
		return names
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		return names
	}
	for name := range properties {
		names[name] = true
	}
	return names
}

func loadRawOpenAPIDocument(t *testing.T) map[string]any {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to locate test file")
	}

	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
	raw, err := os.ReadFile(filepath.Join(repoRoot, "docs", "openapi.json"))
	if err != nil {
		t.Fatalf("read OpenAPI document: %v", err)
	}

	var document map[string]any
	err = json.Unmarshal(raw, &document)
	if err != nil {
		t.Fatalf("parse OpenAPI document: %v", err)
	}
	return document
}
