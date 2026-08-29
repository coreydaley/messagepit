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
	"strconv"
	"strings"
	"time"

	"github.com/coreydaley/messagepit/config"
	"github.com/coreydaley/messagepit/internal/logger"
	"github.com/coreydaley/messagepit/internal/storage"
	"github.com/gorilla/mux"
)

// apiVersion is the Twilio REST API version MessagePit emulates.
const apiVersion = "2010-04-01"

// twilioMessageResponse mirrors the shape of a real Twilio Messages API response.
type twilioMessageResponse struct {
	SID          string `json:"sid"`
	AccountSID   string `json:"account_sid"`
	From         string `json:"from"`
	To           string `json:"to"`
	Body         string `json:"body"`
	Status       string `json:"status"`
	Direction    string `json:"direction"`
	NumSegments  string `json:"num_segments"`
	Price        string `json:"price"`
	PriceUnit    string `json:"price_unit"`
	ErrorCode    any    `json:"error_code"`
	ErrorMessage any    `json:"error_message"`
	DateCreated  string `json:"date_created"`
	DateUpdated  string `json:"date_updated"`
	DateSent     any    `json:"date_sent,omitempty"`
	APIVersion   string `json:"api_version"`
	URI          string `json:"uri"`
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

	// Validate using HTTP Basic Auth (username=AccountSID, password=AuthToken) —
	// the pattern the Twilio Messages API uses for outbound sends.
	if config.TwilioAuthToken != "" {
		_, password, ok := r.BasicAuth()
		if !ok || password != config.TwilioAuthToken {
			logger.Log().Warnf("[twilio] invalid credentials from %s", r.RemoteAddr)
			httpError(w, http.StatusForbidden, "invalid credentials")
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

	// Twilio rejects its reserved magic test numbers before queueing, so no
	// message is stored and no status callback ever fires.
	if e := magicAPIError(from, to); e != nil {
		logger.Log().Debugf("[twilio] magic number rejection %d for %s -> %s", e.Code, from, to)
		twilioError(w, http.StatusBadRequest, e.Code, e.Message)
		return
	}

	id, err := storage.StoreSMS(from, to, body, accountSID)
	if err != nil {
		logger.Log().Errorf("[twilio] failed to store SMS: %s", err.Error())
		httpError(w, http.StatusInternalServerError, "failed to store message")
		return
	}

	// Prefer the per-request StatusCallback URL (mirrors real Twilio behaviour),
	// fall back to the globally configured MP_TWILIO_WEBHOOK_URL.
	callbackURL := strings.TrimSpace(r.FormValue("StatusCallback"))
	if callbackURL == "" {
		callbackURL = config.TwilioWebhookURL
	}
	fireSMSCallback(id, accountSID, to, from, callbackURL)

	now := time.Now().UTC().Format(time.RFC1123Z)
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
		DateCreated:  now,
		DateUpdated:  now,
		APIVersion:   apiVersion,
		URI:          fmt.Sprintf("/2010-04-01/Accounts/%s/Messages/%s.json", accountSID, id),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// apiError is a Twilio API-level rejection, returned before the message is
// ever queued. Real Twilio produces these for its reserved magic test numbers.
type apiError struct {
	Code    int
	Message string
}

// magicFromErrors mirrors Twilio's reserved "From" test numbers.
// See https://www.twilio.com/docs/iam/test-credentials
var magicFromErrors = map[string]apiError{
	"15005550001": {21212, "The 'From' number %s is not a valid phone number, shortcode, or alphanumeric sender ID."},
	"15005550007": {21606, "The 'From' phone number %s is not a valid, SMS-capable inbound phone number or short code for your account."},
	"15005550008": {21611, "This 'From' number has exceeded the maximum number of queued messages."},
}

// magicToErrors mirrors Twilio's reserved "To" test numbers.
var magicToErrors = map[string]apiError{
	"15005550001": {21211, "The 'To' number %s is not a valid phone number."},
	"15005550002": {21612, "The 'To' phone number %s is not currently reachable via SMS."},
	"15005550003": {21408, "Permission to send an SMS has not been enabled for the region indicated by the 'To' number %s."},
	"15005550004": {21610, "Attempt to send to unsubscribed recipient %s."},
	"15005550009": {21614, "'To' number %s is not a valid mobile number."},
}

// statusStep is one hop in a message's delivery status progression.
type statusStep struct {
	Status    string
	ErrorCode int
	ErrorMsg  string
}

// deliveredProgression is the happy path every message follows unless a
// MessagePit magic number selects a failure branch.
var deliveredProgression = []statusStep{
	{Status: "queued"},
	{Status: "sent"},
	{Status: "delivered"},
}

// magicToProgressions are MessagePit extensions living in Twilio's reserved
// test range. They exercise the delivery-failure branches, which real Twilio
// has no way to trigger on demand.
var magicToProgressions = map[string][]statusStep{
	// Rejected outright — never leaves Twilio.
	"15005550010": {
		{Status: "queued"},
		{Status: "failed", ErrorCode: 30008, ErrorMsg: "Unknown error"},
	},
	// Handed to the carrier, then rejected downstream.
	"15005550011": {
		{Status: "queued"},
		{Status: "sent"},
		{Status: "undelivered", ErrorCode: 30003, ErrorMsg: "Unreachable destination handset"},
	},
	"15005550012": {
		{Status: "queued"},
		{Status: "sent"},
		{Status: "undelivered", ErrorCode: 30005, ErrorMsg: "Unknown destination handset"},
	},
	"15005550013": {
		{Status: "queued"},
		{Status: "sent"},
		{Status: "undelivered", ErrorCode: 30006, ErrorMsg: "Landline or unreachable carrier"},
	},
	// Carrier never confirms — the progression stalls at "sent".
	"15005550014": {
		{Status: "queued"},
		{Status: "sent"},
	},
}

// magicKey reduces a phone number to its digits so the magic-number lookups
// tolerate the formatting variations a client might send — "+15005550001",
// "15005550001", or "+1 (500) 555-0001" all resolve to the same entry.
func magicKey(number string) string {
	var sb strings.Builder
	for _, r := range number {
		if r >= '0' && r <= '9' {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// magicAPIError returns the Twilio API rejection for a reserved test number,
// or nil when the pair is deliverable. Real Twilio only honours these under
// test credentials; MessagePit is only ever a test environment, so they are
// always active.
func magicAPIError(from, to string) *apiError {
	if e, ok := magicFromErrors[magicKey(from)]; ok {
		return &apiError{Code: e.Code, Message: fmt.Sprintf(e.Message, from)}
	}
	if e, ok := magicToErrors[magicKey(to)]; ok {
		return &apiError{Code: e.Code, Message: fmt.Sprintf(e.Message, to)}
	}
	return nil
}

// progressionFor returns the status callback sequence for a destination number.
func progressionFor(to string) []statusStep {
	if p, ok := magicToProgressions[magicKey(to)]; ok {
		return p
	}
	return deliveredProgression
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

// fireSMSCallback replays the message's delivery status progression to
// callbackURL — queued → sent → delivered, or a failure branch selected by a
// MessagePit magic destination number. The call is non-blocking; errors are
// logged but not propagated.
func fireSMSCallback(sid, accountSID, to, from, callbackURL string) {
	if callbackURL == "" {
		return
	}

	steps := progressionFor(to)

	go func() {
		for i, step := range steps {
			if i > 0 && config.TwilioCallbackDelay > 0 {
				time.Sleep(config.TwilioCallbackDelay)
			}
			postStatusCallback(sid, accountSID, to, from, callbackURL, step)
		}
	}()
}

// postStatusCallback delivers a single status callback.
func postStatusCallback(sid, accountSID, to, from, callbackURL string, step statusStep) {
	params := url.Values{
		"MessageSid":    {sid},
		"SmsSid":        {sid},
		"MessageStatus": {step.Status},
		"SmsStatus":     {step.Status},
		"To":            {to},
		"From":          {from},
		"AccountSid":    {accountSID},
		"ApiVersion":    {apiVersion},
	}
	if step.ErrorCode != 0 {
		params.Set("ErrorCode", strconv.Itoa(step.ErrorCode))
		params.Set("ErrorMessage", step.ErrorMsg)
	}

	req, err := http.NewRequest("POST", callbackURL, strings.NewReader(params.Encode()))
	if err != nil {
		logger.Log().Errorf("[twilio-callback] failed to build request: %s", err)
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "TwilioProxy/1.1")
	if config.TwilioAuthToken != "" {
		req.Header.Set("X-Twilio-Signature", smsCallbackSignature(callbackURL, params, config.TwilioAuthToken))
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logger.Log().Errorf("[twilio-callback] error: %s", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		logger.Log().Warnf("[twilio-callback] %s returned %d for status %q", callbackURL, resp.StatusCode, step.Status)
	} else {
		logger.Log().Debugf("[twilio-callback] %s status sent for %s", step.Status, sid)
	}
}

// smsCallbackSignature computes the Twilio request signature for an outgoing status callback.
func smsCallbackSignature(rawURL string, params url.Values, authToken string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	sb.WriteString(rawURL)
	for _, k := range keys {
		sb.WriteString(k)
		sb.WriteString(params.Get(k))
	}
	mac := hmac.New(sha1.New, []byte(authToken)) // #nosec G401
	mac.Write([]byte(sb.String()))               // #nosec G104
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// twilioError writes a Twilio API error response carrying a real Twilio error code.
func twilioError(w http.ResponseWriter, httpStatus, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	e := struct {
		Code     int    `json:"code"`
		Message  string `json:"message"`
		MoreInfo string `json:"more_info"`
		Status   int    `json:"status"`
	}{
		Code:     code,
		Message:  msg,
		MoreInfo: fmt.Sprintf("https://www.twilio.com/docs/errors/%d", code),
		Status:   httpStatus,
	}
	_ = json.NewEncoder(w).Encode(e)
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
