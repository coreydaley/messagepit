package twilio

import (
	"crypto/hmac"
	"crypto/sha1" // #nosec G505
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/coreydaley/messagepit/config"
	"github.com/coreydaley/messagepit/internal/logger"
	"github.com/coreydaley/messagepit/internal/storage"
	"github.com/gorilla/mux"
)

func setup() {
	logger.NoLogging = true
	config.TwilioAuthToken = ""
	if err := storage.InitDB(); err != nil {
		panic(err)
	}
	if err := storage.DeleteAllSMS(); err != nil {
		panic(err)
	}
}

func makeRequest(t *testing.T, accountSID string, form url.Values, authToken string) *httptest.ResponseRecorder {
	t.Helper()

	body := form.Encode()
	req := httptest.NewRequest(http.MethodPost,
		fmt.Sprintf("/2010-04-01/Accounts/%s/Messages.json", accountSID),
		strings.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if authToken != "" {
		req.SetBasicAuth(accountSID, authToken)
	}

	// populate ParseForm data
	if err := req.ParseForm(); err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	router := mux.NewRouter()
	router.HandleFunc("/2010-04-01/Accounts/{AccountSid}/Messages.json", CreateMessage).Methods("POST")
	router.ServeHTTP(rr, req)
	return rr
}

func TestCreateMessageSuccess(t *testing.T) {
	setup()
	defer storage.Close()

	form := url.Values{
		"From": {"+15550001111"},
		"To":   {"+15552223333"},
		"Body": {"Hello from test"},
	}

	rr := makeRequest(t, "ACtest123", form, "")

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	stats, _ := storage.GetSMSMailboxStats()
	if stats.Total != 1 {
		t.Fatalf("expected 1 SMS stored, got %d", stats.Total)
	}
}

func TestCreateMessageMissingFrom(t *testing.T) {
	setup()
	defer storage.Close()

	form := url.Values{
		"To":   {"+15552223333"},
		"Body": {"Hello"},
	}

	rr := makeRequest(t, "ACtest", form, "")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestCreateMessageMissingBody(t *testing.T) {
	setup()
	defer storage.Close()

	form := url.Values{
		"From": {"+15550001111"},
		"To":   {"+15552223333"},
	}

	rr := makeRequest(t, "ACtest", form, "")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestCreateMessageWithValidSignature(t *testing.T) {
	setup()
	defer storage.Close()

	const token = "test-auth-token-abc123"
	config.TwilioAuthToken = token
	defer func() { config.TwilioAuthToken = "" }()

	form := url.Values{
		"From": {"+15550001111"},
		"To":   {"+15552223333"},
		"Body": {"Signed message"},
	}

	rr := makeRequest(t, "ACtest", form, token)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 with valid signature, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCreateMessageWithInvalidSignature(t *testing.T) {
	setup()
	defer storage.Close()

	config.TwilioAuthToken = "real-token"
	defer func() { config.TwilioAuthToken = "" }()

	form := url.Values{
		"From": {"+15550001111"},
		"To":   {"+15552223333"},
		"Body": {"Unsigned message"},
	}

	rr := makeRequest(t, "ACtest", form, "wrong-token")
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 with invalid signature, got %d", rr.Code)
	}
}

func TestCreateMessageNoSignatureWhenTokenRequired(t *testing.T) {
	setup()
	defer storage.Close()

	config.TwilioAuthToken = "required-token"
	defer func() { config.TwilioAuthToken = "" }()

	form := url.Values{
		"From": {"+15550001111"},
		"To":   {"+15552223333"},
		"Body": {"No signature"},
	}

	rr := makeRequest(t, "ACtest", form, "")
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when signature missing, got %d", rr.Code)
	}
}

func TestValidSignatureFunction(t *testing.T) {
	const authToken = "test-token"
	const host = "example.com"
	const path = "/2010-04-01/Accounts/ACtest/Messages.json"
	// validSignature builds "http://" + r.Host + r.RequestURI; use a relative
	// URL in NewRequest so RequestURI is just the path, not the full URL.
	const fullURL = "http://" + host + path

	form := url.Values{
		"Body": {"Hello"},
		"From": {"+15550001111"},
		"To":   {"+15552223333"},
	}

	var sb strings.Builder
	sb.WriteString(fullURL)
	sb.WriteString("Body")
	sb.WriteString("Hello")
	sb.WriteString("From")
	sb.WriteString("+15550001111")
	sb.WriteString("To")
	sb.WriteString("+15552223333")

	mac := hmac.New(sha1.New, []byte(authToken)) // #nosec G401
	mac.Write([]byte(sb.String()))               // #nosec G104
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Host = host
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Twilio-Signature", sig)
	if err := req.ParseForm(); err != nil {
		t.Fatal(err)
	}

	if !validSignature(req, authToken) {
		t.Fatal("validSignature returned false for a correct signature")
	}

	if validSignature(req, "wrong-token") {
		t.Fatal("validSignature returned true for a wrong token")
	}
}

// callbackCollector serves a StatusCallback endpoint and returns a channel of
// received callback bodies in arrival order.
func callbackCollector(t *testing.T) (chan url.Values, string, func()) {
	t.Helper()

	received := make(chan url.Values, 16)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("callback body is not form-encoded: %v", err)
			return
		}
		received <- r.PostForm
		w.WriteHeader(http.StatusNoContent)
	}))

	prevURL, prevDelay := config.TwilioWebhookURL, config.TwilioCallbackDelay
	config.TwilioWebhookURL = srv.URL
	config.TwilioCallbackDelay = 0

	return received, srv.URL, func() {
		srv.Close()
		config.TwilioWebhookURL = prevURL
		config.TwilioCallbackDelay = prevDelay
	}
}

