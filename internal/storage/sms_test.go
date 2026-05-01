package storage

import (
	"testing"
)

func TestSMSStore(t *testing.T) {
	setup("")
	defer Close()

	id, err := StoreSMS("+15550001111", "+15552223333", "Hello world", "ACtest")
	if err != nil {
		t.Fatalf("StoreSMS: %v", err)
	}
	if id == "" {
		t.Fatal("StoreSMS returned empty ID")
	}

	stats, err := GetSMSMailboxStats()
	if err != nil {
		t.Fatalf("GetSMSMailboxStats: %v", err)
	}
	assertEqual(t, stats.Total, uint64(1), "total after insert")
	assertEqual(t, stats.Unread, uint64(1), "unread after insert")
}

func TestSMSStatsEmptyTable(t *testing.T) {
	setup("")
	defer Close()

	// GetSMSMailboxStats on an empty table must not return a NULL scan error
	stats, err := GetSMSMailboxStats()
	if err != nil {
		t.Fatalf("GetSMSMailboxStats on empty table: %v", err)
	}
	assertEqual(t, stats.Total, uint64(0), "total on empty table")
	assertEqual(t, stats.Unread, uint64(0), "unread on empty table")
}

func TestSMSGet(t *testing.T) {
	setup("")
	defer Close()

	id, err := StoreSMS("+15550001111", "+15552223333", "Test body", "ACtest")
	if err != nil {
		t.Fatalf("StoreSMS: %v", err)
	}

	msg, err := GetSMS(id)
	if err != nil {
		t.Fatalf("GetSMS: %v", err)
	}
	assertEqual(t, msg.ID, id, "ID")
	assertEqual(t, msg.From, "+15550001111", "From")
	assertEqual(t, msg.To, "+15552223333", "To")
	assertEqual(t, msg.Body, "Test body", "Body")
	assertEqual(t, msg.AccountSID, "ACtest", "AccountSID")
	assertEqual(t, msg.Read, false, "Read should be false after store")
}

func TestSMSGetNotFound(t *testing.T) {
	setup("")
	defer Close()

	_, err := GetSMS("nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent ID, got nil")
	}
}

func TestSMSList(t *testing.T) {
	setup("")
	defer Close()

	for i := range 5 {
		body := "Message " + string(rune('A'+i))
		if _, err := StoreSMS("+15550001111", "+15552223333", body, ""); err != nil {
			t.Fatalf("StoreSMS: %v", err)
		}
	}

	msgs, err := ListSMS(0, 10)
	if err != nil {
		t.Fatalf("ListSMS: %v", err)
	}
	assertEqual(t, len(msgs), 5, "list count")

	// results are newest-first
	assertEqual(t, msgs[0].Body, "Message E", "first result should be newest")
}

func TestSMSListPagination(t *testing.T) {
	setup("")
	defer Close()

	for i := range 10 {
		if _, err := StoreSMS("+15550001111", "+15552223333", "msg", string(rune('A'+i))); err != nil {
			t.Fatalf("StoreSMS: %v", err)
		}
	}

	page1, err := ListSMS(0, 3)
	if err != nil {
		t.Fatalf("ListSMS page1: %v", err)
	}
	assertEqual(t, len(page1), 3, "page 1 count")

	page2, err := ListSMS(3, 3)
	if err != nil {
		t.Fatalf("ListSMS page2: %v", err)
	}
	assertEqual(t, len(page2), 3, "page 2 count")

	// ensure no overlap
	if page1[0].ID == page2[0].ID {
		t.Fatal("page 1 and page 2 returned the same message")
	}
}

func TestSMSMarkRead(t *testing.T) {
	setup("")
	defer Close()

	id, err := StoreSMS("+15550001111", "+15552223333", "Hello", "")
	if err != nil {
		t.Fatalf("StoreSMS: %v", err)
	}

	stats, _ := GetSMSMailboxStats()
	assertEqual(t, stats.Unread, uint64(1), "unread before mark read")

	if err := MarkSMSRead([]string{id}); err != nil {
		t.Fatalf("MarkSMSRead: %v", err)
	}

	msg, err := GetSMS(id)
	if err != nil {
		t.Fatalf("GetSMS after mark read: %v", err)
	}
	assertEqual(t, msg.Read, true, "Read flag after MarkSMSRead")

	stats, _ = GetSMSMailboxStats()
	assertEqual(t, stats.Unread, uint64(0), "unread after mark read")
}

