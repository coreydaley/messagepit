// Package twilio implements the Twilio Messages API for SMS ingest.
package twilio

import (
	"crypto/hmac"
	"crypto/sha1" // #nosec G505 -- Twilio's API uses SHA1; we have no choice
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/coreydaley/messagepit/config"
	"github.com/coreydaley/messagepit/internal/logger"
	"github.com/coreydaley/messagepit/internal/storage"
	"github.com/gorilla/mux"
)

// twilioMessageResponse mirrors the shape of a real Twilio Messages API response.
type twilioMessageResponse struct {
	SID         string `json:"sid"`
	AccountSID  string `json:"account_sid"`
	From        string `json:"from"`
	To          string `json:"to"`
	Body        string `json:"body"`
	Status      string `json:"status"`
	Direction   string `json:"direction"`
	NumSegments string `json:"num_segments"`
	Price       string `json:"price"`
	PriceUnit   string `json:"price_unit"`
	ErrorCode   any    `json:"error_code"`
	ErrorMessage any   `json:"error_message"`
	DateCreated string `json:"date_created"`
	DateUpdated string `json:"date_updated"`
	DateSent    any    `json:"date_sent"`
	URI         string `json:"uri"`
}

// CreateMessage handles POST /2010-04-01/Accounts/{AccountSid}/Messages.json
// and stores the incoming SMS in the MessagePit database.
func CreateMessage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	accountSID := vars["AccountSid"]

	if err := r.ParseForm(); err != nil {
		httpError(w, http.StatusBadRequest, "invalid form data")
		return
	}

	// Validate Twilio signature if an auth token is configured
	if config.TwilioAuthToken != "" {
		if !validSignature(r, config.TwilioAuthToken) {
			logger.Log().Warnf("[twilio] invalid signature from %s", r.RemoteAddr)
			httpError(w, http.StatusForbidden, "invalid Twilio signature")
			return
		}
	}

	from := strings.TrimSpace(r.FormValue("From"))
	to := strings.TrimSpace(r.FormValue("To"))
	body := strings.TrimSpace(r.FormValue("Body"))

	if from == "" || to == "" || body == "" {
		httpError(w, http.StatusBadRequest, "From, To and Body are required")
		return
	}

	id, err := storage.StoreSMS(from, to, body, accountSID)
	if err != nil {
		logger.Log().Errorf("[twilio] failed to store SMS: %s", err.Error())
		httpError(w, http.StatusInternalServerError, "failed to store message")
		return
	}

	resp := twilioMessageResponse{
		SID:          id,
		AccountSID:   accountSID,
		From:         from,
		To:           to,
		Body:         body,
		Status:       "queued",
		Direction:    "outbound-api",
		NumSegments:  "1",
		Price:        "0",
		PriceUnit:    "USD",
		ErrorCode:    nil,
		ErrorMessage: nil,
		URI:          fmt.Sprintf("/2010-04-01/Accounts/%s/Messages/%s.json", accountSID, id),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// validSignature validates the X-Twilio-Signature header against the request.
// See https://www.twilio.com/docs/usage/webhooks/webhooks-security
func validSignature(r *http.Request, authToken string) bool {
	signature := r.Header.Get("X-Twilio-Signature")
	if signature == "" {
		return false
	}

	// Build the full URL as Twilio would see it
	scheme := "https"
	if r.TLS == nil {
		scheme = "http"
	}
	fullURL := fmt.Sprintf("%s://%s%s", scheme, r.Host, r.RequestURI)

	// Sort POST params and concatenate key+value pairs
	params := url.Values{}
	for k, vs := range r.PostForm {
		params[k] = vs
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	sb.WriteString(fullURL)
	for _, k := range keys {
		sb.WriteString(k)
		sb.WriteString(params.Get(k))
	}

	mac := hmac.New(sha1.New, []byte(authToken))
	mac.Write([]byte(sb.String())) // #nosec G104
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expected))
}

func httpError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	e := struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  int    `json:"status"`
	}{Code: status, Message: msg, Status: status}
	_ = json.NewEncoder(w).Encode(e)
}
