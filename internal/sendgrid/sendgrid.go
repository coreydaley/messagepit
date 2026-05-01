// Package sendgrid implements a SendGrid v3 Mail Send API stub for local development.
package sendgrid

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/coreydaley/messagepit/config"
	"github.com/coreydaley/messagepit/internal/logger"
	"github.com/coreydaley/messagepit/internal/storage"
	"github.com/lithammer/shortuuid/v4"
)

// address mirrors SendGrid's {"email":"...","name":"..."} shape.
type address struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

func (a address) format() string {
	if a.Name != "" {
		return fmt.Sprintf("%s <%s>", a.Name, a.Email)
	}
	return a.Email
}

type content struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type personalization struct {
	To         []address         `json:"to"`
	CC         []address         `json:"cc"`
	BCC        []address         `json:"bcc"`
	CustomArgs map[string]string `json:"custom_args"`
}

type mailSendRequest struct {
	From             address           `json:"from"`
	Subject          string            `json:"subject"`
	Personalizations []personalization `json:"personalizations"`
	Content          []content         `json:"content"`
	CustomArgs       map[string]string `json:"custom_args"`
	Headers          map[string]string `json:"headers"`
}

// CreateMessage handles POST /v3/mail/send (SendGrid v3 Mail Send API).
// It stores the email in the MessagePit mailbox and fires the email delivery
// webhook if an X-Notification-Id is found in the message custom_args.
func CreateMessage(w http.ResponseWriter, r *http.Request) {
	if config.SendGridAPIKey != "" {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") || strings.TrimPrefix(authHeader, "Bearer ") != config.SendGridAPIKey {
			logger.Log().Warnf("[sendgrid] invalid API key from %s", r.RemoteAddr)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var msg mailSendRequest
	if err := json.Unmarshal(body, &msg); err != nil {
		logger.Log().Warnf("[sendgrid] invalid JSON: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if msg.From.Email == "" || msg.Subject == "" || len(msg.Personalizations) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	username := "sendgrid"

	for _, p := range msg.Personalizations {
		// Merge custom_args: message-level first, personalization overrides.
		merged := make(map[string]string, len(msg.CustomArgs)+len(p.CustomArgs))
		for k, v := range msg.CustomArgs {
			merged[k] = v
		}
		for k, v := range p.CustomArgs {
			merged[k] = v
		}
		notificationID := merged["notification_id"]

		for _, to := range p.To {
			mime := buildMIME(&msg, &p, to, notificationID)
			id, err := storage.Store(&mime, &username)
			if err != nil {
				logger.Log().Errorf("[sendgrid] failed to store email for %s: %s", to.Email, err)
			} else {
				logger.Log().Debugf("[sendgrid] stored email %s for %s", id, to.Email)
			}
		}
	}

	// 202 Accepted — no body, matching real SendGrid.
	w.WriteHeader(http.StatusAccepted)
}

// buildMIME constructs a minimal RFC 2822 MIME message from a v3 API payload.
// The X-Notification-Id header is set so the existing email webhook code picks it up.
func buildMIME(msg *mailSendRequest, p *personalization, to address, notificationID string) []byte {
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "Date: %s\r\n", time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 +0000"))
	fmt.Fprintf(&buf, "From: %s\r\n", msg.From.format())
	fmt.Fprintf(&buf, "To: %s\r\n", to.format())

	if len(p.CC) > 0 {
		addrs := make([]string, len(p.CC))
		for i, a := range p.CC {
			addrs[i] = a.format()
		}
		fmt.Fprintf(&buf, "Cc: %s\r\n", strings.Join(addrs, ", "))
	}

	fmt.Fprintf(&buf, "Subject: %s\r\n", msg.Subject)
	fmt.Fprintf(&buf, "MIME-Version: 1.0\r\n")

	for k, v := range msg.Headers {
		fmt.Fprintf(&buf, "%s: %s\r\n", k, v)
	}

	if notificationID != "" {
		fmt.Fprintf(&buf, "X-Notification-Id: %s\r\n", notificationID)
	}

	switch len(msg.Content) {
	case 0:
		fmt.Fprintf(&buf, "Content-Type: text/plain; charset=UTF-8\r\n\r\n\r\n")
	case 1:
		fmt.Fprintf(&buf, "Content-Type: %s; charset=UTF-8\r\n\r\n%s\r\n", msg.Content[0].Type, msg.Content[0].Value)
	default:
		boundary := "MP" + shortuuid.New()
		fmt.Fprintf(&buf, "Content-Type: multipart/alternative; boundary=%q\r\n\r\n", boundary)
		for _, c := range msg.Content {
			fmt.Fprintf(&buf, "--%s\r\n", boundary)
			fmt.Fprintf(&buf, "Content-Type: %s; charset=UTF-8\r\n\r\n", c.Type)
			fmt.Fprintf(&buf, "%s\r\n", c.Value)
		}
		fmt.Fprintf(&buf, "--%s--\r\n", boundary)
	}

	return buf.Bytes()
}