// waitFor collects exactly n callbacks, failing if they do not arrive in time.
func waitFor(t *testing.T, ch chan url.Values, n int) []url.Values {
	t.Helper()

	got := make([]url.Values, 0, n)
	for len(got) < n {
		select {
		case v := <-ch:
			got = append(got, v)
		case <-time.After(3 * time.Second):
			t.Fatalf("timed out waiting for callback %d of %d (got %d)", len(got)+1, n, len(got))
		}
	}
	return got
}

// expectNoMore asserts the progression has stopped.
func expectNoMore(t *testing.T, ch chan url.Values) {
	t.Helper()

	select {
	case v := <-ch:
		t.Fatalf("unexpected extra callback: MessageStatus=%q", v.Get("MessageStatus"))
	case <-time.After(300 * time.Millisecond):
	}
}

func statuses(vals []url.Values) []string {
	out := make([]string, len(vals))
	for i, v := range vals {
		out[i] = v.Get("MessageStatus")
	}
	return out
}

func TestStatusCallback_DeliveredProgression(t *testing.T) {
	setup()
	defer storage.Close()

	ch, _, cleanup := callbackCollector(t)
	defer cleanup()

	rr := makeRequest(t, "ACtest", url.Values{
		"From": {"+15550001111"},
		"To":   {"+15552223333"},
		"Body": {"Hello"},
	}, "")
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	got := statuses(waitFor(t, ch, 3))
	want := []string{"queued", "sent", "delivered"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("status progression = %v, want %v", got, want)
	}
	expectNoMore(t, ch)
}

func TestStatusCallback_Fields(t *testing.T) {
	setup()
	defer storage.Close()

	ch, _, cleanup := callbackCollector(t)
	defer cleanup()

	makeRequest(t, "ACfields", url.Values{
		"From": {"+15550001111"},
		"To":   {"+15552223333"},
		"Body": {"Hello"},
	}, "")

	got := waitFor(t, ch, 3)
	first := got[0]

	if first.Get("MessageSid") == "" {
		t.Error("MessageSid is empty")
	}
	// Twilio sends the legacy Sms* aliases alongside the Message* fields.
	if first.Get("SmsSid") != first.Get("MessageSid") {
		t.Errorf("SmsSid %q != MessageSid %q", first.Get("SmsSid"), first.Get("MessageSid"))
	}
	if first.Get("SmsStatus") != first.Get("MessageStatus") {
		t.Errorf("SmsStatus %q != MessageStatus %q", first.Get("SmsStatus"), first.Get("MessageStatus"))
	}
	if first.Get("AccountSid") != "ACfields" {
		t.Errorf("AccountSid = %q, want ACfields", first.Get("AccountSid"))
	}
	if first.Get("ApiVersion") != "2010-04-01" {
		t.Errorf("ApiVersion = %q", first.Get("ApiVersion"))
	}
	if first.Get("To") != "+15552223333" || first.Get("From") != "+15550001111" {
		t.Errorf("To/From = %q/%q", first.Get("To"), first.Get("From"))
	}
	// A successful progression must not carry an error code.
	for _, v := range got {
		if v.Get("ErrorCode") != "" {
			t.Errorf("%s callback carries ErrorCode %q", v.Get("MessageStatus"), v.Get("ErrorCode"))
		}
	}

	// The MessageSid must be stable across the whole progression.
	for _, v := range got[1:] {
		if v.Get("MessageSid") != first.Get("MessageSid") {
			t.Errorf("MessageSid changed mid-progression: %q vs %q", v.Get("MessageSid"), first.Get("MessageSid"))
		}
	}
}

