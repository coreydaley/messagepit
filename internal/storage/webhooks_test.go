package storage

import (
	"testing"
)

func TestWebhookStore(t *testing.T) {
	setup("")
	defer Close()

	headers := map[string][]string{
		"Content-Type": {"application/json"},
		"X-Custom":     {"value"},
	}

	id, err := StoreWebhook("POST", "/test", "", headers, []byte(`{"key":"val"}`), "application/json", "127.0.0.1")
	if err != nil {
		t.Fatalf("StoreWebhook: %v", err)
	}
	if id == "" {
		t.Fatal("StoreWebhook returned empty ID")
	}

	stats, err := GetWebhookMailboxStats()
	if err != nil {
		t.Fatalf("GetWebhookMailboxStats: %v", err)
	}
	assertEqual(t, stats.Total, uint64(1), "total after insert")
	assertEqual(t, stats.Unread, uint64(1), "unread after insert")
}

func TestWebhookStatsEmptyTable(t *testing.T) {
	setup("")
	defer Close()

	stats, err := GetWebhookMailboxStats()
	if err != nil {
		t.Fatalf("GetWebhookMailboxStats on empty table: %v", err)
	}
	assertEqual(t, stats.Total, uint64(0), "total on empty table")
	assertEqual(t, stats.Unread, uint64(0), "unread on empty table")
}

func TestWebhookGet(t *testing.T) {
	setup("")
	defer Close()

	headers := map[string][]string{
		"Content-Type": {"application/json"},
	}
	body := []byte(`{"hello":"world"}`)

	id, err := StoreWebhook("POST", "/api/hook", "foo=bar", headers, body, "application/json", "10.0.0.1")
	if err != nil {
		t.Fatalf("StoreWebhook: %v", err)
	}

	msg, err := GetWebhook(id)
	if err != nil {
		t.Fatalf("GetWebhook: %v", err)
	}
	assertEqual(t, msg.ID, id, "ID")
	assertEqual(t, msg.Method, "POST", "Method")
	assertEqual(t, msg.Path, "/api/hook", "Path")
	assertEqual(t, msg.Query, "foo=bar", "Query")
	assertEqual(t, msg.Body, `{"hello":"world"}`, "Body")
	assertEqual(t, msg.BodySize, int64(len(body)), "BodySize")
	assertEqual(t, msg.ContentType, "application/json", "ContentType")
	assertEqual(t, msg.SourceIP, "10.0.0.1", "SourceIP")
	assertEqual(t, msg.Read, false, "Read should be false after store")

	if len(msg.Headers["Content-Type"]) == 0 || msg.Headers["Content-Type"][0] != "application/json" {
		t.Fatalf("expected Content-Type header, got %v", msg.Headers)
	}
}

func TestWebhookGetNotFound(t *testing.T) {
	setup("")
	defer Close()

	_, err := GetWebhook("nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent ID, got nil")
	}
}

func TestWebhookGetNoBody(t *testing.T) {
	setup("")
	defer Close()

	id, err := StoreWebhook("GET", "/ping", "", nil, nil, "", "192.168.1.1")
	if err != nil {
		t.Fatalf("StoreWebhook with no body: %v", err)
	}

	msg, err := GetWebhook(id)
	if err != nil {
		t.Fatalf("GetWebhook: %v", err)
	}
	assertEqual(t, msg.Method, "GET", "Method")
	assertEqual(t, msg.Body, "", "Body should be empty")
	assertEqual(t, msg.BodySize, int64(0), "BodySize should be zero")
}

func TestWebhookList(t *testing.T) {
	setup("")
	defer Close()

	methods := []string{"GET", "POST", "PUT", "PATCH", "DELETE"}
	for i, m := range methods {
		path := "/path/" + string(rune('a'+i))
		if _, err := StoreWebhook(m, path, "", nil, nil, "", "127.0.0.1"); err != nil {
			t.Fatalf("StoreWebhook: %v", err)
		}
	}

	msgs, err := ListWebhooks(0, 10)
	if err != nil {
		t.Fatalf("ListWebhooks: %v", err)
	}
	assertEqual(t, len(msgs), 5, "list count")

	// results are newest-first
	assertEqual(t, msgs[0].Method, "DELETE", "first result should be newest")
}

