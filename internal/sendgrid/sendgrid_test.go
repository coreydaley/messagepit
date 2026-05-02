package sendgrid

import (
	"encoding/json"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coreydaley/messagepit/config"
	"github.com/coreydaley/messagepit/internal/logger"
	"github.com/coreydaley/messagepit/internal/storage"
	"github.com/gorilla/mux"
)

func setup() {
	logger.NoLogging = true
	config.SendGridAPIKey = ""
	if err := storage.InitDB(); err != nil {
		panic(err)
	}
	if err := storage.DeleteAllMessages(); err != nil {
		panic(err)
	}
}

func assertJSONError(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body is not valid JSON: %v\nbody: %s", err, rr.Body.String())
	}
	if _, ok := body["error"]; !ok {
		t.Errorf("expected 'error' key in JSON response body, got: %v", body)
	}
}

func makeRequest(t *testing.T, body string, authHeader string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/v3/mail/send", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	rr := httptest.NewRecorder()
	router := mux.NewRouter()
	router.HandleFunc("/v3/mail/send", CreateMessage).Methods("POST")
	router.ServeHTTP(rr, req)
	return rr
}

func validPayload(t *testing.T, fields map[string]any) string {
	t.Helper()
	base := map[string]any{
		"from":    map[string]string{"email": "sender@example.com"},
		"subject": "Test Subject",
		"personalizations": []map[string]any{
			{"to": []map[string]string{{"email": "recipient@example.com"}}},
		},
		"content": []map[string]string{{"type": "text/plain", "value": "Hello"}},
	}
	for k, v := range fields {
		base[k] = v
	}
	b, err := json.Marshal(base)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func listMessages(t *testing.T) []storage.MessageSummary {
	t.Helper()
	msgs, err := storage.List(0, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	return msgs
}

func TestCreateMessage_ValidRequest_Returns202(t *testing.T) {
	setup()
	defer storage.Close()

	rr := makeRequest(t, validPayload(t, nil), "")
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCreateMessage_MalformedJSON_Returns400(t *testing.T) {
	setup()
	defer storage.Close()

	rr := makeRequest(t, `{not valid json`, "")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	assertJSONError(t, rr)
}

func TestCreateMessage_MissingFrom_Returns400(t *testing.T) {
	setup()
	defer storage.Close()

	payload := `{"subject":"Test","personalizations":[{"to":[{"email":"r@example.com"}]}]}`
	rr := makeRequest(t, payload, "")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestCreateMessage_MissingSubject_Returns400(t *testing.T) {
	setup()
	defer storage.Close()

	payload := `{"from":{"email":"s@example.com"},"personalizations":[{"to":[{"email":"r@example.com"}]}]}`
	rr := makeRequest(t, payload, "")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestCreateMessage_MissingPersonalizations_Returns400(t *testing.T) {
	setup()
	defer storage.Close()

	payload := `{"from":{"email":"s@example.com"},"subject":"Test","personalizations":[]}`
	rr := makeRequest(t, payload, "")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestCreateMessage_InvalidAuth_Returns401(t *testing.T) {
	setup()
	defer storage.Close()

	config.SendGridAPIKey = "correct-key"
	defer func() { config.SendGridAPIKey = "" }()

	rr := makeRequest(t, validPayload(t, nil), "Bearer wrong-key")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
	assertJSONError(t, rr)
}

func TestCreateMessage_MalformedAuthHeader_Returns401(t *testing.T) {
	setup()
	defer storage.Close()

	config.SendGridAPIKey = "correct-key"
	defer func() { config.SendGridAPIKey = "" }()

	// Not in "Bearer <token>" form — TrimPrefix leaves the full value, which won't match.
	rr := makeRequest(t, validPayload(t, nil), "correct-key")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for malformed auth header, got %d", rr.Code)
	}
	assertJSONError(t, rr)
}

func TestCreateMessage_NoAuthWhenKeyEmpty_Returns202(t *testing.T) {
	setup()
	defer storage.Close()

	// config.SendGridAPIKey is "" — auth bypass path; any request is accepted.
	rr := makeRequest(t, validPayload(t, nil), "")
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202 when API key unconfigured, got %d", rr.Code)
	}
}

func TestCreateMessage_MultiRecipient_StoresAll(t *testing.T) {
	setup()
	defer storage.Close()

	payload := validPayload(t, map[string]any{
		"personalizations": []map[string]any{
			{"to": []map[string]string{
				{"email": "a@example.com"},
				{"email": "b@example.com"},
			}},
		},
	})

	rr := makeRequest(t, payload, "")
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}

	msgs := listMessages(t)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages stored, got %d", len(msgs))
	}

	// Verify each stored message has the correct To header for its personalization recipient.
	seen := map[string]bool{"a@example.com": false, "b@example.com": false}
	for _, m := range msgs {
		msg, err := storage.GetMessage(m.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, addr := range msg.To {
			if _, ok := seen[addr.Address]; ok {
				seen[addr.Address] = true
			}
		}
	}
	for addr, found := range seen {
		if !found {
			t.Errorf("recipient %s not found in stored messages", addr)
		}
	}
}

func TestCreateMessage_CCAddresses_AppearsInMIME(t *testing.T) {
	setup()
	defer storage.Close()

	payload := validPayload(t, map[string]any{
		"personalizations": []map[string]any{
			{
				"to": []map[string]string{{"email": "to@example.com"}},
				"cc": []map[string]string{{"email": "cc@example.com", "name": "Rémi CC"}},
			},
		},
	})

	rr := makeRequest(t, payload, "")
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}

	msgs := listMessages(t)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	msg, err := storage.GetMessage(msgs[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msg.Cc) == 0 {
		t.Fatal("expected Cc in stored message, got none")
	}
	if msg.Cc[0].Address != "cc@example.com" {
		t.Errorf("expected cc@example.com, got %q", msg.Cc[0].Address)
	}

	raw, err := storage.GetMessageRaw(msgs[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "=?utf-8?") {
		t.Errorf("expected Q-encoded display name in Cc header, got:\n%s", string(raw))
	}
}

func TestCreateMessage_BCC_SuppressedInMIME(t *testing.T) {
	setup()
	defer storage.Close()

	payload := validPayload(t, map[string]any{
		"personalizations": []map[string]any{
			{
				"to":  []map[string]string{{"email": "to@example.com"}},
				"bcc": []map[string]string{{"email": "bcc@example.com"}},
			},
		},
	})

	rr := makeRequest(t, payload, "")
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}

	msgs := listMessages(t)
	// Primary To recipient must still be stored.
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message stored (primary To only), got %d", len(msgs))
	}

	// The stored MIME must not contain a Bcc: header.
	raw, err := storage.GetMessageRaw(msgs[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	rawStr := string(raw)
	if strings.Contains(rawStr, "Bcc:") || strings.Contains(rawStr, "bcc:") {
		t.Error("Bcc header must not appear in stored MIME")
	}
}

func TestCreateMessage_NotificationID_SetAsHeader(t *testing.T) {
	setup()
	defer storage.Close()

	payload := validPayload(t, map[string]any{
		"custom_args": map[string]string{"notification_id": "notif-123"},
	})

	rr := makeRequest(t, payload, "")
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}

	msgs := listMessages(t)
	if len(msgs) == 0 {
		t.Fatal("no messages stored")
	}

	raw, err := storage.GetMessageRaw(msgs[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "X-Notification-Id: notif-123") {
		t.Errorf("expected X-Notification-Id: notif-123 in MIME, got:\n%s", string(raw))
	}
}

func TestCreateMessage_CustomArgsPersonalizationOverride(t *testing.T) {
	setup()
	defer storage.Close()

	// Message-level notification_id must be overridden by the personalization-level value.
	payload := validPayload(t, map[string]any{
		"custom_args": map[string]string{"notification_id": "msg-level-id"},
		"personalizations": []map[string]any{
			{
				"to":          []map[string]string{{"email": "to@example.com"}},
				"custom_args": map[string]string{"notification_id": "persona-level-id"},
			},
		},
	})

	rr := makeRequest(t, payload, "")
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}

	msgs := listMessages(t)
	if len(msgs) == 0 {
		t.Fatal("no messages stored")
	}

	raw, err := storage.GetMessageRaw(msgs[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	rawStr := string(raw)
	if !strings.Contains(rawStr, "X-Notification-Id: persona-level-id") {
		t.Errorf("expected personalization-level value; got:\n%s", rawStr)
	}
	if strings.Contains(rawStr, "msg-level-id") {
		t.Errorf("message-level value must be overridden; got:\n%s", rawStr)
	}
}

func TestCreateMessage_MultiContent_MultipartAlternative(t *testing.T) {
	setup()
	defer storage.Close()

	payload := validPayload(t, map[string]any{
		"content": []map[string]string{
			{"type": "text/plain", "value": "plain text"},
			{"type": "text/html", "value": "<p>html</p>"},
		},
	})

	rr := makeRequest(t, payload, "")
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}

	msgs := listMessages(t)
	if len(msgs) == 0 {
		t.Fatal("no messages stored")
	}

	raw, err := storage.GetMessageRaw(msgs[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	rawStr := string(raw)

	// Find the multipart Content-Type header line.
	var ctValue string
	for _, line := range strings.Split(rawStr, "\r\n") {
		if strings.HasPrefix(line, "Content-Type:") {
			ctValue = strings.TrimPrefix(line, "Content-Type: ")
			break
		}
	}
	if ctValue == "" {
		t.Fatal("no Content-Type header found in stored MIME")
	}

	mediaType, params, err := mime.ParseMediaType(ctValue)
	if err != nil {
		t.Fatalf("failed to parse Content-Type %q: %v", ctValue, err)
	}
	if mediaType != "multipart/alternative" {
		t.Errorf("expected multipart/alternative, got %q", mediaType)
	}

	boundary := params["boundary"]
	if boundary == "" {
		t.Fatal("no boundary in multipart Content-Type")
	}
	if !strings.Contains(rawStr, "--"+boundary) {
		t.Error("boundary delimiter not found in MIME body")
	}
	_ = multipart.NewReader(strings.NewReader(rawStr), boundary)
}

func TestCreateMessage_BodyTooLarge_Returns413(t *testing.T) {
	setup()
	defer storage.Close()

	// Build a body just over the 10 MiB limit.
	large := make([]byte, 10<<20+1)
	req := httptest.NewRequest(http.MethodPost, "/v3/mail/send", strings.NewReader(string(large)))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	router := mux.NewRouter()
	router.HandleFunc("/v3/mail/send", CreateMessage).Methods("POST")
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 for oversized body, got %d", rr.Code)
	}
	assertJSONError(t, rr)
}

func TestCreateMessage_InvalidFromAddress_Returns400(t *testing.T) {
	setup()
	defer storage.Close()

	payload := `{"from":{"email":"not-an-email"},"subject":"Test","personalizations":[{"to":[{"email":"r@example.com"}]}]}`
	rr := makeRequest(t, payload, "")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid from address, got %d", rr.Code)
	}
	assertJSONError(t, rr)
}

func TestCreateMessage_HeaderInjection_Sanitized(t *testing.T) {
	setup()
	defer storage.Close()

	payload := validPayload(t, map[string]any{
		"headers": map[string]string{"X-Injected": "val\r\nX-Extra: injected"},
	})
	rr := makeRequest(t, payload, "")
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}

	msgs := listMessages(t)
	if len(msgs) == 0 {
		t.Fatal("no messages stored")
	}
	raw, err := storage.GetMessageRaw(msgs[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "\nX-Extra: injected") {
		t.Error("header injection via CR/LF must be stripped")
	}
}

func TestCreateMessage_ConstantTimeAuth_RejectsPartialKey(t *testing.T) {
	setup()
	defer storage.Close()

	config.SendGridAPIKey = "full-secret-key"
	defer func() { config.SendGridAPIKey = "" }()

	// Send only a prefix of the real key — must be rejected.
	rr := makeRequest(t, validPayload(t, nil), "Bearer full-secret")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for partial key, got %d", rr.Code)
	}
}
