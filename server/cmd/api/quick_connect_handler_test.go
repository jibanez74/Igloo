package main

import (
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
			ID        int64  `json:"id"`
			Name      string `json:"name"`
			Platform  string `json:"platform"`
			IsCurrent bool   `json:"is_current"`
		} `json:"device"`
	} `json:"data"`
}

func initiateQuickConnectForTest(t *testing.T, app *Application) (string, string) {
	t.Helper()

	body := `{"device_name":"Living Room TV","platform":"android_tv","app_version":"1.0.0"}`
	req := httptest.NewRequest(http.MethodPost, "/api/quick-connect/initiate", strings.NewReader(body))
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("initiate status = %d, want 201, body = %s", w.Code, w.Body.String())
	}

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
	req := httptest.NewRequest(http.MethodPost, "/api/quick-connect/lookup", strings.NewReader(body))
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	return w
}

func redeemQuickConnectForTest(t *testing.T, app *Application, code, secret string) *httptest.ResponseRecorder {
	t.Helper()

	body := fmt.Sprintf(`{"code":%q,"secret":%q}`, code, secret)
	req := httptest.NewRequest(http.MethodPost, "/api/quick-connect/redeem", strings.NewReader(body))
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	return w
}

func TestQuickConnect_FullPairingFlow(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
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
	if approved.Data.Device.Name != "Living Room TV" || !approved.Data.Device.IsCurrent {
		t.Fatalf("device = %+v, want the initiating device marked current", approved.Data.Device)
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

// TestQuickConnect_DeviceLifecycle drives the entire device lifecycle through
// the real router in one uninterrupted sequence: pairing, using the token,
// managing the device from a session, and revoking it. It intentionally
// overlaps TestQuickConnect_FullPairingFlow on the pairing steps.
func TestQuickConnect_DeviceLifecycle(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
	app.InitRouter()

	user := createTestUser(t, app, "Approver", "approver@example.com", false)
	cookie := newAuthSessionCookie(t, app, user.ID)

	// 1. The device asks for a pairing code.
	code, secret := initiateQuickConnectForTest(t, app)

	// 2. Polling before approval reports pending.
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

	// 3. The user approves the code from their browser session.
	w = approveQuickConnectForTest(t, app, code, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("approve status = %d, want 200, body = %s", w.Code, w.Body.String())
	}

	// 4. The device's next poll receives its token.
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
	token := approved.Data.Token
	deviceID := approved.Data.Device.ID

	// 5. The token authenticates requests as the approving user.
	req := httptest.NewRequest(http.MethodGet, "/api/auth/user", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("bearer auth/user status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	authResp := decodeAuthUserResponse(t, w)
	if authResp.Data.User.ID != user.ID {
		t.Fatalf("bearer user id = %d, want %d", authResp.Data.User.ID, user.ID)
	}

	// 6. The user's session sees the new device in the list.
	req = httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	var list deviceListResponse
	err = json.Unmarshal(w.Body.Bytes(), &list)
	if err != nil {
		t.Fatalf("decode devices response: %v", err)
	}
	if len(list.Data.Devices) != 1 {
		t.Fatalf("devices = %d, want 1", len(list.Data.Devices))
	}
	if list.Data.Devices[0].ID != deviceID || list.Data.Devices[0].Name != "Living Room TV" {
		t.Fatalf("device = %+v, want id %d named %q", list.Data.Devices[0], deviceID, "Living Room TV")
	}
	if list.Data.Devices[0].IsCurrent {
		t.Fatal("is_current = true, want false in the session-only list")
	}

	// 7. The user revokes the device from their session.
	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/devices/%d", deviceID), nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("revoke status = %d, want 200, body = %s", w.Code, w.Body.String())
	}

	// 8. The device is gone from the list.
	req = httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list after revoke status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	list = deviceListResponse{}
	err = json.Unmarshal(w.Body.Bytes(), &list)
	if err != nil {
		t.Fatalf("decode devices response after revoke: %v", err)
	}
	if len(list.Data.Devices) != 0 {
		t.Fatalf("devices after revoke = %d, want 0", len(list.Data.Devices))
	}

	// 9. The revoked token no longer authenticates anything.
	req = httptest.NewRequest(http.MethodGet, "/api/auth/user", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("revoked token status = %d, want 401, body = %s", w.Code, w.Body.String())
	}
}

func TestQuickConnect_RedeemRejectsWrongSecret(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
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
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
	app.InitRouter()

	user := createTestUser(t, app, "Approver", "approver@example.com", false)

	w := approveQuickConnectForTest(t, app, "ZZZZZZ", newAuthSessionCookie(t, app, user.ID))
	if w.Code != http.StatusNotFound {
		t.Fatalf("approve unknown code status = %d, want 404, body = %s", w.Code, w.Body.String())
	}
}

func TestQuickConnect_ApproveRejectsDeviceTokenAuth(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
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
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
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
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
	app.InitRouter()

	user := createTestUser(t, app, "Approver", "approver@example.com", false)

	w := lookupQuickConnectForTest(t, app, "ZZZZZZ", newAuthSessionCookie(t, app, user.ID))
	if w.Code != http.StatusNotFound {
		t.Fatalf("lookup unknown code status = %d, want 404, body = %s", w.Code, w.Body.String())
	}
}

func TestQuickConnect_LookupApprovedCodeReturnsNotFound(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
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
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
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
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
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
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
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

func TestQuickConnect_ApproveIsRateLimited(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
	app.InitRouter()

	user := createTestUser(t, app, "Approver", "approver@example.com", false)
	cookie := newAuthSessionCookie(t, app, user.ID)

	for i := 0; i < 10; i++ {
		w := approveQuickConnectForTest(t, app, "AAAAAA", cookie)
		if w.Code != http.StatusNotFound {
			t.Fatalf("attempt %d status = %d, want 404", i+1, w.Code)
		}
	}

	w := approveQuickConnectForTest(t, app, "AAAAAA", cookie)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("11th attempt status = %d, want 429, body = %s", w.Code, w.Body.String())
	}
}

func TestQuickConnect_InitiateRequiresDeviceName(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
	app.InitRouter()

	req := httptest.NewRequest(http.MethodPost, "/api/quick-connect/initiate", strings.NewReader(`{"platform":"ios"}`))
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", w.Code, w.Body.String())
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

	// A failed token issuance must leave the code redeemable on the next poll.
	for i := 0; i < 2; i++ {
		result := broker.Redeem(code, secret)
		if result.status != redeemApproved {
			t.Fatalf("redeem #%d status = %d, want redeemApproved", i+1, result.status)
		}
	}

	broker.Consume(code)

	result := broker.Redeem(code, secret)
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