func TestWebhookListPagination(t *testing.T) {
	setup("")
	defer Close()

	for i := range 10 {
		path := "/path/" + string(rune('a'+i))
		if _, err := StoreWebhook("POST", path, "", nil, []byte("body"), "application/json", "127.0.0.1"); err != nil {
			t.Fatalf("StoreWebhook: %v", err)
		}
	}

	page1, err := ListWebhooks(0, 3)
	if err != nil {
		t.Fatalf("ListWebhooks page1: %v", err)
	}
	assertEqual(t, len(page1), 3, "page 1 count")

	page2, err := ListWebhooks(3, 3)
	if err != nil {
		t.Fatalf("ListWebhooks page2: %v", err)
	}
	assertEqual(t, len(page2), 3, "page 2 count")

	if page1[0].ID == page2[0].ID {
		t.Fatal("page 1 and page 2 returned the same message")
	}
}

func TestWebhookSnippet(t *testing.T) {
	setup("")
	defer Close()

	// body longer than 250 chars — snippet must be truncated
	longBody := make([]byte, 300)
	for i := range longBody {
		longBody[i] = 'x'
	}

	id, err := StoreWebhook("POST", "/large", "", nil, longBody, "text/plain", "127.0.0.1")
	if err != nil {
		t.Fatalf("StoreWebhook: %v", err)
	}

	msgs, err := ListWebhooks(0, 10)
	if err != nil {
		t.Fatalf("ListWebhooks: %v", err)
	}

	var found bool
	for _, m := range msgs {
		if m.ID == id {
			found = true
			if len(m.Snippet) != 250 {
				t.Fatalf("expected snippet length 250, got %d", len(m.Snippet))
			}
		}
	}
	if !found {
		t.Fatal("stored webhook not found in list")
	}
}

func TestWebhookMarkRead(t *testing.T) {
	setup("")
	defer Close()

	id, err := StoreWebhook("POST", "/hook", "", nil, []byte("data"), "text/plain", "127.0.0.1")
	if err != nil {
		t.Fatalf("StoreWebhook: %v", err)
	}

	stats, _ := GetWebhookMailboxStats()
	assertEqual(t, stats.Unread, uint64(1), "unread before mark read")

	if err := MarkWebhookRead(id); err != nil {
		t.Fatalf("MarkWebhookRead: %v", err)
	}

	msg, err := GetWebhook(id)
	if err != nil {
		t.Fatalf("GetWebhook after mark read: %v", err)
	}
	assertEqual(t, msg.Read, true, "Read flag after MarkWebhookRead")

	stats, _ = GetWebhookMailboxStats()
	assertEqual(t, stats.Unread, uint64(0), "unread after mark read")
}

func TestWebhookDelete(t *testing.T) {
	setup("")
	defer Close()

	id, err := StoreWebhook("DELETE", "/resource/1", "", nil, nil, "", "127.0.0.1")
	if err != nil {
		t.Fatalf("StoreWebhook: %v", err)
	}

	if err := DeleteWebhook(id); err != nil {
		t.Fatalf("DeleteWebhook: %v", err)
	}

	stats, _ := GetWebhookMailboxStats()
	assertEqual(t, stats.Total, uint64(0), "total after delete")

	_, err = GetWebhook(id)
	if err == nil {
		t.Fatal("expected error fetching deleted webhook, got nil")
	}
}

func TestWebhookDeleteAll(t *testing.T) {
	setup("")
	defer Close()

	for range 5 {
		if _, err := StoreWebhook("POST", "/hook", "", nil, []byte("body"), "text/plain", "127.0.0.1"); err != nil {
			t.Fatalf("StoreWebhook: %v", err)
		}
	}

	stats, _ := GetWebhookMailboxStats()
	assertEqual(t, stats.Total, uint64(5), "total before delete all")

	if err := DeleteAllWebhooks(); err != nil {
		t.Fatalf("DeleteAllWebhooks: %v", err)
	}

	stats, _ = GetWebhookMailboxStats()
	assertEqual(t, stats.Total, uint64(0), "total after delete all")
	assertEqual(t, stats.Unread, uint64(0), "unread after delete all")
}

