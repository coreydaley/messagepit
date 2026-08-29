package mailevents

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/coreydaley/messagepit/config"
	"github.com/coreydaley/messagepit/internal/logger"
)

func init() {
	logger.NoLogging = true
}

// capture is a single received webhook request.
type capture struct {
	Event     map[string]any
	Body      []byte
	Signature string
	Timestamp string
}

// collector spins up a webhook endpoint and points the config at it.
// It returns the received events in arrival order, and a cleanup func.
func collector(t *testing.T) (*[]capture, *ecdsa.PublicKey, func()) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	got := []capture{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading webhook body: %v", err)
			return
		}
		var events []map[string]any
		if err := json.Unmarshal(body, &events); err != nil {
			t.Errorf("webhook body is not a JSON array: %v (%s)", err, body)
			return
		}
		if len(events) != 1 {
			t.Errorf("expected exactly 1 event per request, got %d", len(events))
			return
		}
		got = append(got, capture{
			Event:     events[0],
			Body:      body,
			Signature: r.Header.Get("X-Twilio-Email-Event-Webhook-Signature"),
			Timestamp: r.Header.Get("X-Twilio-Email-Event-Webhook-Timestamp"),
		})
		w.WriteHeader(http.StatusOK)
	}))

	prevURL, prevDelay := config.EmailWebhookURL, config.EmailWebhookEventDelay
	config.EmailWebhookURL = srv.URL
	config.EmailWebhookEventDelay = 0
	config.SetEmailWebhookPrivateKey(key)

	return &got, &key.PublicKey, func() {
		srv.Close()
		config.EmailWebhookURL = prevURL
		config.EmailWebhookEventDelay = prevDelay
		config.SetEmailWebhookPrivateKey(nil)
	}
}

func eventNames(got []capture) []string {
	names := make([]string, len(got))
	for i, c := range got {
		names[i], _ = c.Event["event"].(string)
	}
	return names
}

func TestScenarioFor(t *testing.T) {
	tests := []struct {
		email string
		want  string
	}{
		{"user@example.com", ScenarioDelivered},
		{"bounce@example.com", ScenarioBounce},
		{"BOUNCE@Example.COM", ScenarioBounce},
		{"user+bounce@example.com", ScenarioBounce},
		{"user+tag+dropped@example.com", ScenarioDropped},
		{"deferred@example.com", ScenarioDeferred},
		{"click@example.com", ScenarioClick},
		{"spamreport@example.com", ScenarioSpamReport},
		{"group_unsubscribe@example.com", ScenarioGroupUnsubscribe},
		{"not-an-address", ScenarioDelivered},
		{"", ScenarioDelivered},
		// An unknown tag must not hijack the branch.
		{"user+promo@example.com", ScenarioDelivered},
	}

	for _, tc := range tests {
		if got := ScenarioFor(tc.email); got != tc.want {
			t.Errorf("ScenarioFor(%q) = %q, want %q", tc.email, got, tc.want)
		}
	}
}

func TestSequenceUnknownScenarioFallsBackToDelivered(t *testing.T) {
	if got := Sequence("nonsense"); !reflect.DeepEqual(got, []string{"processed", "delivered"}) {
		t.Errorf("unknown scenario produced %v", got)
	}
}

func TestFireDefaultLifecycle(t *testing.T) {
	got, _, cleanup := collector(t)
	defer cleanup()

	fire(Message{Email: "user@example.com", MessageID: "<abc@messagepit>"})

	want := []string{"processed", "delivered"}
	if names := eventNames(*got); !reflect.DeepEqual(names, want) {
		t.Fatalf("event sequence = %v, want %v", names, want)
	}
}

