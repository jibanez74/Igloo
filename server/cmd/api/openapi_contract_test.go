package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	"igloo/cmd/internal/helpers"

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
	assertOpenAPIHTTPExchange(t, operationID, request, response, true)
}

// Rejection and lenient-query tests deliberately send requests outside the
// request schema. They still validate the actual response against the contract.
func assertOpenAPIResponse(t *testing.T, operationID string, request *http.Request, response *httptest.ResponseRecorder) {
	t.Helper()
	assertOpenAPIHTTPExchange(t, operationID, request, response, false)
}

func assertOpenAPIHTTPExchange(t *testing.T, operationID string, request *http.Request, response *httptest.ResponseRecorder, validateRequest bool) {
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
	if validateRequest {
		err = openapi3filter.ValidateRequest(context.Background(), requestInput)
		if err != nil {
			t.Fatalf("OpenAPI request validation: %v", err)
		}
	}

	result := response.Result()
	responseInput := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: requestInput,
		Status:                 result.StatusCode,
		Header:                 result.Header,
		Body:                   io.NopCloser(bytes.NewBuffer(response.Body.Bytes())),
		Options:                openAPIValidationOptions,
	}
	err = validateOpenAPIResponse(responseInput)
	if err != nil {
		t.Fatalf("OpenAPI response validation: %v", err)
	}

	validatedOpenAPIExchanges.record(operationID, route.Operation, result.StatusCode)
}

func validateOpenAPIResponse(input *openapi3filter.ResponseValidationInput) error {
	// kin-openapi skips HEAD, 304, and some redirects before checking statuses.
	responses := input.RequestValidationInput.Route.Operation.Responses
	if responses.Status(input.Status) == nil && responses.Default() == nil {
		return &openapi3filter.ResponseError{Input: input, Reason: "status is not supported"}
	}
	return openapi3filter.ValidateResponse(context.Background(), input)
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
	AuthenticationFunc:    assertOpenAPICredentialPresent,
	IncludeResponseStatus: true,
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
	// kin-openapi does not register Apple's HLS playlist media type by
	// default. It is a textual response, so validate it with the same decoder
	// used for text/plain instead of skipping the response body.
	openapi3filter.RegisterBodyDecoder(hlsPlaylistContentType, openapi3filter.PlainBodyDecoder)

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

func TestHealthCheckDatabaseFailureConformsToOpenAPI(t *testing.T) {
	app := setupTestApp(t)
	err := app.DB.Close()
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	response := httptest.NewRecorder()
	app.HealthCheck(response, request)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", response.Code)
	}
	assertOpenAPIExchange(t, "healthCheck", request, response)
	var envelope helpers.JSONResponse
	err = json.Unmarshal(response.Body.Bytes(), &envelope)
	if err != nil {
		t.Fatal(err)
	}
	if envelope.Message != internalServerErrorMessage {
		t.Fatalf("unsafe health error: %s", response.Body.String())
	}
}

func TestOpenAPIResponseStatusValidation(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		status      int
		responseKey string
		wantError   bool
	}{
		{"documented success", http.MethodGet, 200, "200", false},
		{"undocumented error", http.MethodGet, 418, "200", true},
		{"undocumented HEAD", http.MethodHead, 500, "200", true},
		{"undocumented conditional", http.MethodGet, 304, "200", true},
		{"undocumented redirect", http.MethodGet, 307, "200", true},
		{"documented HEAD", http.MethodHead, 200, "200", false},
		{"documented conditional", http.MethodGet, 304, "304", false},
		{"status range", http.MethodGet, 201, "2XX", false},
		{"default response", http.MethodGet, 500, "default", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			responses := openapi3.NewResponses()
			responses.Set(tt.responseKey, &openapi3.ResponseRef{Value: &openapi3.Response{}})
			input := &openapi3filter.ResponseValidationInput{
				RequestValidationInput: &openapi3filter.RequestValidationInput{
					Request: httptest.NewRequest(tt.method, "/", nil),
					Route:   &routers.Route{Spec: &openapi3.T{OpenAPI: "3.1.0"}, Operation: &openapi3.Operation{Responses: responses}},
				},
				Status:  tt.status,
				Options: openAPIValidationOptions,
			}
			err := validateOpenAPIResponse(input)
			if (err != nil) != tt.wantError {
				t.Fatalf("response validation error = %v, want error = %v", err, tt.wantError)
			}
			if tt.wantError {
				var responseError *openapi3filter.ResponseError
				if !errors.As(err, &responseError) || responseError.Reason != "status is not supported" {
					t.Fatalf("expected unsupported-status error, got %v", err)
				}
			}
		})
	}
}

