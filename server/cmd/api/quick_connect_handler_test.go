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
	Error bool `json:"error"`
	Data  struct {
		Code                string `json:"code"`
		Secret              string `json:"secret"`
		ExpiresInSeconds    int    `json:"expires_in_seconds"`
		PollIntervalSeconds int    `json:"poll_interval_seconds"`
	} `json:"data"`
}

type quickConnectRedeemResponse struct {
	Error   bool   `json:"error"`
	Message string `json:"message"`
	Data    struct {
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
	req := httptest.NewRequest(http.MethodPost, "/api/quick-connect/approve", strings.NewReader(body))
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

	// Codes are entered by hand, so approval must tolerate lowercase input.
	w = approveQuickConnectForTest(t, app, strings.ToLower(code), newAuthSessionCookie(t, app, user.ID))
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

func TestQuickConnect_ExpiredCodeCannotBeApprovedOrRedeemed(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
	app.InitRouter()

	user := createTestUser(t, app, "Approver", "approver@example.com", false)
	code, secret := initiateQuickConnectForTest(t, app)

	// Age the entry past its TTL from the broker's perspective.
	app.QuickConnect.now = func() time.Time { return time.Now().Add(quickConnectCodeTTL + time.Second) }

	w := approveQuickConnectForTest(t, app, code, newAuthSessionCookie(t, app, user.ID))
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