func TestStatusCallback_FailureProgressions(t *testing.T) {
	tests := []struct {
		to           string
		wantStatuses []string
		wantCode     string
	}{
		{"+15005550010", []string{"queued", "failed"}, "30008"},
		{"+15005550011", []string{"queued", "sent", "undelivered"}, "30003"},
		{"+15005550012", []string{"queued", "sent", "undelivered"}, "30005"},
		{"+15005550013", []string{"queued", "sent", "undelivered"}, "30006"},
		{"+15005550014", []string{"queued", "sent"}, ""},
	}

	for _, tc := range tests {
		t.Run(tc.to, func(t *testing.T) {
			setup()
			defer storage.Close()

			ch, _, cleanup := callbackCollector(t)
			defer cleanup()

			rr := makeRequest(t, "ACtest", url.Values{
				"From": {"+15550001111"},
				"To":   {tc.to},
				"Body": {"Hello"},
			}, "")
			// The send itself still succeeds — the failure surfaces via callback.
			if rr.Code != http.StatusCreated {
				t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
			}

			got := waitFor(t, ch, len(tc.wantStatuses))
			if !reflect.DeepEqual(statuses(got), tc.wantStatuses) {
				t.Fatalf("status progression = %v, want %v", statuses(got), tc.wantStatuses)
			}
			expectNoMore(t, ch)

			last := got[len(got)-1]
			if last.Get("ErrorCode") != tc.wantCode {
				t.Errorf("ErrorCode = %q, want %q", last.Get("ErrorCode"), tc.wantCode)
			}
			if tc.wantCode != "" && last.Get("ErrorMessage") == "" {
				t.Error("failure callback has no ErrorMessage")
			}
			// Intermediate statuses must stay error-free.
			for _, v := range got[:len(got)-1] {
				if v.Get("ErrorCode") != "" {
					t.Errorf("%s callback carries ErrorCode %q", v.Get("MessageStatus"), v.Get("ErrorCode"))
				}
			}
		})
	}
}

func TestStatusCallback_PerRequestURLWins(t *testing.T) {
	setup()
	defer storage.Close()

	// The global URL points at a collector that must stay untouched.
	global, globalURL, cleanup := callbackCollector(t)
	defer cleanup()

	perRequest, perRequestURL, cleanup2 := callbackCollector(t)
	defer cleanup2()

	// The second collector repointed the global URL at itself; restore it so
	// the two destinations are genuinely distinct.
	config.TwilioWebhookURL = globalURL
	if globalURL == perRequestURL {
		t.Fatal("collectors share a URL; the test cannot distinguish them")
	}

	makeRequest(t, "ACtest", url.Values{
		"From":           {"+15550001111"},
		"To":             {"+15552223333"},
		"Body":           {"Hello"},
		"StatusCallback": {perRequestURL},
	}, "")

	got := statuses(waitFor(t, perRequest, 3))
	if !reflect.DeepEqual(got, []string{"queued", "sent", "delivered"}) {
		t.Fatalf("per-request progression = %v", got)
	}
	expectNoMore(t, global)
}

