// Package mailevents emits SendGrid-style event webhooks for captured email.
//
// Real SendGrid does not deliver a single "delivered" event — it emits an
// ordered lifecycle (processed → delivered → open → click, or a failure branch
// such as processed → bounce). This package replays that lifecycle so an
// application's event-webhook handler exercises every state it would see in
// production.
package mailevents

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/coreydaley/messagepit/config"
	"github.com/coreydaley/messagepit/internal/logger"
	"github.com/lithammer/shortuuid/v4"
)

// Scenario names the delivery outcome to replay. Real SendGrid picks the
// outcome from what the receiving MTA does; MessagePit lets the caller choose
// it so failure paths are reachable in development.
const (
	ScenarioDelivered        = "delivered"
	ScenarioOpen             = "open"
	ScenarioClick            = "click"
	ScenarioDeferred         = "deferred"
	ScenarioBounce           = "bounce"
	ScenarioBlocked          = "blocked"
	ScenarioDropped          = "dropped"
	ScenarioSpamReport       = "spamreport"
	ScenarioUnsubscribe      = "unsubscribe"
	ScenarioGroupUnsubscribe = "group_unsubscribe"
	ScenarioGroupResubscribe = "group_resubscribe"
)

// sequences maps a scenario to the ordered events it produces.
var sequences = map[string][]string{
	ScenarioDelivered:        {"processed", "delivered"},
	ScenarioOpen:             {"processed", "delivered", "open"},
	ScenarioClick:            {"processed", "delivered", "open", "click"},
	ScenarioDeferred:         {"processed", "deferred", "delivered"},
	ScenarioBounce:           {"processed", "bounce"},
	ScenarioBlocked:          {"processed", "bounce"},
	ScenarioDropped:          {"processed", "dropped"},
	ScenarioSpamReport:       {"processed", "delivered", "spamreport"},
	ScenarioUnsubscribe:      {"processed", "delivered", "unsubscribe"},
	ScenarioGroupUnsubscribe: {"processed", "delivered", "group_unsubscribe"},
	ScenarioGroupResubscribe: {"processed", "delivered", "group_resubscribe"},
}

// userAgent and clientIP are the fixed values reported on engagement events.
const (
	userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36"
	clientIP  = "192.0.2.10"
	clickURL  = "https://example.com/"
)

// Message describes a captured email to replay events for.
type Message struct {
	// Email is the primary recipient address.
	Email string

	// MessageID is the MIME Message-ID, reported as the event "smtp-id".
	MessageID string

	// Categories are the SendGrid categories attached to the message.
	Categories []string

	// UniqueArgs are the SendGrid custom_args. Real SendGrid flattens these
	// onto every event as top-level keys, so MessagePit does too.
	UniqueArgs map[string]string

	// Scenario selects the lifecycle branch. Empty resolves from the
	// recipient address, defaulting to ScenarioDelivered.
	Scenario string
}

// Sequence returns the ordered event names a scenario produces.
// An unknown scenario falls back to the delivered lifecycle.
func Sequence(scenario string) []string {
	if seq, ok := sequences[scenario]; ok {
		return seq
	}
	return sequences[ScenarioDelivered]
}

// ScenarioFor resolves the lifecycle branch for a recipient address.
// A bare local part ("bounce@example.com") or a plus-address tag
// ("user+bounce@example.com") selects the matching scenario.
func ScenarioFor(email string) string {
	local, _, ok := strings.Cut(strings.ToLower(email), "@")
	if !ok {
		return ScenarioDelivered
	}

	parts := strings.Split(local, "+")
	// Check the plus tags first so "user+bounce" beats the local part itself.
	for i := len(parts) - 1; i >= 0; i-- {
		if _, ok := sequences[parts[i]]; ok {
			return parts[i]
		}
	}

	return ScenarioDelivered
}

// Fire replays the full event lifecycle for m in the background.
// It is a no-op when no email webhook URL or signing key is configured.
func Fire(m Message) {
	if config.EmailWebhookURL == "" || config.EmailWebhookPrivateKey() == nil {
		return
	}
	go fire(m)
}

