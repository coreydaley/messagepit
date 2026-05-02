package mailtrap

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coreydaley/messagepit/config"
	"github.com/coreydaley/messagepit/internal/logger"
	"github.com/coreydaley/messagepit/internal/storage"
)

func setup() {
	logger.NoLogging = true
	config.MailtrapAPIKey = ""
	if err := storage.InitDB(); err != nil {
		panic(err)
	}
	if err := storage.DeleteAllMessages(); err != nil {
		panic(err)
	}
}

func makeRequest(t *testing.T, body any, authHeader string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/send", &buf)
	req.Header.Set("Content-Type", "application/json")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	rr := httptest.NewRecorder()
	CreateMessage(rr, req)
	return rr
}

func validPayload(overrides map[string]any) map[string]any {
	base := map[string]any{
		"from":    map[string]string{"email": "sender@example.com"},
		"to":      []map[string]string{{"email": "recipient@example.com"}},
		"subject": "Test Subject",
		"text":    "Hello",
	}
	for k, v := range overrides {
		base[k] = v
	}
	return base
}

func listMessages(t *testing.T) []storage.MessageSummary {
	t.Helper()
	msgs, err := storage.List(0, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	return msgs
}

func getRaw(t *testing.T, id string) string {
	t.Helper()
	raw, err := storage.GetMessageRaw(id)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// 1. Valid request returns 200
func TestCreateMessage_ValidRequest_Returns200(t *testing.T) {
	setup()
	defer storage.Close()

	rr := makeRequest(t, validPayload(nil), "")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

// 2. Response body contains success:true and message_ids
func TestCreateMessage_ResponseShape(t *testing.T) {
	setup()
	defer storage.Close()

	rr := makeRequest(t, validPayload(nil), "")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid JSON response: %s", err)
	}
	if resp["success"] != true {
		t.Errorf("expected success:true, got %v", resp["success"])
	}
	ids, ok := resp["message_ids"].([]any)
	if !ok || len(ids) == 0 {
		t.Errorf("expected non-empty message_ids, got %v", resp["message_ids"])
	}
}

// 3. Malformed JSON returns 400
func TestCreateMessage_MalformedJSON_Returns400(t *testing.T) {
	setup()
	defer storage.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/send", strings.NewReader(`{not valid json`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	CreateMessage(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// 4. Missing from returns 400
func TestCreateMessage_MissingFrom_Returns400(t *testing.T) {
	setup()
	defer storage.Close()

	payload := validPayload(map[string]any{"from": map[string]string{"email": ""}})
	rr := makeRequest(t, payload, "")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// 5. Missing subject returns 400
func TestCreateMessage_MissingSubject_Returns400(t *testing.T) {
	setup()
	defer storage.Close()

	payload := validPayload(map[string]any{"subject": ""})
	rr := makeRequest(t, payload, "")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// 6. Missing to returns 400
func TestCreateMessage_MissingTo_Returns400(t *testing.T) {
	setup()
	defer storage.Close()

	payload := validPayload(map[string]any{"to": []map[string]string{}})
	rr := makeRequest(t, payload, "")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// 7. Invalid API key returns 401
func TestCreateMessage_InvalidAuth_Returns401(t *testing.T) {
	setup()
	defer storage.Close()

	config.MailtrapAPIKey = "correct-key"
	defer func() { config.MailtrapAPIKey = "" }()

	rr := makeRequest(t, validPayload(nil), "Bearer wrong-key")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

// 8. Malformed auth header returns 401
func TestCreateMessage_MalformedAuthHeader_Returns401(t *testing.T) {
	setup()
	defer storage.Close()

	config.MailtrapAPIKey = "correct-key"
	defer func() { config.MailtrapAPIKey = "" }()

	rr := makeRequest(t, validPayload(nil), "correct-key")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing Bearer prefix, got %d", rr.Code)
	}
}

// 9. Valid Bearer token returns 200
func TestCreateMessage_ValidAuth_Returns200(t *testing.T) {
	setup()
	defer storage.Close()

	config.MailtrapAPIKey = "secret"
	defer func() { config.MailtrapAPIKey = "" }()

	rr := makeRequest(t, validPayload(nil), "Bearer secret")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid auth, got %d", rr.Code)
	}
}

// 10. No auth when key empty returns 200
func TestCreateMessage_NoAuthWhenKeyEmpty_Returns200(t *testing.T) {
	setup()
	defer storage.Close()

	rr := makeRequest(t, validPayload(nil), "")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 when API key unconfigured, got %d", rr.Code)
	}
}

// 11. Multi to[] recipients stored as ONE message
func TestCreateMessage_MultiRecipient_StoresOneMessage(t *testing.T) {
	setup()
	defer storage.Close()

	payload := validPayload(map[string]any{
		"to": []map[string]string{
			{"email": "a@example.com"},
			{"email": "b@example.com"},
			{"email": "c@example.com"},
		},
	})
	rr := makeRequest(t, payload, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	msgs := listMessages(t)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 stored message (all recipients in one), got %d", len(msgs))
	}
}

// 12. All to recipients appear in the To: header
func TestCreateMessage_MultipleToRecipients_InSingleMIME(t *testing.T) {
	setup()
	defer storage.Close()

	payload := validPayload(map[string]any{
		"to": []map[string]string{
			{"email": "a@example.com"},
			{"email": "b@example.com"},
		},
	})
	makeRequest(t, payload, "")

	msgs := listMessages(t)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	raw := getRaw(t, msgs[0].ID)
	if !strings.Contains(raw, "a@example.com") || !strings.Contains(raw, "b@example.com") {
		t.Errorf("expected both recipients in To header, got:\n%s", raw)
	}
}

// 13. CC appears in MIME
func TestCreateMessage_CCAddresses_AppearsInMIME(t *testing.T) {
	setup()
	defer storage.Close()

	payload := validPayload(map[string]any{
		"cc": []map[string]string{{"email": "cc@example.com", "name": "CC Person"}},
	})
	makeRequest(t, payload, "")

	msgs := listMessages(t)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	msg, err := storage.GetMessage(msgs[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msg.Cc) == 0 || msg.Cc[0].Address != "cc@example.com" {
		t.Errorf("expected cc@example.com in stored Cc, got %v", msg.Cc)
	}
}

// 14. BCC suppressed from MIME
func TestCreateMessage_BCC_SuppressedInMIME(t *testing.T) {
	setup()
	defer storage.Close()

	payload := validPayload(map[string]any{
		"bcc": []map[string]string{{"email": "bcc@example.com"}},
	})
	makeRequest(t, payload, "")

	msgs := listMessages(t)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	raw := getRaw(t, msgs[0].ID)
	if strings.Contains(raw, "Bcc:") || strings.Contains(raw, "bcc:") {
		t.Error("Bcc header must not appear in stored MIME")
	}
}

// 15. Text-only sets Content-Type text/plain
func TestCreateMessage_TextOnly_ContentTypePlain(t *testing.T) {
	setup()
	defer storage.Close()

	payload := validPayload(map[string]any{"text": "plain only"})
	makeRequest(t, payload, "")

	msgs := listMessages(t)
	raw := getRaw(t, msgs[0].ID)
	if !strings.Contains(raw, "Content-Type: text/plain") {
		t.Errorf("expected text/plain content type, got:\n%s", raw)
	}
}

// 16. HTML-only sets Content-Type text/html
func TestCreateMessage_HTMLOnly_ContentTypeHTML(t *testing.T) {
	setup()
	defer storage.Close()

	payload := validPayload(map[string]any{"text": "", "html": "<p>hello</p>"})
	makeRequest(t, payload, "")

	msgs := listMessages(t)
	raw := getRaw(t, msgs[0].ID)
	if !strings.Contains(raw, "Content-Type: text/html") {
		t.Errorf("expected text/html content type, got:\n%s", raw)
	}
}

// 17. Both text and html produces multipart/alternative
func TestCreateMessage_BothTextAndHTML_MultipartAlternative(t *testing.T) {
	setup()
	defer storage.Close()

	payload := validPayload(map[string]any{
		"text": "plain",
		"html": "<p>html</p>",
	})
	makeRequest(t, payload, "")

	msgs := listMessages(t)
	raw := getRaw(t, msgs[0].ID)
	if !strings.Contains(raw, "multipart/alternative") {
		t.Errorf("expected multipart/alternative, got:\n%s", raw)
	}
	if !strings.Contains(raw, "plain") || !strings.Contains(raw, "html") {
		t.Errorf("expected both text parts in body, got:\n%s", raw)
	}
}

// 18. Custom headers appear in MIME
func TestCreateMessage_CustomHeaders_AppearsInMIME(t *testing.T) {
	setup()
	defer storage.Close()

	payload := validPayload(map[string]any{
		"headers": map[string]string{"X-Custom-Header": "my-value"},
	})
	makeRequest(t, payload, "")

	msgs := listMessages(t)
	raw := getRaw(t, msgs[0].ID)
	if !strings.Contains(raw, "X-Custom-Header: my-value") {
		t.Errorf("expected custom header in MIME, got:\n%s", raw)
	}
}

// 19. Reserved headers are dropped silently
func TestCreateMessage_ReservedHeader_Dropped(t *testing.T) {
	setup()
	defer storage.Close()

	payload := validPayload(map[string]any{
		"headers": map[string]string{
			"From":         "attacker@evil.com",
			"Content-Type": "text/evil",
		},
	})
	makeRequest(t, payload, "")

	msgs := listMessages(t)
	raw := getRaw(t, msgs[0].ID)
	if strings.Contains(raw, "attacker@evil.com") {
		t.Error("reserved From header must not be overridden")
	}
}

// 20. CR/LF in header value is stripped (no injected standalone header line)
func TestCreateMessage_HeaderInjection_Sanitized(t *testing.T) {
	setup()
	defer storage.Close()

	payload := validPayload(map[string]any{
		"headers": map[string]string{"X-Injected": "val\r\nX-Extra: injected"},
	})
	makeRequest(t, payload, "")

	msgs := listMessages(t)
	raw := getRaw(t, msgs[0].ID)
	// After sanitization, no standalone X-Extra header line should exist.
	if strings.Contains(raw, "\nX-Extra: injected") {
		t.Error("header injection via CR/LF must be stripped")
	}
}

// 21. Invalid from address returns 400
func TestCreateMessage_InvalidFromAddress_Returns400(t *testing.T) {
	setup()
	defer storage.Close()

	payload := validPayload(map[string]any{
		"from": map[string]string{"email": "not-an-email"},
	})
	rr := makeRequest(t, payload, "")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid from address, got %d", rr.Code)
	}
}

// 22. Invalid to address returns 400
func TestCreateMessage_InvalidToAddress_Returns400(t *testing.T) {
	setup()
	defer storage.Close()

	payload := validPayload(map[string]any{
		"to": []map[string]string{{"email": "not-an-email"}},
	})
	rr := makeRequest(t, payload, "")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid to address, got %d", rr.Code)
	}
}

// 23. Ignored fields (category, custom_variables, attachments) accepted without error
func TestCreateMessage_IgnoredFields_AcceptedSuccessfully(t *testing.T) {
	setup()
	defer storage.Close()

	payload := validPayload(map[string]any{
		"category":         "transactional",
		"custom_variables": map[string]string{"key": "val"},
		"attachments":      []map[string]string{{"content": "base64data", "filename": "file.txt"}},
	})
	rr := makeRequest(t, payload, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 when ignored fields present, got %d: %s", rr.Code, rr.Body.String())
	}
}

// 24. Display name encoded in formatted address
func TestCreateMessage_DisplayName_AppearsInMIME(t *testing.T) {
	setup()
	defer storage.Close()

	payload := validPayload(map[string]any{
		"from": map[string]string{"email": "sender@example.com", "name": "Alice"},
		"to":   []map[string]string{{"email": "bob@example.com", "name": "Bob"}},
	})
	makeRequest(t, payload, "")

	msgs := listMessages(t)
	raw := getRaw(t, msgs[0].ID)
	if !strings.Contains(raw, "sender@example.com") {
		t.Errorf("expected sender address in MIME, got:\n%s", raw)
	}
	if !strings.Contains(raw, "bob@example.com") {
		t.Errorf("expected recipient address in MIME, got:\n%s", raw)
	}
}
