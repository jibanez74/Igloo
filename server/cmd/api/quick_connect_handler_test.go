package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type quickConnectInitiateResponse struct {
	Data struct {
		Code                string `json:"code"`
		Secret              string `json:"secret"`
		PollIntervalSeconds int    `json:"poll_interval_seconds"`
	} `json:"data"`
}

type quickConnectRedeemResponse struct {
	Data struct {
		Status string `json:"status"`
		Token  string `json:"token"`
		Device struct {
			ID       int64  `json:"id"`
			Name     string `json:"name"`
			Platform string `json:"platform"`
		} `json:"device"`
	} `json:"data"`
}

func initiateQuickConnectForTest(t *testing.T, app *Application) (string, string) {
	t.Helper()

	body := `{"device_name":"Living Room TV","platform":"android_tv","app_version":"1.0.0"}`
	req := newOpenAPIJSONRequest(http.MethodPost, "/api/quick-connect/initiate", body)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("initiate status = %d, want 201, body = %s", w.Code, w.Body.String())
	}
	assertOpenAPIExchange(t, "initiateQuickConnect", req, w)

	var resp quickConnectInitiateResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("decode initiate response: %v\nbody=%s", err, w.Body.String())
	}

	if len(resp.Data.Code) != quickConnectCodeLength {
		t.Fatalf("code = %q, want %d characters", resp.Data.Code, quickConnectCodeLength)
	}
	if resp.Data.Secret == "" {
		t.Fatal("expected a device secret")
	}
	if resp.Data.PollIntervalSeconds != quickConnectPollSeconds {
		t.Fatalf("poll interval = %d, want %d", resp.Data.PollIntervalSeconds, quickConnectPollSeconds)
	}

	return resp.Data.Code, resp.Data.Secret
}

func approveQuickConnectForTest(t *testing.T, app *Application, code string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	body := fmt.Sprintf(`{"code":%q}`, code)
	req := newOpenAPIJSONRequest(http.MethodPost, "/api/quick-connect/approve", body)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	assertOpenAPIExchange(t, "approveQuickConnect", req, w)
	return w
}

type quickConnectLookupResponse struct {
	Data struct {
		DeviceName string  `json:"device_name"`
		Platform   string  `json:"platform"`
		AppVersion *string `json:"app_version"`
	} `json:"data"`
}

func lookupQuickConnectForTest(t *testing.T, app *Application, code string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	body := fmt.Sprintf(`{"code":%q}`, code)
	req := newOpenAPIJSONRequest(http.MethodPost, "/api/quick-connect/lookup", body)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	if w.Code == http.StatusOK {
		assertOpenAPIExchange(t, "lookupQuickConnect", req, w)
	}
	return w
}

func redeemQuickConnectForTest(t *testing.T, app *Application, code, secret string) *httptest.ResponseRecorder {
	t.Helper()

	body := fmt.Sprintf(`{"code":%q,"secret":%q}`, code, secret)
	req := newOpenAPIJSONRequest(http.MethodPost, "/api/quick-connect/redeem", body)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	if w.Code == http.StatusOK {
		assertOpenAPIExchange(t, "redeemQuickConnect", req, w)
	}
	return w
}

