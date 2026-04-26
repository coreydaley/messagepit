package twilio

import (
	"crypto/hmac"
	"crypto/sha1" // #nosec G505
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

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
		// Compute valid Twilio signature
		fullURL := "http://" + req.Host + req.URL.RequestURI()
		keys := make([]string, 0, len(form))
		for k := range form {
			keys = append(keys, k)
		}
		// sort keys
		for i := 0; i < len(keys); i++ {
			for j := i + 1; j < len(keys); j++ {
				if keys[i] > keys[j] {
					keys[i], keys[j] = keys[j], keys[i]
				}
			}
		}
		var sb strings.Builder
		sb.WriteString(fullURL)
		for _, k := range keys {
			sb.WriteString(k)
			sb.WriteString(form.Get(k))
		}
		mac := hmac.New(sha1.New, []byte(authToken))
		mac.Write([]byte(sb.String())) // #nosec G104
		sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-Twilio-Signature", sig)
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
