package apiv1

import (
	"encoding/json"
	"net/http"

	"github.com/coreydaley/messagepit/internal/storage"
	"github.com/gorilla/mux"
)

// SMSMessagesSummary is the response shape for the SMS message list endpoint.
type SMSMessagesSummary struct {
	Total    uint64                    `json:"total"`
	Unread   uint64                    `json:"unread"`
	Start    int                       `json:"start"`
	Messages []storage.SMSMessageSummary `json:"messages"`
}

// GetSMSMessages returns a paginated list of SMS messages as JSON.
func GetSMSMessages(w http.ResponseWriter, r *http.Request) {
	// swagger:route GET /api/v1/sms/messages sms GetSMSMessagesParams
	//
	// # List SMS messages
	//
	// Returns SMS messages ordered from newest to oldest.
	//
	//	Produces:
	//	  - application/json
	//
	//	Schemes: http, https
	//
	//	Responses:
	//	  200: SMSMessagesSummaryResponse
	//    400: ErrorResponse
	start, _, limit := getStartLimit(r)

	messages, err := storage.ListSMS(start, limit)
	if err != nil {
		httpError(w, err.Error())
		return
	}

	stats, err := storage.GetSMSMailboxStats()
	if err != nil {
		httpError(w, err.Error())
		return
	}

	resp := SMSMessagesSummary{
		Total:    stats.Total,
		Unread:   stats.Unread,
		Start:    start,
		Messages: messages,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		httpError(w, err.Error())
	}
}

// GetSMSMessage returns a single SMS message as JSON.
func GetSMSMessage(w http.ResponseWriter, r *http.Request) {
	// swagger:route GET /api/v1/sms/message/{id} sms GetSMSMessageParams
	//
	// # Get SMS message
	//
	// Returns a single SMS message.
	//
	//	Produces:
	//	  - application/json
	//
	//	Schemes: http, https
	//
	//	Responses:
	//	  200: SMSMessageResponse
	//	  404: NotFoundResponse
	vars := mux.Vars(r)
	id := vars["id"]

	msg, err := storage.GetSMS(id)
	if err != nil {
		fourOFour(w)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(msg); err != nil {
		httpError(w, err.Error())
	}
}

// MarkSMSRead marks an SMS message as read.
func MarkSMSRead(w http.ResponseWriter, r *http.Request) {
	// swagger:route PUT /api/v1/sms/message/{id}/read sms MarkSMSReadParams
	//
	// # Mark SMS message read
	//
	// Marks an SMS message as read.
	//
	//	Produces:
	//	  - application/json
	//
	//	Schemes: http, https
	//
	//	Responses:
	//	  200: OKResponse
	//	  404: NotFoundResponse
	vars := mux.Vars(r)
	id := vars["id"]

	if err := storage.MarkSMSRead([]string{id}); err != nil {
		httpError(w, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"read": true})
}

// DeleteSMSMessage deletes a single SMS message.
func DeleteSMSMessage(w http.ResponseWriter, r *http.Request) {
	// swagger:route DELETE /api/v1/sms/message/{id} sms DeleteSMSMessageParams
	//
	// # Delete SMS message
	//
	// Deletes a single SMS message.
	//
	//	Produces:
	//	  - text/plain
	//
	//	Schemes: http, https
	//
	//	Responses:
	//	  200: OKResponse
	//	  404: NotFoundResponse
	vars := mux.Vars(r)
	id := vars["id"]

	if err := storage.DeleteSMS(id); err != nil {
		httpError(w, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
}

// DeleteAllSMS deletes all SMS messages.
func DeleteAllSMS(w http.ResponseWriter, r *http.Request) {
	// swagger:route DELETE /api/v1/sms/messages sms DeleteAllSMSParams
	//
	// # Delete all SMS messages
	//
	// Deletes all SMS messages.
	//
	//	Produces:
	//	  - text/plain
	//
	//	Schemes: http, https
	//
	//	Responses:
	//	  200: OKResponse
	if err := storage.DeleteAllSMS(); err != nil {
		httpError(w, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
}
