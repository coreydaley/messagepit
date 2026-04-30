// Package webhookd implements the HTTP webhook capture server.
package webhookd

import (
	"io"
	"maps"
	"net"
	"net/http"
	"strings"

	"github.com/coreydaley/messagepit/internal/logger"
	"github.com/coreydaley/messagepit/internal/storage"
)

const maxBodySize = 10 * 1024 * 1024 // 10MB

// CaptureHandler is a catch-all HTTP handler that captures incoming requests.
func CaptureHandler(w http.ResponseWriter, r *http.Request) {
	// Respond to CORS preflight without storing anything
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "*")
	w.Header().Set("Access-Control-Allow-Headers", "*")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	body, _ := io.ReadAll(io.LimitReader(r.Body, maxBodySize))
	_ = r.Body.Close()

	contentType := r.Header.Get("Content-Type")
	if i := strings.Index(contentType, ";"); i != -1 {
		contentType = strings.TrimSpace(contentType[:i])
	}

	sourceIP, _, _ := net.SplitHostPort(r.RemoteAddr)

	headers := make(map[string][]string, len(r.Header))
	maps.Copy(headers, r.Header)

	_, err := storage.StoreWebhook(r.Method, r.URL.Path, r.URL.RawQuery, headers, body, contentType, sourceIP)
	if err != nil {
		logger.Log().Errorf("[webhookd] failed to store webhook: %s", err.Error())
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