func TestStopPersonalHLSSession_AuthenticationDatabaseFailure(t *testing.T) {
	for _, credential := range []string{"bearer", "cookie"} {
		t.Run(credential, func(t *testing.T) {
			app := setupSessionTestApp(t)
			app.InitRouter()
			err := app.DB.Close()
			if err != nil {
				t.Fatalf("close authentication database: %v", err)
			}
			request := httptest.NewRequest(http.MethodPost, "/api/movies/5/hls/session/stop?playback_session="+testPlaybackSessionID, nil)
			if credential == "bearer" {
				request.Header.Set("Authorization", "Bearer igd_contract-database-failure")
			} else {
				addOpenAPITestCookie(request)
			}
			response := httptest.NewRecorder()

			app.Router.ServeHTTP(response, request)

			if response.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want 500: %s", response.Code, response.Body.String())
			}
			assertOpenAPIExchange(t, "stopPersonalHlsSession", request, response)
			var envelope helpers.JSONResponse
			err = json.Unmarshal(response.Body.Bytes(), &envelope)
			if err != nil {
				t.Fatalf("decode authentication error: %v", err)
			}
			if !envelope.Error || envelope.Message != internalServerErrorMessage || envelope.Data != nil {
				t.Fatalf("unexpected authentication error: %+v", envelope)
			}
		})
	}
}

func TestLoginSessionCommitFailureConformsToOpenAPI(t *testing.T) {
	app := setupSessionTestApp(t)
	t.Cleanup(func() { _ = app.DB.Close() })
	app.InitRouter()
	createTestUserWithPassword(t, app, "Contract User", "contract@example.com", "correct horse")
	_, err := app.DB.Exec(`CREATE TRIGGER fail_session_insert BEFORE INSERT ON sessions BEGIN SELECT RAISE(FAIL, 'private session storage failure'); END`)
	if err != nil {
		t.Fatalf("install session failure trigger: %v", err)
	}
	request := newOpenAPIJSONRequest(http.MethodPost, "/api/auth/login", `{"email":"contract@example.com","password":"correct horse"}`)
	response := httptest.NewRecorder()
	app.Router.ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500: %s", response.Code, response.Body.String())
	}
	assertOpenAPIExchange(t, "authenticateUser", request, response)
	var envelope helpers.JSONResponse
	err = json.Unmarshal(response.Body.Bytes(), &envelope)
	if err != nil {
		t.Fatalf("expected a single JSON error: %v; body: %s", err, response.Body.String())
	}
	if !envelope.Error || envelope.Message != internalServerErrorMessage || envelope.Data != nil {
		t.Fatalf("unexpected session error: %+v", envelope)
	}
	if response.Header().Get("Set-Cookie") != "" {
		t.Fatal("failed session commit must not issue a cookie")
	}
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

func TestOpenAPIHLSEnumsMatchServerConstants(t *testing.T) {
	document, _ := loadOpenAPIContract(t)

	tests := []struct {
		name   string
		schema string
		want   []any
	}{
		{
			name:   "profiles",
			schema: "HLSProfile",
			want: func() []any {
				values := make([]any, len(helpers.HLSAllowedProfiles))
				for i, profile := range helpers.HLSAllowedProfiles {
					values[i] = profile
				}
				return values
			}(),
		},
		{
			name:   "requested audio codecs",
			schema: "HLSRequestedAudioCodec",
			want: []any{
				string(helpers.HLSAudioCodecAC3),
				string(helpers.HLSAudioCodecEAC3),
			},
		},
		{
			name:   "effective audio codecs",
			schema: "HLSEffectiveAudioCodec",
			want: []any{
				string(helpers.HLSAudioCodecAAC),
				string(helpers.HLSAudioCodecAC3),
				string(helpers.HLSAudioCodecEAC3),
			},
		},
		{
			name:   "audio channel limits",
			schema: "HLSAudioChannelLimit",
			want: []any{
				float64(helpers.HLS_AUDIO_MAX_CHANNELS_STEREO),
				float64(helpers.HLS_AUDIO_MAX_CHANNELS_SURROUND),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := document.Components.Schemas[tt.schema]
			if schema == nil || schema.Value == nil {
				t.Fatalf("OpenAPI schema %q is missing", tt.schema)
			}
			if !reflect.DeepEqual(schema.Value.Enum, tt.want) {
				t.Fatalf("OpenAPI %s enum = %#v, want %#v", tt.schema, schema.Value.Enum, tt.want)
			}
		})
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
