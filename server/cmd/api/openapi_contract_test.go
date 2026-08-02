package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"
)

// newOpenAPIJSONRequest builds a request that a handler and then
// assertOpenAPIExchange can both read. Serving the request drains its body, so
// the assertion replays it through GetBody. Real clients always send the
// content type the contract documents, so set it here too.
func newOpenAPIJSONRequest(method, target, body string) *http.Request {
	return newOpenAPIRequest(method, target, "application/json", []byte(body))
}

func newOpenAPIRequest(method, target, contentType string, body []byte) *http.Request {
	request := httptest.NewRequest(method, target, bytes.NewReader(body))
	request.Header.Set("Content-Type", contentType)
	request.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
	return request
}

func addOpenAPITestCookie(request *http.Request) {
	request.AddCookie(&http.Cookie{Name: "session", Value: "openapi-contract"})
}

// assertOpenAPIExchange validates the observable HTTP boundary rather than a
// handler implementation detail. Call it from endpoint tests after the real
// request has been served. Requests carrying a body must come from
// newOpenAPIJSONRequest so the consumed body can be replayed.
func assertOpenAPIExchange(t *testing.T, operationID string, request *http.Request, response *httptest.ResponseRecorder) {
	t.Helper()

	_, router := loadOpenAPIContract(t)

	route, pathParams, err := router.FindRoute(request)
	if err != nil {
		t.Fatalf("find OpenAPI route for %s %s: %v", request.Method, request.URL.Path, err)
	}
	if route.Operation.OperationID != operationID {
		t.Fatalf("operation ID = %q, want %q", route.Operation.OperationID, operationID)
	}

	if route.Operation.RequestBody != nil && request.GetBody == nil {
		t.Fatalf("operation %s sends a request body; build the request with newOpenAPIJSONRequest so it can be replayed", operationID)
	}
	if request.GetBody != nil {
		replayed, replayErr := request.GetBody()
		if replayErr != nil {
			t.Fatalf("replay request body for %s: %v", operationID, replayErr)
		}
		request.Body = replayed
	}

	requestInput := &openapi3filter.RequestValidationInput{
		Request:    request,
		PathParams: pathParams,
		Route:      route,
		Options:    openAPIValidationOptions,
	}
	err = openapi3filter.ValidateRequest(context.Background(), requestInput)
	if err != nil {
		t.Fatalf("OpenAPI request validation: %v", err)
	}

	result := response.Result()
	responseInput := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: requestInput,
		Status:                 result.StatusCode,
		Header:                 result.Header,
		Body:                   io.NopCloser(bytes.NewBuffer(response.Body.Bytes())),
	}
	err = openapi3filter.ValidateResponse(context.Background(), responseInput)
	if err != nil {
		t.Fatalf("OpenAPI response validation: %v", err)
	}

	validatedOpenAPIExchanges.record(operationID, route.Operation, result.StatusCode)
}

type openAPIExchangeRecorder struct {
	mu         sync.Mutex
	operations map[string]struct{}
}

func (recorder *openAPIExchangeRecorder) record(operationID string, operation *openapi3.Operation, status int) {
	if !operationResponseReturnsJSON(operation, status) {
		return
	}

	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	if recorder.operations == nil {
		recorder.operations = make(map[string]struct{})
	}
	recorder.operations[operationID] = struct{}{}
}

func (recorder *openAPIExchangeRecorder) snapshot() map[string]struct{} {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()

	operations := make(map[string]struct{}, len(recorder.operations))
	for operationID := range recorder.operations {
		operations[operationID] = struct{}{}
	}
	return operations
}

var validatedOpenAPIExchanges openAPIExchangeRecorder

// The contract document is large, so parse, validate, and route-index it once
// for the whole package rather than per assertion.
var (
	openAPIContractOnce   sync.Once
	openAPIContractDoc    *openapi3.T
	openAPIContractRouter routers.Router
	openAPIContractErr    error
)