func TestSearchWebhooks(t *testing.T) {
	setup("")
	defer Close()

	_, _ = StoreWebhook("POST", "/api/orders", "", nil, []byte(`{"item":"book"}`), "application/json", "10.0.0.1")
	_, _ = StoreWebhook("GET", "/api/health", "status=ok", nil, nil, "", "10.0.0.2")
	_, _ = StoreWebhook("DELETE", "/api/orders/42", "", nil, nil, "", "192.168.1.5")

	// match on path
	results, total, err := SearchWebhooks("/api/orders", 0, 10)
	if err != nil {
		t.Fatalf("SearchWebhooks by path: %v", err)
	}
	assertEqual(t, total, 2, "total matches for '/api/orders'")
	assertEqual(t, len(results), 2, "returned results for '/api/orders'")

	// match on method
	results, total, err = SearchWebhooks("DELETE", 0, 10)
	if err != nil {
		t.Fatalf("SearchWebhooks by method: %v", err)
	}
	assertEqual(t, total, 1, "total matches for DELETE method")
	assertEqual(t, len(results), 1, "returned results for DELETE method")
	assertEqual(t, results[0].Method, "DELETE", "method field")

	// match on query string
	results, total, err = SearchWebhooks("status=ok", 0, 10)
	if err != nil {
		t.Fatalf("SearchWebhooks by query: %v", err)
	}
	assertEqual(t, total, 1, "total matches for query string")
	assertEqual(t, results[0].Path, "/api/health", "path for query match")

	// match on source IP
	results, total, err = SearchWebhooks("192.168", 0, 10)
	if err != nil {
		t.Fatalf("SearchWebhooks by IP: %v", err)
	}
	assertEqual(t, total, 1, "total matches for IP prefix")
	assertEqual(t, results[0].SourceIP, "192.168.1.5", "source IP")

	// match on body snippet
	results, total, err = SearchWebhooks("book", 0, 10)
	if err != nil {
		t.Fatalf("SearchWebhooks by snippet: %v", err)
	}
	assertEqual(t, total, 1, "total matches for snippet content")
	assertEqual(t, results[0].Path, "/api/orders", "path for snippet match")

	// no match
	results, total, err = SearchWebhooks("zzz_nomatch", 0, 10)
	if err != nil {
		t.Fatalf("SearchWebhooks no match: %v", err)
	}
	assertEqual(t, total, 0, "total for no-match query")
	assertEqual(t, len(results), 0, "results for no-match query")
}

func TestSearchWebhooksPagination(t *testing.T) {
	setup("")
	defer Close()

	for i := range 10 {
		path := "/paginate/" + string(rune('a'+i))
		if _, err := StoreWebhook("POST", path, "", nil, []byte("data"), "text/plain", "127.0.0.1"); err != nil {
			t.Fatalf("StoreWebhook: %v", err)
		}
	}

	page1, total, err := SearchWebhooks("/paginate/", 0, 3)
	if err != nil {
		t.Fatalf("SearchWebhooks page1: %v", err)
	}
	assertEqual(t, total, 10, "total across pages")
	assertEqual(t, len(page1), 3, "page 1 count")

	page2, _, err := SearchWebhooks("/paginate/", 3, 3)
	if err != nil {
		t.Fatalf("SearchWebhooks page2: %v", err)
	}
	assertEqual(t, len(page2), 3, "page 2 count")

	if page1[0].ID == page2[0].ID {
		t.Fatal("page 1 and page 2 returned the same webhook")
	}
}

func TestWebhookHTTPMethods(t *testing.T) {
	setup("")
	defer Close()

	methods := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	for _, m := range methods {
		id, err := StoreWebhook(m, "/test", "", nil, nil, "", "127.0.0.1")
		if err != nil {
			t.Fatalf("StoreWebhook(%s): %v", m, err)
		}
		msg, err := GetWebhook(id)
		if err != nil {
			t.Fatalf("GetWebhook(%s): %v", m, err)
		}
		assertEqual(t, msg.Method, m, "Method")
	}

	stats, _ := GetWebhookMailboxStats()
	assertEqual(t, stats.Total, uint64(len(methods)), "total for all methods")
}
