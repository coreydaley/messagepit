// Package mailtrap implements a Mailtrap Email Sending API stub for local development.
package mailtrap

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
	"github.com/coreydaley/messagepit/internal/storage"
	"github.com/lithammer/shortuuid/v4"
)

type address struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

type mailSendRequest struct {
	From            address           `json:"from"`
	To              []address         `json:"to"`
	CC              []address         `json:"cc"`
	BCC             []address         `json:"bcc"`
	Subject         string            `json:"subject"`
	Text            string            `json:"text"`
	HTML            string            `json:"html"`
	Headers         map[string]string `json:"headers"`
	Category        json.RawMessage   `json:"category,omitempty"`
	CustomVariables json.RawMessage   `json:"custom_variables,omitempty"`
	Attachments     json.RawMessage   `json:"attachments,omitempty"`
}

// reservedHeaders are standard headers that must not be overridden by user-supplied values.
var reservedHeaders = map[string]bool{
	"content-type":              true,
	"content-transfer-encoding": true,
	"mime-version":              true,
	"bcc":                       true,
	"from":                      true,
	"to":                        true,
	"cc":                        true,
	"subject":                   true,
	"date":                      true,
}

// CreateMessage handles POST /api/send (Mailtrap Email Sending API).
func CreateMessage(w http.ResponseWriter, r *http.Request) {
	mailadapter.LimitBody(w, r)

	if !mailadapter.BearerAuth(w, r, config.MailtrapAPIKey, "[mailtrap]") {
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			mailadapter.JSONError(w, http.StatusRequestEntityTooLarge, "request body too large")
		} else {
			mailadapter.JSONError(w, http.StatusBadRequest, "failed to read request body")
		}
		return
	}

	var msg mailSendRequest
	if err := json.Unmarshal(body, &msg); err != nil {
		logger.Log().Warnf("[mailtrap] invalid JSON: %s", err)
		mailadapter.JSONError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if msg.From.Email == "" || msg.Subject == "" || len(msg.To) == 0 {
		mailadapter.JSONError(w, http.StatusBadRequest, "from, to, and subject are required")
		return
	}

	allAddrs := append(append(append([]address{msg.From}, msg.To...), msg.CC...), msg.BCC...)
	for _, a := range allAddrs {
		if a.Email == "" {
			continue
		}
		if _, err := mail.ParseAddress(a.Email); err != nil {
			mailadapter.JSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid email address: %s", a.Email))
			return
		}
	}

	mimeBytes := buildMIME(&msg)
	username := "mailtrap"
	id, err := storage.Store(&mimeBytes, &username)
	if err != nil {
		logger.Log().Errorf("[mailtrap] failed to store email: %s", err)
		mailadapter.JSONError(w, http.StatusInternalServerError, "failed to store message")
		return
	}
	logger.Log().Debugf("[mailtrap] stored email %s", id)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":     true,
		"message_ids": []string{id},
	})
}

func buildMIME(msg *mailSendRequest) []byte {
	var buf bytes.Buffer

	fromStr, _ := mailadapter.FormatAddress(msg.From.Email, msg.From.Name)
	fmt.Fprintf(&buf, "Date: %s\r\n", time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 +0000"))
	fmt.Fprintf(&buf, "From: %s\r\n", fromStr)

	toAddrs := make([]string, 0, len(msg.To))
	for _, a := range msg.To {
		s, _ := mailadapter.FormatAddress(a.Email, a.Name)
		if s != "" {
			toAddrs = append(toAddrs, s)
		}
	}
	fmt.Fprintf(&buf, "To: %s\r\n", strings.Join(toAddrs, ", "))

	if len(msg.CC) > 0 {
		ccAddrs := make([]string, 0, len(msg.CC))
		for _, a := range msg.CC {
			s, _ := mailadapter.FormatAddress(a.Email, a.Name)
			if s != "" {
				ccAddrs = append(ccAddrs, s)
			}
		}
		fmt.Fprintf(&buf, "Cc: %s\r\n", strings.Join(ccAddrs, ", "))
	}

	fmt.Fprintf(&buf, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", msg.Subject))
	fmt.Fprintf(&buf, "MIME-Version: 1.0\r\n")

	for k, v := range msg.Headers {
		if reservedHeaders[strings.ToLower(k)] {
			continue
		}
		fmt.Fprintf(&buf, "%s: %s\r\n", k, mailadapter.SanitizeHeaderValue(v))
	}

	hasText := msg.Text != ""
	hasHTML := msg.HTML != ""

	switch {
	case hasText && hasHTML:
		boundary := "MP" + shortuuid.New()
		fmt.Fprintf(&buf, "Content-Type: multipart/alternative; boundary=%q\r\n\r\n", boundary)
		fmt.Fprintf(&buf, "--%s\r\n", boundary)
		fmt.Fprintf(&buf, "Content-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n", msg.Text)
		fmt.Fprintf(&buf, "--%s\r\n", boundary)
		fmt.Fprintf(&buf, "Content-Type: text/html; charset=UTF-8\r\n\r\n%s\r\n", msg.HTML)
		fmt.Fprintf(&buf, "--%s--\r\n", boundary)
	case hasHTML:
		fmt.Fprintf(&buf, "Content-Type: text/html; charset=UTF-8\r\n\r\n%s\r\n", msg.HTML)
	case hasText:
		fmt.Fprintf(&buf, "Content-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n", msg.Text)
	default:
		fmt.Fprintf(&buf, "Content-Type: text/plain; charset=UTF-8\r\n\r\n\r\n")
	}

	return buf.Bytes()
}
