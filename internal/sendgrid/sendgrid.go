// Package sendgrid implements a SendGrid v3 Mail Send API stub for local development.
package sendgrid

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/coreydaley/messagepit/config"
	"github.com/coreydaley/messagepit/internal/logger"
	"github.com/coreydaley/messagepit/internal/mailadapter"
	"github.com/coreydaley/messagepit/internal/mailevents"
	"github.com/coreydaley/messagepit/internal/storage"
	"github.com/lithammer/shortuuid/v4"
)

// address mirrors SendGrid's {"email":"...","name":"..."} shape.
type address struct {
	Email string `json:"email"`
	Name  string `json:"name"`
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
	Categories       []string          `json:"categories"`
	Headers          map[string]string `json:"headers"`
}

// jsonError writes an error response in SendGrid's {"errors":[{"message":"..."}]} shape.
func jsonError(w http.ResponseWriter, status int, msg string) {
	type item struct {
		Message string `json:"message"`
	}
	type body struct {
		Errors []item `json:"errors"`
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body{Errors: []item{{Message: msg}}})
}

// CreateMessage handles POST /v3/mail/send (SendGrid v3 Mail Send API).
// It stores the email in the MessagePit mailbox; the storage layer then replays
// the SendGrid event lifecycle to the configured event webhook. Categories and
// custom_args are handed over via the X-SMTPAPI header, the same mechanism real
// SendGrid uses to carry them across an SMTP hop.
func CreateMessage(w http.ResponseWriter, r *http.Request) {
	mailadapter.LimitBody(w, r)

	if !mailadapter.BearerAuth(w, r, config.SendGridAPIKey, "[sendgrid]") {
		return
	}

	b, err := io.ReadAll(r.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			jsonError(w, http.StatusRequestEntityTooLarge, "request body too large")
		} else {
			jsonError(w, http.StatusBadRequest, "failed to read request body")
		}
		return
	}

	var msg mailSendRequest
	if err := json.Unmarshal(b, &msg); err != nil {
		logger.Log().Warnf("[sendgrid] invalid JSON: %s", err)
		jsonError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if msg.From.Email == "" || msg.Subject == "" || len(msg.Personalizations) == 0 {
		jsonError(w, http.StatusBadRequest, "from, subject, and personalizations are required")
		return
	}

	if _, err := mail.ParseAddress(msg.From.Email); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid from address")
		return
	}

	for i, p := range msg.Personalizations {
		if len(p.To) == 0 {
			jsonError(w, http.StatusBadRequest, fmt.Sprintf("personalizations[%d].to is required", i))
			return
		}
	}

	for i, c := range msg.Content {
		if c.Type == "" {
			jsonError(w, http.StatusBadRequest, fmt.Sprintf("content[%d].type is required (minLength=1)", i))
			return
		}
		if c.Value == "" {
			jsonError(w, http.StatusBadRequest, fmt.Sprintf("content[%d].value is required (minLength=1)", i))
			return
		}
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
		for _, to := range p.To {
			mimeBytes := buildMIME(&msg, &p, to, merged)
			id, err := storage.Store(&mimeBytes, &username)
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
// Categories and custom_args are serialised into X-SMTPAPI so the storage layer
// can attach them to the event webhook; X-Notification-Id is still emitted for
// backwards compatibility with existing integrations.
func buildMIME(msg *mailSendRequest, p *personalization, to address, customArgs map[string]string) []byte {
	var buf bytes.Buffer

	fromStr, _ := mailadapter.FormatAddress(msg.From.Email, msg.From.Name)
	toStr, _ := mailadapter.FormatAddress(to.Email, to.Name)

	fmt.Fprintf(&buf, "Date: %s\r\n", time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 +0000"))
	fmt.Fprintf(&buf, "From: %s\r\n", fromStr)
	fmt.Fprintf(&buf, "To: %s\r\n", toStr)

	if len(p.CC) > 0 {
		addrs := make([]string, len(p.CC))
		for i, a := range p.CC {
			addrs[i], _ = mailadapter.FormatAddress(a.Email, a.Name)
		}
		fmt.Fprintf(&buf, "Cc: %s\r\n", strings.Join(addrs, ", "))
	}

	fmt.Fprintf(&buf, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", msg.Subject))
	fmt.Fprintf(&buf, "MIME-Version: 1.0\r\n")

	for k, v := range msg.Headers {
		fmt.Fprintf(&buf, "%s: %s\r\n", k, mailadapter.SanitizeHeaderValue(v))
	}

	if notificationID := customArgs["notification_id"]; notificationID != "" {
		fmt.Fprintf(&buf, "X-Notification-Id: %s\r\n", mailadapter.SanitizeHeaderValue(notificationID))
	}

	if scenario := customArgs["mp_scenario"]; scenario != "" {
		fmt.Fprintf(&buf, "%s: %s\r\n", mailevents.ScenarioHeader, mailadapter.SanitizeHeaderValue(scenario))
	}

	if smtpAPI := mailevents.BuildSMTPAPI(msg.Categories, customArgs); smtpAPI != "" {
		fmt.Fprintf(&buf, "%s: %s\r\n", mailevents.SMTPAPIHeader, smtpAPI)
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