// openAPIValidationOptions checks that a request documented as authenticated
// actually carries the credential its security scheme names. Whether that
// credential grants access is the middleware's concern and is covered by the
// handler tests; this helper only validates the documented HTTP boundary.
var openAPIValidationOptions = &openapi3filter.Options{
	AuthenticationFunc: assertOpenAPICredentialPresent,
}

func assertOpenAPICredentialPresent(_ context.Context, input *openapi3filter.AuthenticationInput) error {
	scheme := input.SecurityScheme

	isCookieScheme := scheme.Type == "apiKey" && scheme.In == "cookie"
	if isCookieScheme {
		_, err := input.RequestValidationInput.Request.Cookie(scheme.Name)
		if err != nil {
			return fmt.Errorf("security scheme %q requires cookie %q: %w", input.SecuritySchemeName, scheme.Name, err)
		}
		return nil
	}

	isBearerScheme := scheme.Type == "http" && strings.EqualFold(scheme.Scheme, "bearer")
	if isBearerScheme {
		header := input.RequestValidationInput.Request.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			return fmt.Errorf("security scheme %q requires a Bearer Authorization header", input.SecuritySchemeName)
		}
		return nil
	}

	return fmt.Errorf("unsupported security scheme %q", input.SecuritySchemeName)
}

func loadOpenAPIContract(t *testing.T) (*openapi3.T, routers.Router) {
	t.Helper()
	openAPIContractOnce.Do(loadOpenAPIContractOnce)
	if openAPIContractErr != nil {
		t.Fatalf("load OpenAPI contract: %v", openAPIContractErr)
	}
	return openAPIContractDoc, openAPIContractRouter
}

func loadOpenAPIContractOnce() {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		openAPIContractErr = errors.New("failed to locate OpenAPI contract test")
		return
	}

	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
	documentPath := filepath.Join(repoRoot, "docs", "openapi.json")
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = false
	document, err := loader.LoadFromFile(documentPath)
	if err != nil {
		openAPIContractErr = fmt.Errorf("load OpenAPI document: %w", err)
		return
	}

	err = document.Validate(context.Background())
	if err != nil {
		openAPIContractErr = fmt.Errorf("validate OpenAPI document: %w", err)
		return
	}

	router, err := legacy.NewRouter(document)
	if err != nil {
		openAPIContractErr = fmt.Errorf("create OpenAPI router: %w", err)
		return
	}

	openAPIContractDoc = document
	openAPIContractRouter = router
}

func TestHealthCheckConformsToOpenAPI(t *testing.T) {
	app := setupTestApp(t)
	t.Cleanup(func() { _ = app.DB.Close() })
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	response := httptest.NewRecorder()

	app.HealthCheck(response, request)

	assertOpenAPIExchange(t, "healthCheck", request, response)
}

func TestMain(m *testing.M) {
	code := m.Run()
	if code == 0 && openAPITestRunIsUnfiltered() {
		openAPIContractOnce.Do(loadOpenAPIContractOnce)
		if openAPIContractErr != nil {
			fmt.Fprintf(os.Stderr, "load OpenAPI contract for exchange coverage: %v\\n", openAPIContractErr)
			code = 1
		} else {
			observed := validatedOpenAPIExchanges.snapshot()
			missing := missingOpenAPIJSONOperations(openAPIContractDoc, observed)
			if len(missing) > 0 {
				fmt.Fprintf(os.Stderr, "JSON OpenAPI operations without a validated successful handler exchange: %s\\n", strings.Join(missing, ", "))
				code = 1
			}
		}
	}
	os.Exit(code)
}

func openAPITestRunIsUnfiltered() bool {
	for _, name := range []string{"test.run", "test.skip", "test.list"} {
		testFlag := flag.Lookup(name)
		if testFlag != nil && testFlag.Value.String() != "" {
			return false
		}
	}
	return true
}

