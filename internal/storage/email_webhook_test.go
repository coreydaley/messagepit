package storage

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/coreydaley/messagepit/config"
	"github.com/coreydaley/messagepit/internal/mailevents"
)

// eventCollector points the email webhook config at a test endpoint and returns
// the events it receives, in arrival order.
func eventCollector(t *testing.T) (chan map[string]any, func()) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	events := make(chan map[string]any, 16)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading webhook body: %v", err)
			return
		}
		var batch []map[string]any
		if err := json.Unmarshal(body, &batch); err != nil {
			t.Errorf("webhook body is not a JSON array: %v", err)
			return
		}
		for _, e := range batch {
			events <- e
		}
		w.WriteHeader(http.StatusOK)
	}))

	prevURL, prevDelay := config.EmailWebhookURL, config.EmailWebhookEventDelay
	config.EmailWebhookURL = srv.URL
	config.EmailWebhookEventDelay = 0
	config.SetEmailWebhookPrivateKey(key)

	return events, func() {
		srv.Close()
		config.EmailWebhookURL = prevURL
		config.EmailWebhookEventDelay = prevDelay
		config.SetEmailWebhookPrivateKey(nil)
	}
}

// collectEvents drains n events, failing if they do not arrive in time.
func collectEvents(t *testing.T, ch chan map[string]any, n int) []map[string]any {
	t.Helper()

	got := make([]map[string]any, 0, n)
	for len(got) < n {
		select {
		case e := <-ch:
			got = append(got, e)
		case <-time.After(3 * time.Second):
			t.Fatalf("timed out waiting for event %d of %d (got %d)", len(got)+1, n, len(got))
		}
	}
	return got
}

func namesOf(events []map[string]any) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i], _ = e["event"].(string)
	}
	return out
}

func TestStore_FiresEventLifecycle(t *testing.T) {
	setup("")
	defer Close()

	events, cleanup := eventCollector(t)
	defer cleanup()

	msg := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: Hello\r\n" +
		"Message-ID: <lifecycle-1@messagepit>\r\n" +
		"\r\n" +
		"Body\r\n"
	body := []byte(msg)
	username := "sendgrid"

	if _, err := Store(&body, &username); err != nil {
		t.Fatal(err)
	}

	got := collectEvents(t, events, 2)
	if want := []string{"processed", "delivered"}; !reflect.DeepEqual(namesOf(got), want) {
		t.Fatalf("event sequence = %v, want %v", namesOf(got), want)
	}
	if got[0]["email"] != "recipient@example.com" {
		t.Errorf("email = %v", got[0]["email"])
	}
	if got[0]["smtp-id"] != "<lifecycle-1@messagepit>" {
		t.Errorf("smtp-id = %v", got[0]["smtp-id"])
	}
}

func TestStore_FiresWithoutCustomArgs(t *testing.T) {
	setup("")
	defer Close()

	events, cleanup := eventCollector(t)
	defer cleanup()

	// Real SendGrid fires the event webhook for everything it accepts, so a
	// plain SMTP message with no custom_args must still produce events.
	body := []byte("From: s@example.com\r\nTo: r@example.com\r\nSubject: Plain\r\n\r\nBody\r\n")
	username := ""

	if _, err := Store(&body, &username); err != nil {
		t.Fatal(err)
	}

	got := collectEvents(t, events, 2)
	if want := []string{"processed", "delivered"}; !reflect.DeepEqual(namesOf(got), want) {
		t.Fatalf("event sequence = %v, want %v", namesOf(got), want)
	}
}

func TestStore_CarriesSMTPAPIMetadata(t *testing.T) {
	setup("")
	defer Close()

	events, cleanup := eventCollector(t)
	defer cleanup()

	smtpAPI := mailevents.BuildSMTPAPI(
		[]string{"welcome", "onboarding"},
		map[string]string{"notification_id": "n-77", "tenant": "acme"},
	)

	body := []byte("From: s@example.com\r\nTo: r@example.com\r\nSubject: Meta\r\n" +
		mailevents.SMTPAPIHeader + ": " + smtpAPI + "\r\n\r\nBody\r\n")
	username := "sendgrid"

	if _, err := Store(&body, &username); err != nil {
		t.Fatal(err)
	}

	for _, e := range collectEvents(t, events, 2) {
		if e["notification_id"] != "n-77" {
			t.Errorf("notification_id = %v", e["notification_id"])
		}
		if e["tenant"] != "acme" {
			t.Errorf("tenant = %v", e["tenant"])
		}
		cats, ok := e["category"].([]any)
		if !ok || len(cats) != 2 {
			t.Errorf("category = %v", e["category"])
		}
	}
}

func TestStore_LegacyNotificationIDHeaderStillHonoured(t *testing.T) {
	setup("")
	defer Close()

	events, cleanup := eventCollector(t)
	defer cleanup()

	body := []byte("From: s@example.com\r\nTo: r@example.com\r\nSubject: Legacy\r\n" +
		"X-Notification-Id: legacy-9\r\n\r\nBody\r\n")
	username := ""

	if _, err := Store(&body, &username); err != nil {
		t.Fatal(err)
	}

	for _, e := range collectEvents(t, events, 2) {
		if e["notification_id"] != "legacy-9" {
			t.Errorf("notification_id = %v, want legacy-9", e["notification_id"])
		}
	}
}

func TestStore_ScenarioHeaderSelectsBranch(t *testing.T) {
	setup("")
	defer Close()

	events, cleanup := eventCollector(t)
	defer cleanup()

	body := []byte("From: s@example.com\r\nTo: r@example.com\r\nSubject: Bounce\r\n" +
		mailevents.ScenarioHeader + ": bounce\r\n\r\nBody\r\n")
	username := ""

	if _, err := Store(&body, &username); err != nil {
		t.Fatal(err)
	}

	got := collectEvents(t, events, 2)
	if want := []string{"processed", "bounce"}; !reflect.DeepEqual(namesOf(got), want) {
		t.Fatalf("event sequence = %v, want %v", namesOf(got), want)
	}
	if got[1]["type"] != "bounce" {
		t.Errorf("bounce type = %v", got[1]["type"])
	}
}

func TestStore_ScenarioFromRecipientAddress(t *testing.T) {
	setup("")
	defer Close()

	events, cleanup := eventCollector(t)
	defer cleanup()

	body := []byte("From: s@example.com\r\nTo: user+dropped@example.com\r\nSubject: Drop\r\n\r\nBody\r\n")
	username := ""

	if _, err := Store(&body, &username); err != nil {
		t.Fatal(err)
	}

	got := collectEvents(t, events, 2)
	if want := []string{"processed", "dropped"}; !reflect.DeepEqual(namesOf(got), want) {
		t.Fatalf("event sequence = %v, want %v", namesOf(got), want)
	}
}

func TestStore_NoEventsWhenWebhookDisabled(t *testing.T) {
	setup("")
	defer Close()

	events, cleanup := eventCollector(t)
	defer cleanup()

	config.EmailWebhookURL = ""

	body := []byte("From: s@example.com\r\nTo: r@example.com\r\nSubject: Quiet\r\n\r\nBody\r\n")
	username := ""

	if _, err := Store(&body, &username); err != nil {
		t.Fatal(err)
	}

	select {
	case e := <-events:
		t.Fatalf("unexpected event %v with the webhook disabled", e["event"])
	case <-time.After(300 * time.Millisecond):
	}
}