func TestQuickConnect_FullPairingFlow(t *testing.T) {
	app := setupSessionTestApp(t)
	app.InitRouter()

	user := createTestUser(t, app, "Approver", "approver@example.com", false)
	code, secret := initiateQuickConnectForTest(t, app)

	// Before approval the device sees a pending status.
	w := redeemQuickConnectForTest(t, app, code, secret)
	if w.Code != http.StatusOK {
		t.Fatalf("pending redeem status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	var pending quickConnectRedeemResponse
	err := json.Unmarshal(w.Body.Bytes(), &pending)
	if err != nil {
		t.Fatalf("decode pending response: %v", err)
	}
	if pending.Data.Status != "pending" {
		t.Fatalf("status = %q, want pending", pending.Data.Status)
	}

	// The approving user first looks the code up to see which device it is.
	cookie := newAuthSessionCookie(t, app, user.ID)
	w = lookupQuickConnectForTest(t, app, strings.ToLower(code), cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("lookup status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	var lookup quickConnectLookupResponse
	err = json.Unmarshal(w.Body.Bytes(), &lookup)
	if err != nil {
		t.Fatalf("decode lookup response: %v", err)
	}
	if lookup.Data.DeviceName != "Living Room TV" || lookup.Data.Platform != "android_tv" {
		t.Fatalf("lookup device = %+v, want the initiating device", lookup.Data)
	}
	if lookup.Data.AppVersion == nil || *lookup.Data.AppVersion != "1.0.0" {
		t.Fatalf("lookup app_version = %v, want 1.0.0", lookup.Data.AppVersion)
	}

	// Codes are entered by hand, so approval must tolerate lowercase input.
	w = approveQuickConnectForTest(t, app, strings.ToLower(code), cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("approve status = %d, want 200, body = %s", w.Code, w.Body.String())
	}

	w = redeemQuickConnectForTest(t, app, code, secret)
	if w.Code != http.StatusOK {
		t.Fatalf("approved redeem status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	var approved quickConnectRedeemResponse
	err = json.Unmarshal(w.Body.Bytes(), &approved)
	if err != nil {
		t.Fatalf("decode approved response: %v", err)
	}
	if approved.Data.Status != "approved" {
		t.Fatalf("status = %q, want approved", approved.Data.Status)
	}
	if !strings.HasPrefix(approved.Data.Token, deviceTokenPrefix) {
		t.Fatalf("token = %q, want %q prefix", approved.Data.Token, deviceTokenPrefix)
	}
	if approved.Data.Device.Name != "Living Room TV" {
		t.Fatalf("device = %+v, want the initiating device", approved.Data.Device)
	}

	// The token authenticates API requests for the approving user.
	req := httptest.NewRequest(http.MethodGet, "/api/auth/user", nil)
	req.Header.Set("Authorization", "Bearer "+approved.Data.Token)
	w = httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("bearer auth/user status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	authResp := decodeAuthUserResponse(t, w)
	if authResp.Data.User.ID != user.ID {
		t.Fatalf("bearer user id = %d, want %d", authResp.Data.User.ID, user.ID)
	}

	// The code is consumed: a second redeem cannot mint another token.
	w = redeemQuickConnectForTest(t, app, code, secret)
	if w.Code != http.StatusNotFound {
		t.Fatalf("second redeem status = %d, want 404", w.Code)
	}
}

func TestQuickConnect_RedeemRejectsWrongSecret(t *testing.T) {
	app := setupSessionTestApp(t)
	app.InitRouter()

	user := createTestUser(t, app, "Approver", "approver@example.com", false)
	code, _ := initiateQuickConnectForTest(t, app)

	w := approveQuickConnectForTest(t, app, code, newAuthSessionCookie(t, app, user.ID))
	if w.Code != http.StatusOK {
		t.Fatalf("approve status = %d, want 200", w.Code)
	}

	w = redeemQuickConnectForTest(t, app, code, "wrong-secret")
	if w.Code != http.StatusNotFound {
		t.Fatalf("redeem with wrong secret status = %d, want 404, body = %s", w.Code, w.Body.String())
	}
}

func TestQuickConnect_ApproveUnknownCodeReturnsNotFound(t *testing.T) {
	app := setupSessionTestApp(t)
	app.InitRouter()

	user := createTestUser(t, app, "Approver", "approver@example.com", false)

	w := approveQuickConnectForTest(t, app, "ZZZZZZ", newAuthSessionCookie(t, app, user.ID))
	if w.Code != http.StatusNotFound {
		t.Fatalf("approve unknown code status = %d, want 404, body = %s", w.Code, w.Body.String())
	}
}

func TestQuickConnect_ApproveRejectsDeviceTokenAuth(t *testing.T) {
	app := setupSessionTestApp(t)
	app.InitRouter()

	user := createTestUser(t, app, "Approver", "approver@example.com", false)
	token := createTestDevice(t, app, user.ID, "Phone", "android")

	code, _ := initiateQuickConnectForTest(t, app)

	body := fmt.Sprintf(`{"code":%q}`, code)
	req := httptest.NewRequest(http.MethodPost, "/api/quick-connect/approve", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("bearer approve status = %d, want 401, body = %s", w.Code, w.Body.String())
	}
}

func TestQuickConnect_LookupDoesNotBindOrConsumeCode(t *testing.T) {
	app := setupSessionTestApp(t)
	app.InitRouter()

	user := createTestUser(t, app, "Approver", "approver@example.com", false)
	cookie := newAuthSessionCookie(t, app, user.ID)
	code, secret := initiateQuickConnectForTest(t, app)

	// Repeated lookups keep returning the pending device without side effects.
	for i := 0; i < 2; i++ {
		w := lookupQuickConnectForTest(t, app, code, cookie)
		if w.Code != http.StatusOK {
			t.Fatalf("lookup #%d status = %d, want 200, body = %s", i+1, w.Code, w.Body.String())
		}
	}

	// The device still polls as pending, and approval still works afterward.
	w := redeemQuickConnectForTest(t, app, code, secret)
	var pending quickConnectRedeemResponse
	err := json.Unmarshal(w.Body.Bytes(), &pending)
	if err != nil {
		t.Fatalf("decode pending response: %v", err)
	}
	if pending.Data.Status != "pending" {
		t.Fatalf("status after lookups = %q, want pending", pending.Data.Status)
	}

	w = approveQuickConnectForTest(t, app, code, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("approve after lookup status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
}

func TestQuickConnect_LookupUnknownCodeReturnsNotFound(t *testing.T) {
	app := setupSessionTestApp(t)
	app.InitRouter()

	user := createTestUser(t, app, "Approver", "approver@example.com", false)

	w := lookupQuickConnectForTest(t, app, "ZZZZZZ", newAuthSessionCookie(t, app, user.ID))
	if w.Code != http.StatusNotFound {
		t.Fatalf("lookup unknown code status = %d, want 404, body = %s", w.Code, w.Body.String())
	}
}

func TestQuickConnect_LookupApprovedCodeReturnsNotFound(t *testing.T) {
	app := setupSessionTestApp(t)
	app.InitRouter()

	user := createTestUser(t, app, "Approver", "approver@example.com", false)
	cookie := newAuthSessionCookie(t, app, user.ID)
	code, _ := initiateQuickConnectForTest(t, app)

	w := approveQuickConnectForTest(t, app, code, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("approve status = %d, want 200", w.Code)
	}

	// An already-approved code is indistinguishable from an unknown one.
	w = lookupQuickConnectForTest(t, app, code, cookie)
	if w.Code != http.StatusNotFound {
		t.Fatalf("lookup approved code status = %d, want 404, body = %s", w.Code, w.Body.String())
	}
}

func TestQuickConnect_LookupRejectsDeviceTokenAuth(t *testing.T) {
	app := setupSessionTestApp(t)
	app.InitRouter()

	user := createTestUser(t, app, "Approver", "approver@example.com", false)
	token := createTestDevice(t, app, user.ID, "Phone", "android")

	code, _ := initiateQuickConnectForTest(t, app)

	body := fmt.Sprintf(`{"code":%q}`, code)
	req := httptest.NewRequest(http.MethodPost, "/api/quick-connect/lookup", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("bearer lookup status = %d, want 401, body = %s", w.Code, w.Body.String())
	}
}

func TestQuickConnect_LookupSharesApproveRateLimit(t *testing.T) {
	app := setupSessionTestApp(t)
	app.InitRouter()

	user := createTestUser(t, app, "Approver", "approver@example.com", false)
	cookie := newAuthSessionCookie(t, app, user.ID)

	for i := 0; i < 10; i++ {
		w := lookupQuickConnectForTest(t, app, "AAAAAA", cookie)
		if w.Code != http.StatusNotFound {
			t.Fatalf("attempt %d status = %d, want 404", i+1, w.Code)
		}
	}

	// The bucket is shared, so both lookup and approve are now throttled.
	w := lookupQuickConnectForTest(t, app, "AAAAAA", cookie)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("11th lookup status = %d, want 429, body = %s", w.Code, w.Body.String())
	}
	w = approveQuickConnectForTest(t, app, "AAAAAA", cookie)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("approve after exhausted lookups status = %d, want 429, body = %s", w.Code, w.Body.String())
	}
}

func TestQuickConnect_ExpiredCodeCannotBeApprovedOrRedeemed(t *testing.T) {
	app := setupSessionTestApp(t)
	app.InitRouter()

	user := createTestUser(t, app, "Approver", "approver@example.com", false)
	code, secret := initiateQuickConnectForTest(t, app)

	// Age the entry past its TTL from the broker's perspective.
	app.QuickConnect.now = func() time.Time { return time.Now().Add(quickConnectCodeTTL + time.Second) }

	cookie := newAuthSessionCookie(t, app, user.ID)
	w := lookupQuickConnectForTest(t, app, code, cookie)
	if w.Code != http.StatusNotFound {
		t.Fatalf("lookup expired code status = %d, want 404", w.Code)
	}

	w = approveQuickConnectForTest(t, app, code, cookie)
	if w.Code != http.StatusNotFound {
		t.Fatalf("approve expired code status = %d, want 404", w.Code)
	}

	w = redeemQuickConnectForTest(t, app, code, secret)
	if w.Code != http.StatusNotFound {
		t.Fatalf("redeem expired code status = %d, want 404", w.Code)
	}
}

func TestQuickConnectBroker_PurgesExpiredEntries(t *testing.T) {
	broker := NewQuickConnectBroker()

	code, _, err := broker.Initiate("TV", "android_tv", "")
	if err != nil {
		t.Fatalf("initiate: %v", err)
	}

	broker.now = func() time.Time { return time.Now().Add(quickConnectCodeTTL + time.Second) }

	// Any operation triggers the lazy purge.
	if broker.Approve(code, 1) {
		t.Fatal("expected expired code to be unapprovable")
	}

	broker.mu.Lock()
	remaining := len(broker.entries)
	broker.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("entries = %d, want 0 after purge", remaining)
	}
}

func TestQuickConnectBroker_RedeemLeavesEntryUntilConsumed(t *testing.T) {
	broker := NewQuickConnectBroker()

	code, secret, err := broker.Initiate("TV", "android_tv", "")
	if err != nil {
		t.Fatalf("initiate: %v", err)
	}
	if !broker.Approve(code, 1) {
		t.Fatal("approve failed")
	}

	result := broker.Redeem(code, secret)
	if result.status != redeemApproved {
		t.Fatalf("redeem status = %d, want redeemApproved", result.status)
	}

	// The first redeem holds the claim while it issues the token, so a
	// concurrent redeem must not issue a second one.
	result = broker.Redeem(code, secret)
	if result.status != redeemPending {
		t.Fatalf("redeem while claimed status = %d, want redeemPending", result.status)
	}

	// A failed token issuance must leave the code redeemable on the next poll.
	broker.Release(code)
	result = broker.Redeem(code, secret)
	if result.status != redeemApproved {
		t.Fatalf("redeem after release status = %d, want redeemApproved", result.status)
	}

	broker.Consume(code)

	result = broker.Redeem(code, secret)
	if result.status != redeemNotFound {
		t.Fatalf("redeem after consume status = %d, want redeemNotFound", result.status)
	}
}

func TestQuickConnectBroker_RejectsWhenAtCapacity(t *testing.T) {
	broker := NewQuickConnectBroker()

	for i := 0; i < quickConnectMaxPendingCodes; i++ {
		_, _, err := broker.Initiate("TV", "android_tv", "")
		if err != nil {
			t.Fatalf("initiate #%d: %v", i+1, err)
		}
	}

	_, _, err := broker.Initiate("TV", "android_tv", "")
	if !errors.Is(err, errQuickConnectCapacityReached) {
		t.Fatalf("initiate over capacity err = %v, want errQuickConnectCapacityReached", err)
	}

	// Once entries expire, capacity frees up again.
	broker.now = func() time.Time { return time.Now().Add(quickConnectCodeTTL + time.Second) }
	_, _, err = broker.Initiate("TV", "android_tv", "")
	if err != nil {
		t.Fatalf("initiate after expiry: %v", err)
	}
}

func TestGenerateQuickConnectCode_UsesAlphabetAndLength(t *testing.T) {
	const samples = 2000

	seen := make(map[rune]bool, len(quickConnectCodeAlphabet))

	for i := 0; i < samples; i++ {
		code := generateQuickConnectCode()

		if len(code) != quickConnectCodeLength {
			t.Fatalf("code %q length = %d, want %d", code, len(code), quickConnectCodeLength)
		}

		for _, symbol := range code {
			if !strings.ContainsRune(quickConnectCodeAlphabet, symbol) {
				t.Fatalf("code %q contains %q, which is outside the alphabet", code, symbol)
			}

			seen[symbol] = true
		}
	}

	// Rejection sampling must still be able to reach every symbol; a sampling
	// bug that drops the tail of the alphabet shows up here.
	for _, symbol := range quickConnectCodeAlphabet {
		if !seen[symbol] {
			t.Fatalf("symbol %q never generated across %d codes", symbol, samples)
		}
	}
}

func TestNormalizeQuickConnectCode(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "lowercase", input: "xk4t7p", want: "XK4T7P"},
		{name: "surrounding whitespace", input: "  XK4T7P\n", want: "XK4T7P"},
		{name: "internal spaces", input: "XK4 T7P", want: "XK4T7P"},
		{name: "dashes", input: "xk4-t7p", want: "XK4T7P"},
		{name: "tabs", input: "XK4\tT7P", want: "XK4T7P"},
		// Only separators are dropped, so a mistyped code cannot collapse into
		// a valid one by losing characters the alphabet excludes.
		{name: "keeps excluded characters", input: "abc2340", want: "ABC2340"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := normalizeQuickConnectCode(test.input)
			if got != test.want {
				t.Fatalf("normalizeQuickConnectCode(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

// Two redeems of one approved code used to both pass the broker's check and
// each create a device before either consumed the code.
func TestQuickConnect_ConcurrentRedeemIssuesOneToken(t *testing.T) {
	app := setupSessionTestApp(t)
	app.InitRouter()

	user := createTestUser(t, app, "Approver", "concurrent-approver@example.com", false)
	code, secret := initiateQuickConnectForTest(t, app)
	w := approveQuickConnectForTest(t, app, code, newAuthSessionCookie(t, app, user.ID))
	if w.Code != http.StatusOK {
		t.Fatalf("approve status = %d, want 200: %s", w.Code, w.Body.String())
	}

	// The test pool holds a single connection, so taking it blocks the first
	// redeem between the broker's claim and its device insert.
	conn, err := app.DB.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	firstReq := newOpenAPIJSONRequest(http.MethodPost, "/api/quick-connect/redeem", fmt.Sprintf(`{"code":%q,"secret":%q}`, code, secret))
	first := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		app.Router.ServeHTTP(first, firstReq)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for {
		app.QuickConnect.mu.Lock()
		claimed := app.QuickConnect.entries[code].issuing
		app.QuickConnect.mu.Unlock()
		if claimed {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("first redeem never claimed the code")
		}
		time.Sleep(time.Millisecond)
	}

	second := redeemQuickConnectForTest(t, app, code, secret)
	if second.Code != http.StatusOK {
		t.Fatalf("concurrent redeem status = %d, want 200: %s", second.Code, second.Body.String())
	}
	var pending quickConnectRedeemResponse
	err = json.Unmarshal(second.Body.Bytes(), &pending)
	if err != nil {
		t.Fatal(err)
	}
	if pending.Data.Status != "pending" || pending.Data.Token != "" {
		t.Fatalf("concurrent redeem = %+v, want pending without a token", pending.Data)
	}

	err = conn.Close()
	if err != nil {
		t.Fatal(err)
	}
	<-done

	if first.Code != http.StatusOK {
		t.Fatalf("first redeem status = %d, want 200: %s", first.Code, first.Body.String())
	}
	assertOpenAPIExchange(t, "redeemQuickConnect", firstReq, first)
	var approved quickConnectRedeemResponse
	err = json.Unmarshal(first.Body.Bytes(), &approved)
	if err != nil {
		t.Fatal(err)
	}
	if approved.Data.Status != "approved" || !strings.HasPrefix(approved.Data.Token, deviceTokenPrefix) {
		t.Fatalf("first redeem = %+v, want approved with a token", approved.Data)
	}

	w = redeemQuickConnectForTest(t, app, code, secret)
	if w.Code != http.StatusNotFound {
		t.Fatalf("redeem after consume status = %d, want 404", w.Code)
	}

	devices, err := app.Queries.GetDevicesByUser(context.Background(), user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 1 {
		t.Fatalf("devices = %d, want exactly one from a single approval", len(devices))
	}
}

func TestQuickConnect_FailedIssuanceLeavesCodeRedeemable(t *testing.T) {
	app := setupSessionTestApp(t)
	app.InitRouter()

	user := createTestUser(t, app, "Approver", "retry-approver@example.com", false)
	code, secret := initiateQuickConnectForTest(t, app)
	w := approveQuickConnectForTest(t, app, code, newAuthSessionCookie(t, app, user.ID))
	if w.Code != http.StatusOK {
		t.Fatalf("approve status = %d, want 200: %s", w.Code, w.Body.String())
	}

	_, err := app.DB.Exec(`CREATE TRIGGER fail_device_insert BEFORE INSERT ON devices BEGIN SELECT RAISE(FAIL, 'device storage failure'); END`)
	if err != nil {
		t.Fatal(err)
	}

	w = redeemQuickConnectForTest(t, app, code, secret)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("failed issuance status = %d, want 500: %s", w.Code, w.Body.String())
	}

	_, err = app.DB.Exec(`DROP TRIGGER fail_device_insert`)
	if err != nil {
		t.Fatal(err)
	}

	// The claim was released, so the device's next poll succeeds.
	w = redeemQuickConnectForTest(t, app, code, secret)
	if w.Code != http.StatusOK {
		t.Fatalf("retry status = %d, want 200: %s", w.Code, w.Body.String())
	}
	var approved quickConnectRedeemResponse
	err = json.Unmarshal(w.Body.Bytes(), &approved)
	if err != nil {
		t.Fatal(err)
	}
	if approved.Data.Status != "approved" {
		t.Fatalf("retry status = %q, want approved", approved.Data.Status)
	}

	w = redeemQuickConnectForTest(t, app, code, secret)
	if w.Code != http.StatusNotFound {
		t.Fatalf("redeem after consume status = %d, want 404", w.Code)
	}

	devices, err := app.Queries.GetDevicesByUser(context.Background(), user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 1 {
		t.Fatalf("devices = %d, want one", len(devices))
	}
}
