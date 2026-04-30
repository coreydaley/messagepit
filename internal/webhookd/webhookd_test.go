package webhookd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coreydaley/messagepit/internal/logger"
	"github.com/coreydaley/messagepit/internal/storage"
)

func setup() {
	logger.NoLogging = true
	if err := storage.InitDB(); err != nil {
		panic(err)
	}
	if err := storage.DeleteAllWebhooks(); err != nil {
		panic(err)
	}
}

func makeRequest(t *testing.T, method, path, body, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	var reqBody *strings.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	} else {
		reqBody = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, reqBody)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rr := httptest.NewRecorder()
	CaptureHandler(rr, req)
	return rr
}

func TestCaptureHandlerReturns200(t *testing.T) {
	setup()
	defer storage.Close()

	rr := makeRequest(t, "POST", "/hook", `{"event":"test"}`, "application/json")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestCaptureHandlerStoresRequest(t *testing.T) {
	setup()
	defer storage.Close()

	makeRequest(t, "POST", "/api/event", `{"key":"value"}`, "application/json")

	stats, err := storage.GetWebhookMailboxStats()
	if err != nil {
		t.Fatalf("GetWebhookMailboxStats: %v", err)
	}
	if stats.Total != 1 {
		t.Fatalf("expected 1 stored webhook, got %d", stats.Total)
	}
}

func TestCaptureHandlerAllMethods(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "PATCH", "DELETE"}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			setup()
			defer storage.Close()

			rr := makeRequest(t, method, "/test", "", "")
			if rr.Code != http.StatusOK {
				t.Fatalf("%s: expected 200, got %d", method, rr.Code)
			}

			stats, _ := storage.GetWebhookMailboxStats()
			if stats.Total != 1 {
				t.Fatalf("%s: expected 1 stored, got %d", method, stats.Total)
			}
		})
	}
}

func TestCaptureHandlerCORSHeaders(t *testing.T) {
	setup()
	defer storage.Close()

	rr := makeRequest(t, "POST", "/hook", "data", "text/plain")
	if rr.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatal("expected Access-Control-Allow-Origin: *")
	}
}

func TestCaptureHandlerOptionsPreflightReturns204(t *testing.T) {
	setup()
	defer storage.Close()

	req := httptest.NewRequest(http.MethodOptions, "/hook", nil)
	rr := httptest.NewRecorder()
	CaptureHandler(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for OPTIONS, got %d", rr.Code)
	}
	// OPTIONS preflight should NOT store anything
	stats, _ := storage.GetWebhookMailboxStats()
	if stats.Total != 0 {
		t.Fatal("OPTIONS preflight should not be stored")
	}
}

func TestCaptureHandlerExtractsContentType(t *testing.T) {
	setup()
	defer storage.Close()

	// Content-Type with parameters — only the MIME type should be stored
	makeRequest(t, "POST", "/hook", `{"x":1}`, "application/json; charset=utf-8")

	msgs, err := storage.ListWebhooks(0, 10)
	if err != nil {
		t.Fatalf("ListWebhooks: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 webhook, got %d", len(msgs))
	}
	if msgs[0].ContentType != "application/json" {
		t.Fatalf("expected content type 'application/json', got %q", msgs[0].ContentType)
	}
}

func TestCaptureHandlerQueryString(t *testing.T) {
	setup()
	defer storage.Close()

	req := httptest.NewRequest("GET", "/search?q=hello&page=2", nil)
	rr := httptest.NewRecorder()
	CaptureHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	msgs, _ := storage.ListWebhooks(0, 10)
	if len(msgs) == 0 {
		t.Fatal("no webhooks stored")
	}

	msg, err := storage.GetWebhook(msgs[0].ID)
	if err != nil {
		t.Fatalf("GetWebhook: %v", err)
	}
	if msg.Query != "q=hello&page=2" {
		t.Fatalf("expected query 'q=hello&page=2', got %q", msg.Query)
	}
	if msg.Path != "/search" {
		t.Fatalf("expected path '/search', got %q", msg.Path)
	}
}

func TestCaptureHandlerStoresHeaders(t *testing.T) {
	setup()
	defer storage.Close()

	req := httptest.NewRequest("POST", "/hook", strings.NewReader("body"))
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("X-Signature", "abc123")
	rr := httptest.NewRecorder()
	CaptureHandler(rr, req)

	msgs, _ := storage.ListWebhooks(0, 10)
	if len(msgs) == 0 {
		t.Fatal("no webhooks stored")
	}
	msg, err := storage.GetWebhook(msgs[0].ID)
	if err != nil {
		t.Fatalf("GetWebhook: %v", err)
	}
	if len(msg.Headers["X-Signature"]) == 0 || msg.Headers["X-Signature"][0] != "abc123" {
		t.Fatalf("X-Signature header not stored correctly: %v", msg.Headers)
	}
}

func TestCaptureHandlerBody(t *testing.T) {
	setup()
	defer storage.Close()

	body := `{"event":"payment","amount":99.99}`
	makeRequest(t, "POST", "/webhook", body, "application/json")

	msgs, _ := storage.ListWebhooks(0, 10)
	if len(msgs) == 0 {
		t.Fatal("no webhooks stored")
	}
	msg, err := storage.GetWebhook(msgs[0].ID)
	if err != nil {
		t.Fatalf("GetWebhook: %v", err)
	}
	if msg.Body != body {
		t.Fatalf("body mismatch: got %q, want %q", msg.Body, body)
	}
	if msg.BodySize != int64(len(body)) {
		t.Fatalf("BodySize mismatch: got %d, want %d", msg.BodySize, len(body))
	}
}
