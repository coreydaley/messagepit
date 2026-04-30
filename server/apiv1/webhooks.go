package apiv1

import (
	"encoding/json"
	"net/http"

	"github.com/coreydaley/messagepit/internal/storage"
	"github.com/gorilla/mux"
)

// WebhookRequestsSummary is the response shape for the webhook list endpoint.
type WebhookRequestsSummary struct {
	Total    uint64                          `json:"total"`
	Unread   uint64                          `json:"unread"`
	Start    int                             `json:"start"`
	Messages []storage.WebhookRequestSummary `json:"messages"`
}

// GetWebhooks returns a paginated list of captured webhook requests.
func GetWebhooks(w http.ResponseWriter, r *http.Request) {
	start, _, limit := getStartLimit(r)

	messages, err := storage.ListWebhooks(start, limit)
	if err != nil {
		httpError(w, err.Error())
		return
	}

	stats, err := storage.GetWebhookMailboxStats()
	if err != nil {
		httpError(w, err.Error())
		return
	}

	if messages == nil {
		messages = []storage.WebhookRequestSummary{}
	}

	resp := WebhookRequestsSummary{
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

// GetWebhook returns a single captured webhook request and marks it as read.
func GetWebhook(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	msg, err := storage.GetWebhook(id)
	if err != nil {
		fourOFour(w)
		return
	}

	if !msg.Read {
		_ = storage.MarkWebhookRead(id)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(msg); err != nil {
		httpError(w, err.Error())
	}
}

// DeleteWebhook deletes a single captured webhook request.
func DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	if err := storage.DeleteWebhook(id); err != nil {
		httpError(w, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
}

// DeleteAllWebhooks deletes all captured webhook requests.
func DeleteAllWebhooks(w http.ResponseWriter, r *http.Request) {
	if err := storage.DeleteAllWebhooks(); err != nil {
		httpError(w, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
}