func TestFireLifecycleOrderPerScenario(t *testing.T) {
	tests := []struct {
		scenario string
		want     []string
	}{
		{ScenarioDelivered, []string{"processed", "delivered"}},
		{ScenarioOpen, []string{"processed", "delivered", "open"}},
		{ScenarioClick, []string{"processed", "delivered", "open", "click"}},
		{ScenarioDeferred, []string{"processed", "deferred", "delivered"}},
		{ScenarioBounce, []string{"processed", "bounce"}},
		{ScenarioBlocked, []string{"processed", "bounce"}},
		{ScenarioDropped, []string{"processed", "dropped"}},
		{ScenarioSpamReport, []string{"processed", "delivered", "spamreport"}},
		{ScenarioUnsubscribe, []string{"processed", "delivered", "unsubscribe"}},
		{ScenarioGroupUnsubscribe, []string{"processed", "delivered", "group_unsubscribe"}},
		{ScenarioGroupResubscribe, []string{"processed", "delivered", "group_resubscribe"}},
	}

	for _, tc := range tests {
		t.Run(tc.scenario, func(t *testing.T) {
			got, _, cleanup := collector(t)
			defer cleanup()

			fire(Message{Email: "user@example.com", Scenario: tc.scenario})

			if names := eventNames(*got); !reflect.DeepEqual(names, tc.want) {
				t.Fatalf("event sequence = %v, want %v", names, tc.want)
			}
		})
	}
}

func TestFireScenarioResolvedFromAddress(t *testing.T) {
	got, _, cleanup := collector(t)
	defer cleanup()

	fire(Message{Email: "user+bounce@example.com"})

	want := []string{"processed", "bounce"}
	if names := eventNames(*got); !reflect.DeepEqual(names, want) {
		t.Fatalf("event sequence = %v, want %v", names, want)
	}
}

func TestFireExplicitScenarioBeatsAddress(t *testing.T) {
	got, _, cleanup := collector(t)
	defer cleanup()

	fire(Message{Email: "bounce@example.com", Scenario: ScenarioDelivered})

	want := []string{"processed", "delivered"}
	if names := eventNames(*got); !reflect.DeepEqual(names, want) {
		t.Fatalf("event sequence = %v, want %v", names, want)
	}
}

func TestFireBaseEventFields(t *testing.T) {
	got, _, cleanup := collector(t)
	defer cleanup()

	fire(Message{
		Email:      "user@example.com",
		MessageID:  "<msgid-123@messagepit>",
		Categories: []string{"welcome", "onboarding"},
		UniqueArgs: map[string]string{"notification_id": "n-42", "tenant": "acme"},
	})

	if len(*got) == 0 {
		t.Fatal("no events received")
	}

	// sg_message_id is stable across the lifecycle; sg_event_id is not.
	var msgIDs, eventIDs []string
	for _, c := range *got {
		e := c.Event

		if e["email"] != "user@example.com" {
			t.Errorf("email = %v", e["email"])
		}
		// smtp-id is the Message-ID, angle-bracketed exactly once.
		if e["smtp-id"] != "<msgid-123@messagepit>" {
			t.Errorf("smtp-id = %v, want <msgid-123@messagepit>", e["smtp-id"])
		}
		if _, ok := e["timestamp"].(float64); !ok {
			t.Errorf("timestamp is not numeric: %v", e["timestamp"])
		}
		// custom_args are flattened onto every event, as real SendGrid does.
		if e["notification_id"] != "n-42" {
			t.Errorf("notification_id = %v", e["notification_id"])
		}
		if e["tenant"] != "acme" {
			t.Errorf("tenant = %v", e["tenant"])
		}
		cats, ok := e["category"].([]any)
		if !ok || len(cats) != 2 || cats[0] != "welcome" {
			t.Errorf("category = %v", e["category"])
		}

		msgIDs = append(msgIDs, e["sg_message_id"].(string))
		eventIDs = append(eventIDs, e["sg_event_id"].(string))
	}

	for _, id := range msgIDs[1:] {
		if id != msgIDs[0] {
			t.Errorf("sg_message_id must be stable across the lifecycle: %v", msgIDs)
		}
	}
	if eventIDs[0] == eventIDs[1] {
		t.Errorf("sg_event_id must be unique per event: %v", eventIDs)
	}
}

