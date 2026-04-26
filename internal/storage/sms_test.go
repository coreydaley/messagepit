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
