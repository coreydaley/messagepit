package storage

// BroadcastFunc is called when a storage event should be broadcast to connected
// websocket clients. Set by the server layer at boot; no-op if nil.
var BroadcastFunc func(eventType string, data any)

// WebhookFunc is called when a new message is stored and should be delivered
// to configured outbound webhooks. Set by the server layer at boot; no-op if nil.
var WebhookFunc func(data any)

// BroadcastClientErrorFunc is called when a protocol handler (SMTP, POP3, etc.)
// encounters a client-level error that should be broadcast to connected websocket clients.
// Set by the server layer at boot; no-op if nil.
var BroadcastClientErrorFunc func(severity, protocol, ip, message string)

func broadcast(eventType string, data any) {
	if BroadcastFunc != nil {
		BroadcastFunc(eventType, data)
	}
}

func sendWebhook(data any) {
	if WebhookFunc != nil {
		WebhookFunc(data)
	}
}

// BroadcastClientError notifies connected clients of a protocol-level client error.
func BroadcastClientError(severity, protocol, ip, message string) {
	if BroadcastClientErrorFunc != nil {
		BroadcastClientErrorFunc(severity, protocol, ip, message)
	}
}