func TestFireEventSpecificFields(t *testing.T) {
	tests := []struct {
		scenario string
		event    string
		want     map[string]any
	}{
		{ScenarioDelivered, "delivered", map[string]any{"response": "250 2.0.0 OK"}},
		{ScenarioDeferred, "deferred", map[string]any{"attempt": "1"}},
		{ScenarioBounce, "bounce", map[string]any{"type": "bounce", "status": "5.1.1"}},
		{ScenarioBlocked, "bounce", map[string]any{"type": "blocked", "status": "5.7.1"}},
		{ScenarioDropped, "dropped", map[string]any{"reason": "Bounced Address", "status": "5.0.0"}},
		{ScenarioClick, "click", map[string]any{"url": clickURL, "ip": clientIP}},
		{ScenarioOpen, "open", map[string]any{"ip": clientIP, "sg_machine_open": false}},
	}

	for _, tc := range tests {
		t.Run(tc.scenario+"/"+tc.event, func(t *testing.T) {
			got, _, cleanup := collector(t)
			defer cleanup()

			fire(Message{Email: "user@example.com", Scenario: tc.scenario})

			var found map[string]any
			for _, c := range *got {
				if c.Event["event"] == tc.event {
					found = c.Event
				}
			}
			if found == nil {
				t.Fatalf("no %q event in %v", tc.event, eventNames(*got))
			}
			for k, want := range tc.want {
				if found[k] != want {
					t.Errorf("%s = %#v, want %#v", k, found[k], want)
				}
			}
		})
	}
}

func TestFireSignatureVerifies(t *testing.T) {
	got, pub, cleanup := collector(t)
	defer cleanup()

	fire(Message{Email: "user@example.com", Scenario: ScenarioClick})

	if len(*got) != 4 {
		t.Fatalf("expected 4 events, got %d", len(*got))
	}

	// Every event in the lifecycle must be independently verifiable.
	for i, c := range *got {
		sig, err := base64.StdEncoding.DecodeString(c.Signature)
		if err != nil {
			t.Fatalf("event %d: signature is not base64: %v", i, err)
		}
		hash := sha256.Sum256(append([]byte(c.Timestamp), c.Body...))
		if !ecdsa.VerifyASN1(pub, hash[:], sig) {
			t.Errorf("event %d (%v): signature does not verify", i, c.Event["event"])
		}
	}
}

func TestFireSignatureRejectsTamperedBody(t *testing.T) {
	got, pub, cleanup := collector(t)
	defer cleanup()

	fire(Message{Email: "user@example.com"})

	c := (*got)[0]
	sig, err := base64.StdEncoding.DecodeString(c.Signature)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(append([]byte(c.Timestamp), append(c.Body, ' ')...))
	if ecdsa.VerifyASN1(pub, hash[:], sig) {
		t.Error("signature verified against a tampered payload")
	}
}

func TestFireNoOpWhenUnconfigured(t *testing.T) {
	got, _, cleanup := collector(t)
	defer cleanup()

	config.EmailWebhookURL = ""
	fire(Message{Email: "user@example.com"})

	if len(*got) != 0 {
		t.Fatalf("expected no events without a webhook URL, got %d", len(*got))
	}
}

func TestSMTPAPIRoundTrip(t *testing.T) {
	value := BuildSMTPAPI([]string{"welcome"}, map[string]string{"notification_id": "n-1"})
	if value == "" {
		t.Fatal("BuildSMTPAPI returned empty for non-empty input")
	}

	cats, args := ParseSMTPAPI(value)
	if !reflect.DeepEqual(cats, []string{"welcome"}) {
		t.Errorf("categories = %v", cats)
	}
	if args["notification_id"] != "n-1" {
		t.Errorf("unique_args = %v", args)
	}
}

func TestSMTPAPIEmptyAndMalformed(t *testing.T) {
	if v := BuildSMTPAPI(nil, nil); v != "" {
		t.Errorf("expected empty header value for empty input, got %q", v)
	}

	for _, in := range []string{"", "   ", "not json", "[1,2,3]"} {
		cats, args := ParseSMTPAPI(in)
		if cats != nil || args != nil {
			t.Errorf("ParseSMTPAPI(%q) = %v, %v; want nil, nil", in, cats, args)
		}
	}
}

func TestSMTPAPIStaysOnOneLine(t *testing.T) {
	// A header value spanning lines would let a caller inject headers.
	value := BuildSMTPAPI([]string{"a\r\nX-Injected: yes"}, map[string]string{"k\nX-Also": "v"})
	if value == "" {
		t.Fatal("expected a header value")
	}
	for _, c := range []string{"\r", "\n"} {
		if contains(value, c) {
			t.Errorf("header value contains a bare %q: %q", c, value)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
