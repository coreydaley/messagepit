package storage

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/coreydaley/messagepit/config"
	"github.com/coreydaley/messagepit/internal/logger"
)

// fireEmailWebhook posts a SendGrid-style "delivered" event to config.EmailWebhookURL.
// The call is non-blocking; errors are logged but not propagated.
func fireEmailWebhook(notificationID, email string) {
	key := config.EmailWebhookPrivateKey()
	if key == nil || config.EmailWebhookURL == "" {
		return
	}

	ts := strconv.FormatInt(time.Now().Unix(), 10)
	events := []map[string]any{
		{
			"notification_id": notificationID,
			"event":           "delivered",
			"email":           email,
			"timestamp":       time.Now().Unix(),
		},
	}

	body, err := json.Marshal(events)
	if err != nil {
		logger.Log().Errorf("[email-webhook] marshal error: %s", err)
		return
	}

	payload := ts + string(body)
	hash := sha256.Sum256([]byte(payload))
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
		logger.Log().Warnf("[email-webhook] %s returned %d", config.EmailWebhookURL, resp.StatusCode)
	} else {
		logger.Log().Debugf("[email-webhook] delivery event sent for notification %s", notificationID)
	}
}
