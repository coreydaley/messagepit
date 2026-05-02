package mailadapter_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coreydaley/messagepit/internal/logger"
	"github.com/coreydaley/messagepit/internal/mailadapter"
)

func init() {
	logger.NoLogging = true
}

// TestLimitBody_AllowsBodyUnder10MB verifies that a small body reads without error after LimitBody.
func TestLimitBody_AllowsBodyUnder10MB(t *testing.T) {
	body := strings.NewReader("hello")
	r := httptest.NewRequest(http.MethodPost, "/", body)
	w := httptest.NewRecorder()
	mailadapter.LimitBody(w, r)

	data, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("unexpected error reading body: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("expected body 'hello', got %q", string(data))
	}
}

// TestLimitBody_ReadFailsOver10MB verifies that reading beyond 10 MiB returns MaxBytesError.
func TestLimitBody_ReadFailsOver10MB(t *testing.T) {
	large := strings.NewReader(strings.Repeat("x", 10<<20+1))
	r := httptest.NewRequest(http.MethodPost, "/", large)
	w := httptest.NewRecorder()
	mailadapter.LimitBody(w, r)

	_, err := io.ReadAll(r.Body)
	if err == nil {
		t.Fatal("expected error reading body over 10 MiB limit, got nil")
	}
	var maxErr *http.MaxBytesError
	if !errors.As(err, &maxErr) {
		t.Errorf("expected *http.MaxBytesError, got %T: %v", err, err)
	}
}

// TestBearerAuth_BypassesWhenKeyEmpty verifies that an empty key accepts any request.
func TestBearerAuth_BypassesWhenKeyEmpty(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	w := httptest.NewRecorder()
	if !mailadapter.BearerAuth(w, r, "", "[test]") {
		t.Error("expected BearerAuth to return true when key is empty")
	}
}

// TestBearerAuth_AcceptsMatchingBearerToken verifies that a correct token passes.
func TestBearerAuth_AcceptsMatchingBearerToken(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set("Authorization", "Bearer secret-key")
	w := httptest.NewRecorder()
	if !mailadapter.BearerAuth(w, r, "secret-key", "[test]") {
		t.Error("expected BearerAuth to return true for matching token")
	}
}

// TestBearerAuth_RejectsMissingBearerPrefix verifies that a raw token without "Bearer " is rejected.
func TestBearerAuth_RejectsMissingBearerPrefix(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set("Authorization", "secret-key")
	w := httptest.NewRecorder()
	if mailadapter.BearerAuth(w, r, "secret-key", "[test]") {
		t.Error("expected BearerAuth to return false for missing Bearer prefix")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// TestBearerAuth_RejectsWrongToken verifies that an incorrect token is rejected.
func TestBearerAuth_RejectsWrongToken(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set("Authorization", "Bearer wrong")
	w := httptest.NewRecorder()
	if mailadapter.BearerAuth(w, r, "correct", "[test]") {
		t.Error("expected BearerAuth to return false for wrong token")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// TestBearerAuth_PartialKeyIsRejected verifies that a prefix of the real key is rejected.
func TestBearerAuth_PartialKeyIsRejected(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set("Authorization", "Bearer partial")
	w := httptest.NewRecorder()
	if mailadapter.BearerAuth(w, r, "partial-full", "[test]") {
		t.Error("expected BearerAuth to return false for partial key")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// TestBearerAuth_MissingAuthHeaderRejected verifies that an absent Authorization header is rejected.
func TestBearerAuth_MissingAuthHeaderRejected(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	w := httptest.NewRecorder()
	if mailadapter.BearerAuth(w, r, "some-key", "[test]") {
		t.Error("expected BearerAuth to return false when Authorization header is absent")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// TestFormatAddress_EmailOnly verifies that an address without a name returns the bare email.
func TestFormatAddress_EmailOnly(t *testing.T) {
	result, err := mailadapter.FormatAddress("user@example.com", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "user@example.com" {
		t.Errorf("expected bare email, got %q", result)
	}
}

// TestFormatAddress_NameUsesQEncoding verifies that a name produces a formatted address.
// mime.QEncoding.Encode only wraps in encoded-word format for non-ASCII; ASCII names are used verbatim.
func TestFormatAddress_NameUsesQEncoding(t *testing.T) {
	result, err := mailadapter.FormatAddress("user@example.com", "Alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Alice") {
		t.Errorf("expected display name in result, got %q", result)
	}
	if !strings.Contains(result, "<user@example.com>") {
		t.Errorf("expected email in angle brackets, got %q", result)
	}
}

// TestFormatAddress_UTF8NameEncoded verifies that a non-ASCII name is properly Q-encoded.
func TestFormatAddress_UTF8NameEncoded(t *testing.T) {
	result, err := mailadapter.FormatAddress("user@example.com", "Ré mi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "=?utf-8?") {
		t.Errorf("expected Q-encoded UTF-8 name, got %q", result)
	}
}

// TestFormatAddress_InvalidEmailReturnsError verifies that a bad email returns an error.
func TestFormatAddress_InvalidEmailReturnsError(t *testing.T) {
	_, err := mailadapter.FormatAddress("not-an-email", "")
	if err == nil {
		t.Error("expected error for invalid email, got nil")
	}
}

// TestSanitizeHeaderValue_StripsCRLF verifies that CR and LF are removed independently.
func TestSanitizeHeaderValue_StripsCRLF(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"val\r\nX-Injected: bad", "valX-Injected: bad"},
		{"val\nonly-lf", "valonly-lf"},
		{"val\ronly-cr", "valonly-cr"},
	}
	for _, tt := range tests {
		got := mailadapter.SanitizeHeaderValue(tt.input)
		if got != tt.want {
			t.Errorf("SanitizeHeaderValue(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestSanitizeHeaderValue_CleanValuePassesThrough verifies that a clean value is unchanged.
func TestSanitizeHeaderValue_CleanValuePassesThrough(t *testing.T) {
	input := "normal header value"
	got := mailadapter.SanitizeHeaderValue(input)
	if got != input {
		t.Errorf("expected %q unchanged, got %q", input, got)
	}
}

// TestJSONError_WritesStatusContentTypeAndBody verifies status, Content-Type, and JSON body.
func TestJSONError_WritesStatusContentTypeAndBody(t *testing.T) {
	w := httptest.NewRecorder()
	mailadapter.JSONError(w, http.StatusBadRequest, "something went wrong")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v\nbody: %s", err, w.Body.String())
	}
	if body["error"] != "something went wrong" {
		t.Errorf("expected error 'something went wrong', got %v", body["error"])
	}
}