func TestStatusCallback_SignedWithAuthToken(t *testing.T) {
	setup()
	defer storage.Close()

	const token = "callback-signing-token"
	config.TwilioAuthToken = token
	defer func() { config.TwilioAuthToken = "" }()

	received := make(chan url.Values, 8)
	sigs := make(chan string, 8)

	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("bad form: %v", err)
			return
		}
		sigs <- r.Header.Get("X-Twilio-Signature")
		received <- r.PostForm
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	srvURL = srv.URL

	prev := config.TwilioWebhookURL
	config.TwilioWebhookURL = srvURL
	defer func() { config.TwilioWebhookURL = prev }()

	makeRequest(t, "ACsig", url.Values{
		"From": {"+15550001111"},
		"To":   {"+15552223333"},
		"Body": {"Hello"},
	}, token)

	for i := 0; i < 3; i++ {
		var params url.Values
		var sig string
		select {
		case params = <-received:
			sig = <-sigs
		case <-time.After(3 * time.Second):
			t.Fatalf("timed out waiting for callback %d", i+1)
		}

		if sig == "" {
			t.Fatalf("callback %d has no X-Twilio-Signature", i+1)
		}
		if want := smsCallbackSignature(srvURL, params, token); sig != want {
			t.Errorf("callback %d signature mismatch", i+1)
		}
		if bad := smsCallbackSignature(srvURL, params, "wrong-token"); sig == bad {
			t.Errorf("callback %d verified under the wrong token", i+1)
		}
	}
}

func TestMagicNumbers_APIErrors(t *testing.T) {
	tests := []struct {
		name     string
		from     string
		to       string
		wantCode float64
	}{
		{"invalid to", "+15550001111", "+15005550001", 21211},
		{"unreachable to", "+15550001111", "+15005550002", 21612},
		{"region not enabled", "+15550001111", "+15005550003", 21408},
		{"unsubscribed", "+15550001111", "+15005550004", 21610},
		{"not a mobile", "+15550001111", "+15005550009", 21614},
		{"invalid from", "+15005550001", "+15552223333", 21212},
		{"from not owned", "+15005550007", "+15552223333", 21606},
		{"from queue full", "+15005550008", "+15552223333", 21611},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setup()
			defer storage.Close()

			ch, _, cleanup := callbackCollector(t)
			defer cleanup()

			rr := makeRequest(t, "ACtest", url.Values{
				"From": {tc.from},
				"To":   {tc.to},
				"Body": {"Hello"},
			}, "")

			if rr.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
			}

			var body map[string]any
			if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
				t.Fatalf("response is not JSON: %v", err)
			}
			if body["code"] != tc.wantCode {
				t.Errorf("code = %v, want %v", body["code"], tc.wantCode)
			}
			if body["more_info"] == "" || body["more_info"] == nil {
				t.Error("more_info is missing")
			}
			if body["message"] == "" || body["message"] == nil {
				t.Error("message is missing")
			}

			// A rejected message is never stored and never fires a callback.
			stats, _ := storage.GetSMSMailboxStats()
			if stats.Total != 0 {
				t.Errorf("rejected message was stored (%d in mailbox)", stats.Total)
			}
			expectNoMore(t, ch)
		})
	}
}

func TestProgressionFor(t *testing.T) {
	if got := statusNames(progressionFor("+15552223333")); !reflect.DeepEqual(got, []string{"queued", "sent", "delivered"}) {
		t.Errorf("default progression = %v", got)
	}
	if got := statusNames(progressionFor("+15005550011")); !reflect.DeepEqual(got, []string{"queued", "sent", "undelivered"}) {
		t.Errorf("magic progression = %v", got)
	}
}

func statusNames(steps []statusStep) []string {
	out := make([]string, len(steps))
	for i, s := range steps {
		out[i] = s.Status
	}
	return out
}

func TestMagicKey_ToleratesFormatting(t *testing.T) {
	for _, in := range []string{"+15005550011", "15005550011", "+1 (500) 555-0011", "+1-500-555-0011"} {
		got := statusNames(progressionFor(in))
		want := []string{"queued", "sent", "undelivered"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("progressionFor(%q) = %v, want %v", in, got, want)
		}
	}

	// A number that merely looks similar must not match.
	if got := statusNames(progressionFor("+15005550111")); !reflect.DeepEqual(got, []string{"queued", "sent", "delivered"}) {
		t.Errorf("near-miss number matched a magic progression: %v", got)
	}
}