func missingOpenAPIJSONOperations(document *openapi3.T, observed map[string]struct{}) []string {
	missing := make([]string, 0)
	for _, pathItem := range document.Paths.Map() {
		for _, operation := range pathItem.Operations() {
			if !operationReturnsJSON(operation) {
				continue
			}
			if _, ok := observed[operation.OperationID]; !ok {
				missing = append(missing, operation.OperationID)
			}
		}
	}
	sort.Strings(missing)
	return missing
}

func TestOpenAPIExchangeCoverageComparison(t *testing.T) {
	jsonResponse := &openapi3.ResponseRef{Value: &openapi3.Response{
		Content: openapi3.Content{"application/json": &openapi3.MediaType{}},
	}}
	binaryResponse := &openapi3.ResponseRef{Value: &openapi3.Response{
		Content: openapi3.Content{"application/octet-stream": &openapi3.MediaType{}},
	}}

	document := &openapi3.T{Paths: openapi3.NewPaths()}
	successfulOperation := &openapi3.Operation{
		OperationID: "successfulOperation",
		Responses:   openapi3.NewResponses(openapi3.WithStatus(http.StatusOK, jsonResponse)),
	}
	errorOnlyOperation := &openapi3.Operation{
		OperationID: "errorOnlyOperation",
		Responses: openapi3.NewResponses(
			openapi3.WithStatus(http.StatusOK, jsonResponse),
			openapi3.WithStatus(http.StatusBadRequest, jsonResponse),
		),
	}
	missingOperation := &openapi3.Operation{
		OperationID: "missingOperation",
		Responses:   openapi3.NewResponses(openapi3.WithStatus(http.StatusCreated, jsonResponse)),
	}
	nonJSONOperation := &openapi3.Operation{
		OperationID: "nonJSONOperation",
		Responses:   openapi3.NewResponses(openapi3.WithStatus(http.StatusOK, binaryResponse)),
	}
	document.Paths.Set("/successful", &openapi3.PathItem{Get: successfulOperation})
	document.Paths.Set("/error-only", &openapi3.PathItem{Get: errorOnlyOperation})
	document.Paths.Set("/missing", &openapi3.PathItem{Post: missingOperation})
	document.Paths.Set("/binary", &openapi3.PathItem{Get: nonJSONOperation})

	var recorder openAPIExchangeRecorder
	recorder.record("successfulOperation", successfulOperation, http.StatusOK)
	recorder.record("errorOnlyOperation", errorOnlyOperation, http.StatusBadRequest)
	recorder.record("nonJSONOperation", nonJSONOperation, http.StatusOK)

	missing := missingOpenAPIJSONOperations(document, recorder.snapshot())
	want := []string{"errorOnlyOperation", "missingOperation"}
	if len(missing) != len(want) {
		t.Fatalf("missing operations = %v, want %v", missing, want)
	}
	for i := range want {
		if missing[i] != want[i] {
			t.Fatalf("missing operations = %v, want %v", missing, want)
		}
	}
}

func operationReturnsJSON(operation *openapi3.Operation) bool {
	for status, response := range operation.Responses.Map() {
		isSuccess := strings.HasPrefix(status, "2")
		if isSuccess && response.Value != nil && response.Value.Content["application/json"] != nil {
			return true
		}
	}
	return false
}

func operationResponseReturnsJSON(operation *openapi3.Operation, status int) bool {
	isSuccess := status >= http.StatusOK && status < http.StatusMultipleChoices
	if !isSuccess {
		return false
	}

	response := operation.Responses.Status(status)
	return response != nil && response.Value != nil && response.Value.Content["application/json"] != nil
}

func TestOpenAPIContractFileExists(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to locate contract test")
	}
	path := filepath.Join(filepath.Dir(currentFile), "..", "..", "..", "docs", "openapi.json")
	_, err := os.Stat(path)
	if err != nil {
		t.Fatalf("OpenAPI document: %v", err)
	}
}