// fire replays the lifecycle synchronously. Each event is posted on its own
// request so consumers observe the ordering; SendGrid batches events that land
// in the same window, which would hide the sequence during development.
func fire(m Message) {
	scenario := m.Scenario
	if scenario == "" {
		scenario = ScenarioFor(m.Email)
	}

	sgMessageID := shortuuid.New() + ".messagepit"

	for i, name := range Sequence(scenario) {
		if i > 0 && config.EmailWebhookEventDelay > 0 {
			time.Sleep(config.EmailWebhookEventDelay)
		}
		post(buildEvent(m, scenario, name, sgMessageID))
	}
}

// buildEvent assembles a single SendGrid event object.
func buildEvent(m Message, scenario, name, sgMessageID string) map[string]any {
	smtpID := strings.Trim(m.MessageID, "<>")
	if smtpID == "" {
		smtpID = sgMessageID
	}

	e := map[string]any{
		"email":         m.Email,
		"timestamp":     time.Now().Unix(),
		"smtp-id":       "<" + smtpID + ">",
		"event":         name,
		"sg_event_id":   shortuuid.New(),
		"sg_message_id": sgMessageID,
	}

	if len(m.Categories) > 0 {
		e["category"] = m.Categories
	}

	// custom_args ride along as top-level keys, matching real SendGrid.
	for k, v := range m.UniqueArgs {
		if _, reserved := e[k]; !reserved {
			e[k] = v
		}
	}

	switch name {
	case "processed":
		e["pool"] = map[string]any{"name": "messagepit", "id": 0}
	case "deferred":
		e["response"] = "400 4.2.0 Recipient mailbox temporarily unavailable"
		e["attempt"] = "1"
	case "delivered":
		e["response"] = "250 2.0.0 OK"
	case "bounce":
		if scenario == ScenarioBlocked {
			e["type"] = "blocked"
			e["status"] = "5.7.1"
			e["reason"] = "550 5.7.1 Message rejected by the receiving server"
			e["bounce_classification"] = "Reputation"
		} else {
			e["type"] = "bounce"
			e["status"] = "5.1.1"
			e["reason"] = "550 5.1.1 The email account that you tried to reach does not exist"
			e["bounce_classification"] = "Invalid Address"
		}
	case "dropped":
		e["status"] = "5.0.0"
		e["reason"] = "Bounced Address"
	case "open":
		e["useragent"] = userAgent
		e["ip"] = clientIP
		e["sg_machine_open"] = false
	case "click":
		e["useragent"] = userAgent
		e["ip"] = clientIP
		e["url"] = clickURL
		e["url_offset"] = map[string]any{"index": 0, "type": "html"}
	case "group_unsubscribe", "group_resubscribe":
		e["asm_group_id"] = 1
		e["useragent"] = userAgent
		e["ip"] = clientIP
		e["url"] = clickURL
	}

	return e
}

// post signs and delivers a single event. Errors are logged, not propagated.
func post(event map[string]any) {
	key := config.EmailWebhookPrivateKey()
	if key == nil || config.EmailWebhookURL == "" {
		return
	}

	body, err := json.Marshal([]map[string]any{event})
	if err != nil {
		logger.Log().Errorf("[email-webhook] marshal error: %s", err)
		return
	}

	ts := strconv.FormatInt(time.Now().Unix(), 10)
	hash := sha256.Sum256([]byte(ts + string(body)))
	sig, err := key.Sign(rand.Reader, hash[:], crypto.SHA256)
	if err != nil {
		logger.Log().Errorf("[email-webhook] signing error: %s", err)
		return
	}

	req, err := http.NewRequest("POST", config.EmailWebhookURL, bytes.NewReader(body))
	if err != nil {
		logger.Log().Errorf("[email-webhook] request error: %s", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "SendGrid Event API")
	req.Header.Set("X-Twilio-Email-Event-Webhook-Signature", base64.StdEncoding.EncodeToString(sig))
	req.Header.Set("X-Twilio-Email-Event-Webhook-Timestamp", ts)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logger.Log().Errorf("[email-webhook] error: %s", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		logger.Log().Warnf("[email-webhook] %s returned %d for %q", config.EmailWebhookURL, resp.StatusCode, event["event"])
	} else {
		logger.Log().Debugf("[email-webhook] %q event sent for %s", event["event"], event["email"])
	}
}