func TestSMSDelete(t *testing.T) {
	setup("")
	defer Close()

	id, err := StoreSMS("+15550001111", "+15552223333", "Delete me", "")
	if err != nil {
		t.Fatalf("StoreSMS: %v", err)
	}

	if err := DeleteSMS(id); err != nil {
		t.Fatalf("DeleteSMS: %v", err)
	}

	stats, _ := GetSMSMailboxStats()
	assertEqual(t, stats.Total, uint64(0), "total after delete")

	_, err = GetSMS(id)
	if err == nil {
		t.Fatal("expected error fetching deleted SMS, got nil")
	}
}

func TestSearchSMS(t *testing.T) {
	setup("")
	defer Close()

	// store messages with distinguishable content
	_, _ = StoreSMS("+15550001111", "+15552223333", "Hello from Alice", "")
	_, _ = StoreSMS("+15559998888", "+15552223333", "Hello from Bob", "")
	_, _ = StoreSMS("+15550001111", "+15557776666", "Completely different content", "")

	// match on body
	results, total, err := SearchSMS("Hello", 0, 10)
	if err != nil {
		t.Fatalf("SearchSMS: %v", err)
	}
	assertEqual(t, total, 2, "total matches for 'Hello'")
	assertEqual(t, len(results), 2, "returned results for 'Hello'")

	// match on From number
	results, total, err = SearchSMS("+15559998888", 0, 10)
	if err != nil {
		t.Fatalf("SearchSMS by From: %v", err)
	}
	assertEqual(t, total, 1, "total matches for From number")
	assertEqual(t, len(results), 1, "returned results for From number")
	assertEqual(t, results[0].From, "+15559998888", "From number")

	// match on To number
	results, total, err = SearchSMS("+15557776666", 0, 10)
	if err != nil {
		t.Fatalf("SearchSMS by To: %v", err)
	}
	assertEqual(t, total, 1, "total matches for To number")
	assertEqual(t, results[0].To, "+15557776666", "To number")

	// no match
	results, total, err = SearchSMS("zzz_nomatch", 0, 10)
	if err != nil {
		t.Fatalf("SearchSMS no match: %v", err)
	}
	assertEqual(t, total, 0, "total for no-match query")
	assertEqual(t, len(results), 0, "results for no-match query")
}

func TestSearchSMSPagination(t *testing.T) {
	setup("")
	defer Close()

	for i := range 10 {
		body := "Paginated message " + string(rune('A'+i))
		if _, err := StoreSMS("+15550001111", "+15552223333", body, ""); err != nil {
			t.Fatalf("StoreSMS: %v", err)
		}
	}

	page1, total, err := SearchSMS("Paginated", 0, 3)
	if err != nil {
		t.Fatalf("SearchSMS page1: %v", err)
	}
	assertEqual(t, total, 10, "total across pages")
	assertEqual(t, len(page1), 3, "page 1 count")

	page2, _, err := SearchSMS("Paginated", 3, 3)
	if err != nil {
		t.Fatalf("SearchSMS page2: %v", err)
	}
	assertEqual(t, len(page2), 3, "page 2 count")

	if page1[0].ID == page2[0].ID {
		t.Fatal("page 1 and page 2 returned the same message")
	}
}

func TestSearchSMSEmptyQuery(t *testing.T) {
	setup("")
	defer Close()

	_, _ = StoreSMS("+1555", "+1666", "body", "")

	// empty LIKE matches everything — confirm it returns the stored message
	results, total, err := SearchSMS("", 0, 10)
	if err != nil {
		t.Fatalf("SearchSMS empty query: %v", err)
	}
	assertEqual(t, total, 1, "empty query matches all")
	assertEqual(t, len(results), 1, "empty query result count")
}

func TestSMSDeleteAll(t *testing.T) {
	setup("")
	defer Close()

	for range 5 {
		if _, err := StoreSMS("+15550001111", "+15552223333", "msg", ""); err != nil {
			t.Fatalf("StoreSMS: %v", err)
		}
	}

	stats, _ := GetSMSMailboxStats()
	assertEqual(t, stats.Total, uint64(5), "total before delete all")

	if err := DeleteAllSMS(); err != nil {
		t.Fatalf("DeleteAllSMS: %v", err)
	}

	stats, _ = GetSMSMailboxStats()
	assertEqual(t, stats.Total, uint64(0), "total after delete all")
	assertEqual(t, stats.Unread, uint64(0), "unread after delete all")
}
