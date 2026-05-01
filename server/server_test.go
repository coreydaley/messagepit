package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/coreydaley/messagepit/config"
	"github.com/coreydaley/messagepit/internal/auth"
	"github.com/coreydaley/messagepit/internal/logger"
	"github.com/coreydaley/messagepit/internal/sendgrid"
	"github.com/coreydaley/messagepit/internal/storage"
	"github.com/coreydaley/messagepit/server/apiv1"
	"github.com/jhillyerd/enmime/v2"
	"golang.org/x/crypto/bcrypt"
)

var (
	putDataStruct struct {
		Read bool
		IDs  []string
	}

	// Shared test message structure for consistency
	testSendMessage = map[string]any{
		"From": map[string]string{
			"Email": "test@example.com",
		},
		"To": []map[string]string{
			{"Email": "recipient@example.com"},
		},
		"Subject": "Test",
		"Text":    "Test message",
	}
)

func TestAPIv1Messages(t *testing.T) {
	setup()
	defer storage.Close()

	r := apiRoutes()

	ts := httptest.NewServer(r)
	defer ts.Close()

	m, err := fetchMessages(ts.URL + "/api/v1/messages")
	if err != nil {
		t.Error(err.Error())
	}

	// check count of empty database
	assertStatsEqual(t, ts.URL+"/api/v1/messages", 0, 0)

	// insert 100
	t.Log("Insert 100 messages")
	insertEmailData(t)
	assertStatsEqual(t, ts.URL+"/api/v1/messages", 100, 100)

	m, err = fetchMessages(ts.URL + "/api/v1/messages")
	if err != nil {
		t.Error(err.Error())
	}

	// read first 10 messages
	t.Log("Read first 10 messages including raw & headers")
	for idx, msg := range m.Messages {
		if idx == 10 {
			break
		}

		if _, err := clientGet(ts.URL + "/api/v1/message/" + msg.ID); err != nil {
			t.Error(err.Error())
		}

		// get RAW
		if _, err := clientGet(ts.URL + "/api/v1/message/" + msg.ID + "/raw"); err != nil {
			t.Error(err.Error())
		}

		// get headers
		if _, err := clientGet(ts.URL + "/api/v1/message/" + msg.ID + "/headers"); err != nil {
			t.Error(err.Error())
		}
	}

	// 10 should be marked as read
	assertStatsEqual(t, ts.URL+"/api/v1/messages", 90, 100)

	// delete all
	t.Log("Delete all messages")
	_, err = clientDelete(ts.URL+"/api/v1/messages", "{}")
	if err != nil {
		t.Errorf("Expected nil, received %s", err.Error())
	}
	assertStatsEqual(t, ts.URL+"/api/v1/messages", 0, 0)
}

func TestAPIv1ToggleReadStatus(t *testing.T) {
	setup()
	defer storage.Close()

	r := apiRoutes()

	ts := httptest.NewServer(r)
	defer ts.Close()

	m, err := fetchMessages(ts.URL + "/api/v1/messages")
	if err != nil {
		t.Error(err.Error())
	}

	// check count of empty database
	assertStatsEqual(t, ts.URL+"/api/v1/messages", 0, 0)

	// insert 100
	t.Log("Insert 100 messages")
	insertEmailData(t)
	assertStatsEqual(t, ts.URL+"/api/v1/messages", 100, 100)

	m, err = fetchMessages(ts.URL + "/api/v1/messages")
	if err != nil {
		t.Error(err.Error())
	}

	// read first 10 IDs
	t.Log("Get first 10 IDs")
	putIDs := []string{}
	for idx, msg := range m.Messages {
		if idx == 10 {
			break
		}

		// store for later
		putIDs = append(putIDs, msg.ID)
	}
	assertStatsEqual(t, ts.URL+"/api/v1/messages", 100, 100)

	// mark first 10 as unread
	t.Log("Mark first 10 as read")
	putData := putDataStruct
	putData.Read = true
	putData.IDs = putIDs
	j, err := json.Marshal(putData)
	if err != nil {
		t.Error(err.Error())
	}
	_, err = clientPut(ts.URL+"/api/v1/messages", string(j))
	if err != nil {
		t.Error(err.Error())
	}
	assertStatsEqual(t, ts.URL+"/api/v1/messages", 90, 100)

	// mark first 10 as read
	t.Log("Mark first 10 as unread")
	putData.Read = false
	j, err = json.Marshal(putData)
	if err != nil {
		t.Error(err.Error())
	}
	_, err = clientPut(ts.URL+"/api/v1/messages", string(j))
	if err != nil {
		t.Error(err.Error())
	}
	assertStatsEqual(t, ts.URL+"/api/v1/messages", 100, 100)

	// mark all as read
	putData.Read = true
	putData.IDs = []string{}
	j, err = json.Marshal(putData)
	if err != nil {
		t.Error(err.Error())
	}

	t.Log("Mark all read")
	_, err = clientPut(ts.URL+"/api/v1/messages", string(j))
	if err != nil {
		t.Error(err.Error())
	}
	assertStatsEqual(t, ts.URL+"/api/v1/messages", 0, 100)
}

func TestAPIv1Search(t *testing.T) {
	setup()
	defer storage.Close()

	r := apiRoutes()

	ts := httptest.NewServer(r)
	defer ts.Close()

	// insert 100
	t.Log("Insert 100 messages & tag")
	insertEmailData(t)
	assertStatsEqual(t, ts.URL+"/api/v1/messages", 100, 100)

	// search
	assertSearchEqual(t, ts.URL+"/api/v1/search", "from-1@example.com", 1)
	assertSearchEqual(t, ts.URL+"/api/v1/search", "from:from-1@example.com", 1)
	assertSearchEqual(t, ts.URL+"/api/v1/search", "-from:from-1@example.com", 99)
	assertSearchEqual(t, ts.URL+"/api/v1/search", "-FROM:FROM-1@EXAMPLE.COM", 99)
	assertSearchEqual(t, ts.URL+"/api/v1/search", "to:from-1@example.com", 0)
	assertSearchEqual(t, ts.URL+"/api/v1/search", "from:@example.com", 100)
	assertSearchEqual(t, ts.URL+"/api/v1/search", "subject:\"Subject line\"", 100)
	assertSearchEqual(t, ts.URL+"/api/v1/search", "subject:\"SUBJECT LINE 17 END\"", 1)
	assertSearchEqual(t, ts.URL+"/api/v1/search", "!thisdoesnotexist", 100)
	assertSearchEqual(t, ts.URL+"/api/v1/search", "-ThisDoesNotExist", 100)
	assertSearchEqual(t, ts.URL+"/api/v1/search", "thisdoesnotexist", 0)
	assertSearchEqual(t, ts.URL+"/api/v1/search", "tag:\"Test tag 065\"", 1)
	assertSearchEqual(t, ts.URL+"/api/v1/search", "tag:\"TEST TAG 065\"", 1)
	assertSearchEqual(t, ts.URL+"/api/v1/search", "!tag:\"Test tag 023\"", 99)
}

func TestAPIv1Send(t *testing.T) {
	setup()
	defer storage.Close()

	r := apiRoutes()

	ts := httptest.NewServer(r)
	defer ts.Close()

	jsonData := `{
		"From": {
		  "Email": "john@example.com",
		  "Name": "John Doe"
		},
		"To": [
		  {
			"Email": "jane@example.com",
			"Name": "Jane Doe"
		  }
		],
		"Cc": [
		  {
			"Email": "manager1@example.com",
			"Name": "Manager 1"
		  },
		  {
			"Email": "manager2@example.com",
			"Name": "Manager 2"
		  }
		],
		"Bcc": ["jack@example.com"],
		"Headers": {
			"X-IP": "1.2.3.4"
		},
		"Subject": "Mailpit message via the HTTP API",
		"Text": "This is the text body",
		"HTML": "<p style=\"font-family: arial\">Mailpit is <b>awesome</b>!</p>",
		"Attachments": [
		  {
			"Content": "VGhpcyBpcyBhIHBsYWluIHRleHQgYXR0YWNobWVudA==",
			"Filename": "Attached File.txt"
		  },
		  {
			"Content": "iVBORw0KGgoAAAANSUhEUgAAAEEAAAA8CAMAAAAOlSdoAAAACXBIWXMAAAHrAAAB6wGM2bZBAAAAS1BMVEVHcEwRfnUkZ2gAt4UsSF8At4UtSV4At4YsSV4At4YsSV8At4YsSV4At4YsSV4sSV4At4YsSV4At4YtSV4At4YsSV4At4YtSV8At4YsUWYNAAAAGHRSTlMAAwoXGiktRE5dbnd7kpOlr7zJ0d3h8PD8PCSRAAACWUlEQVR42pXT4ZaqIBSG4W9rhqQYocG+/ys9Y0Z0Br+x3j8zaxUPewFh65K+7yrIMeIY4MT3wPfEJCidKXEMnLaVkxDiELiMz4WEOAZSFghxBIypCOlKiAMgXfIqTnBgSm8CIQ6BImxEUxEckClVQiHGj4Ba4AQHikAIClwTE9KtIghAhUJwoLkmLnCiAHJLRKgIMsEtVUKbBUIwoAg2C4QgQBE6l4VCnApBgSKYLLApCnCa0+96AEMW2BQcmC+Pr3nfp7o5Exy49gIADcIqUELGfeA+bp93LmAJp8QJoEcN3C7NY3sbVANixMyI0nku20/n5/ZRf3KI2k6JEDWQtxcbdGuAqu3TAXG+/799Oyyas1B1MnMiA+XyxHp9q0PUKGPiRAau1fZbLRZV09wZcT8/gHk8QQAxXn8VgaDqcUmU6O/r28nbVwXAqca2mRNtPAF5+zoP2MeN9Fy4NgC6RfcbgE7XITBRYTtOE3U3C2DVff7pk+PkUxgAbvtnPXJaD6DxulMLwOhPS/M3MQkgg1ZFrIXnmfaZoOfpKiFgzeZD/WuKqQEGrfJYkyWf6vlG3xUgTuscnkNkQsb599q124kdpMUjCa/XARHs1gZymVtGt3wLkiFv8rUgTxitYCex5EVGec0Y9VmoDTFBSQte2TfXGXlf7hbdaUM9Sk7fisEN9qfBBTK+FZcvM9fQSdkl2vj4W2oX/bRogO3XasiNH7R0eW7fgRM834ImTg+Lg6BEnx4vz81rhr+MYPBBQg1v8GndEOrthxaCTxNAOut8WKLGZQl+MPz88Q9tAO/hVuSeqQAAAABJRU5ErkJggg==",
			"Filename": "logo.png",
			"ContentID": "inline-cid",
			"ContentType": "overridden/type"
		  }
		],
		"ReplyTo": [
		  {
			"Email": "secretary@example.com",
			"Name": "Secretary"
		  }
		],
		"Tags": [
		  "Tag 1",
		  "Tag 2"
		]
	  }`

	t.Log("Sending message via HTTP API")
	b, err := clientPost(ts.URL+"/api/v1/send", jsonData)
	if err != nil {
		t.Errorf("Expected nil, received %s", err.Error())
	}

	resp := struct {
		ID string
	}{}

	if err := json.Unmarshal(b, &resp); err != nil {
		t.Error(err.Error())
		return
	}

	t.Logf("Fetching response for message %s", resp.ID)
	msg, err := fetchMessage(ts.URL + "/api/v1/message/" + resp.ID)
	if err != nil {
		t.Error(err.Error())
	}

	t.Logf("Testing response for message %s", resp.ID)
	assertEqual(t, `Mailpit message via the HTTP API`, msg.Subject, "wrong subject")
	assertEqual(t, `This is the text body`, msg.Text, "wrong text")
	assertEqual(t, `<p style="font-family: arial">Mailpit is <b>awesome</b>!</p>`, msg.HTML, "wrong HTML")
	assertEqual(t, `"John Doe" <john@example.com>`, msg.From.String(), "wrong HTML")
	assertEqual(t, 1, len(msg.To), "wrong To count")
	assertEqual(t, `"Jane Doe" <jane@example.com>`, msg.To[0].String(), "wrong To address")
	assertEqual(t, 2, len(msg.Cc), "wrong Cc count")
	assertEqual(t, `"Manager 1" <manager1@example.com>`, msg.Cc[0].String(), "wrong Cc address")
	assertEqual(t, `"Manager 2" <manager2@example.com>`, msg.Cc[1].String(), "wrong Cc address")
	assertEqual(t, 1, len(msg.Bcc), "wrong Bcc count")
	assertEqual(t, `<jack@example.com>`, msg.Bcc[0].String(), "wrong Bcc address")
	assertEqual(t, 1, len(msg.ReplyTo), "wrong Reply-To count")
	assertEqual(t, `"Secretary" <secretary@example.com>`, msg.ReplyTo[0].String(), "wrong Reply-To address")
	assertEqual(t, 2, len(msg.Tags), "wrong Tags count")
	assertEqual(t, `Tag 1,Tag 2`, strings.Join(msg.Tags, ","), "wrong Tags")
	assertEqual(t, 1, len(msg.Attachments), "wrong Attachment count")
	assertEqual(t, `Attached File.txt`, msg.Attachments[0].FileName, "wrong Attachment name")
	assertEqual(t, `text/plain`, msg.Attachments[0].ContentType, "wrong Content-Type")
	assertEqual(t, 1, len(msg.Inline), "wrong inline Attachment count")
	assertEqual(t, `logo.png`, msg.Inline[0].FileName, "wrong Attachment name")
	assertEqual(t, `overridden/type`, msg.Inline[0].ContentType, "wrong Content-Type")

	attachmentBytes, err := clientGet(ts.URL + "/api/v1/message/" + resp.ID + "/part/" + msg.Attachments[0].PartID)
	if err != nil {
		t.Error(err.Error())
	}
	assertEqual(t, `This is a plain text attachment`, string(attachmentBytes), "wrong Attachment content")
}

func TestSendAPIAuthMiddleware(t *testing.T) {
	setup()
	defer storage.Close()

	// Test 1: Send API with accept-any enabled (should bypass all auth)
	t.Run("SendAPIAuthAcceptAny", func(t *testing.T) {
		// Set up UI auth and enable accept-any for send API
		originalSendAPIAuthAcceptAny := config.SendAPIAuthAcceptAny
		originalUICredentials := auth.UICredentials
		defer func() {
			config.SendAPIAuthAcceptAny = originalSendAPIAuthAcceptAny
			auth.UICredentials = originalUICredentials
		}()

		// Enable accept-any for send API
		config.SendAPIAuthAcceptAny = true

		// Set up UI auth that would normally block requests
		testHash, _ := bcrypt.GenerateFromPassword([]byte("testpass"), bcrypt.DefaultCost)
		if err := auth.SetUIAuth("testuser:" + string(testHash)); err != nil {
			t.Fatalf("Failed to set UI auth: %s", err.Error())
		}

		r := apiRoutes()
		ts := httptest.NewServer(r)
		defer ts.Close()

		// Should succeed without any auth headers
		jsonData, _ := json.Marshal(testSendMessage)
		_, err := clientPost(ts.URL+"/api/v1/send", string(jsonData))
		if err != nil {
			t.Errorf("Expected send to succeed with accept-any, got error: %s", err.Error())
		}
	})

	// Test 2: Send API with dedicated credentials
	t.Run("SendAPIWithDedicatedCredentials", func(t *testing.T) {
		originalSendAPIAuthAcceptAny := config.SendAPIAuthAcceptAny
		originalUICredentials := auth.UICredentials
		originalSendAPICredentials := auth.SendAPICredentials
		defer func() {
			config.SendAPIAuthAcceptAny = originalSendAPIAuthAcceptAny
			auth.UICredentials = originalUICredentials
			auth.SendAPICredentials = originalSendAPICredentials
		}()

		config.SendAPIAuthAcceptAny = false

		// Set up UI auth
		uiHash, _ := bcrypt.GenerateFromPassword([]byte("uipass"), bcrypt.DefaultCost)
		if err := auth.SetUIAuth("uiuser:" + string(uiHash)); err != nil {
			t.Fatalf("Failed to set UI auth: %s", err.Error())
		}

		// Set up dedicated Send API auth
		sendHash, _ := bcrypt.GenerateFromPassword([]byte("sendpass"), bcrypt.DefaultCost)
		if err := auth.SetSendAPIAuth("senduser:" + string(sendHash)); err != nil {
			t.Fatalf("Failed to set Send API auth: %s", err.Error())
		}

		r := apiRoutes()
		ts := httptest.NewServer(r)
		defer ts.Close()

		jsonData, _ := json.Marshal(testSendMessage)

		// Should succeed with correct Send API credentials
		_, err := clientPostWithAuth(ts.URL+"/api/v1/send", string(jsonData), "senduser", "sendpass")
		if err != nil {
			t.Errorf("Expected send to succeed with correct Send API credentials, got error: %s", err.Error())
		}

		// Should fail with wrong Send API credentials
		_, err = clientPostWithAuth(ts.URL+"/api/v1/send", string(jsonData), "senduser", "wrongpass")
		if err == nil {
			t.Error("Expected send to fail with wrong Send API credentials")
		}

		// Should fail with UI credentials when Send API credentials are set
		_, err = clientPostWithAuth(ts.URL+"/api/v1/send", string(jsonData), "uiuser", "uipass")
		if err == nil {
			t.Error("Expected send to fail with UI credentials when Send API credentials are required")
		}
	})

	// Test 3: Send API fallback to UI auth when no Send API auth is configured
	t.Run("SendAPIFallbackToUIAuth", func(t *testing.T) {
		originalSendAPIAuthAcceptAny := config.SendAPIAuthAcceptAny
		originalUICredentials := auth.UICredentials
		originalSendAPICredentials := auth.SendAPICredentials
		defer func() {
			config.SendAPIAuthAcceptAny = originalSendAPIAuthAcceptAny
			auth.UICredentials = originalUICredentials
			auth.SendAPICredentials = originalSendAPICredentials
		}()

		config.SendAPIAuthAcceptAny = false
		auth.SendAPICredentials = nil

		// Set up only UI auth
		uiHash, _ := bcrypt.GenerateFromPassword([]byte("uipass"), bcrypt.DefaultCost)
		if err := auth.SetUIAuth("uiuser:" + string(uiHash)); err != nil {
			t.Fatalf("Failed to set UI auth: %s", err.Error())
		}

		r := apiRoutes()
		ts := httptest.NewServer(r)
		defer ts.Close()

		jsonData, _ := json.Marshal(testSendMessage)

		// Should succeed with UI credentials when no Send API auth is configured
		_, err := clientPostWithAuth(ts.URL+"/api/v1/send", string(jsonData), "uiuser", "uipass")
		if err != nil {
			t.Errorf("Expected send to succeed with UI credentials when no Send API auth configured, got error: %s", err.Error())
		}

		// Should fail without any credentials
		_, err = clientPost(ts.URL+"/api/v1/send", string(jsonData))
		if err == nil {
			t.Error("Expected send to fail without credentials when UI auth is required")
		}
	})

	// Test 4: Regular API endpoints should not be affected by Send API auth settings
	t.Run("RegularAPINotAffectedBySendAPIAuth", func(t *testing.T) {
		originalSendAPIAuthAcceptAny := config.SendAPIAuthAcceptAny
		originalUICredentials := auth.UICredentials
		originalSendAPICredentials := auth.SendAPICredentials
		defer func() {
			config.SendAPIAuthAcceptAny = originalSendAPIAuthAcceptAny
			auth.UICredentials = originalUICredentials
			auth.SendAPICredentials = originalSendAPICredentials
		}()

		// Set up UI auth and Send API auth
		uiHash, _ := bcrypt.GenerateFromPassword([]byte("uipass"), bcrypt.DefaultCost)
		if err := auth.SetUIAuth("uiuser:" + string(uiHash)); err != nil {
			t.Fatalf("Failed to set UI auth: %s", err.Error())
		}

		sendHash, _ := bcrypt.GenerateFromPassword([]byte("sendpass"), bcrypt.DefaultCost)
		if err := auth.SetSendAPIAuth("senduser:" + string(sendHash)); err != nil {
			t.Fatalf("Failed to set Send API auth: %s", err.Error())
		}

		r := apiRoutes()
		ts := httptest.NewServer(r)
		defer ts.Close()

		// Regular API endpoint should require UI credentials, not Send API credentials
		_, err := clientGetWithAuth(ts.URL+"/api/v1/messages", "uiuser", "uipass")
		if err != nil {
			t.Errorf("Expected regular API to work with UI credentials, got error: %s", err.Error())
		}

		// Regular API endpoint should fail with Send API credentials
		_, err = clientGetWithAuth(ts.URL+"/api/v1/messages", "senduser", "sendpass")
		if err == nil {
			t.Error("Expected regular API to fail with Send API credentials")
		}
	})
}

func TestAPIv1Webhooks(t *testing.T) {
	setup()
	defer storage.Close()

	r := apiRoutes()
	ts := httptest.NewServer(r)
	defer ts.Close()

	// empty list
	data, err := clientGet(ts.URL + "/api/v1/webhooks")
	if err != nil {
		t.Fatalf("GET /api/v1/webhooks empty: %v", err)
	}
	resp := apiv1.WebhookRequestsSummary{}
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("unmarshal empty list: %v", err)
	}
	if resp.Total != 0 {
		t.Fatalf("expected 0 total, got %d", resp.Total)
	}

	// store a webhook directly
	id, err := storage.StoreWebhook("POST", "/test", "foo=1", map[string][]string{
		"Content-Type": {"application/json"},
	}, []byte(`{"hello":"world"}`), "application/json", "127.0.0.1")
	if err != nil {
		t.Fatalf("StoreWebhook: %v", err)
	}

	// list now has one entry
	data, err = clientGet(ts.URL + "/api/v1/webhooks")
	if err != nil {
		t.Fatalf("GET /api/v1/webhooks after insert: %v", err)
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	if resp.Total != 1 {
		t.Fatalf("expected total 1, got %d", resp.Total)
	}
	if resp.Unread != 1 {
		t.Fatalf("expected unread 1, got %d", resp.Unread)
	}
	if len(resp.Messages) != 1 {
		t.Fatalf("expected 1 message in list, got %d", len(resp.Messages))
	}
	if resp.Messages[0].Method != "POST" {
		t.Fatalf("expected method POST, got %s", resp.Messages[0].Method)
	}

	// get detail — should auto-mark read
	data, err = clientGet(ts.URL + "/api/v1/webhook/" + id)
	if err != nil {
		t.Fatalf("GET /api/v1/webhook/%s: %v", id, err)
	}
	detail := storage.WebhookRequest{}
	if err := json.Unmarshal(data, &detail); err != nil {
		t.Fatalf("unmarshal detail: %v", err)
	}
	if detail.ID != id {
		t.Fatalf("expected ID %s, got %s", id, detail.ID)
	}
	if detail.Body != `{"hello":"world"}` {
		t.Fatalf("unexpected body: %s", detail.Body)
	}

	// unread count should drop to 0 after GET detail
	data, _ = clientGet(ts.URL + "/api/v1/webhooks")
	_ = json.Unmarshal(data, &resp)
	if resp.Unread != 0 {
		t.Fatalf("expected unread 0 after reading detail, got %d", resp.Unread)
	}

	// 404 for unknown ID
	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/webhook/nonexistent", nil)
	r404, _ := http.DefaultClient.Do(req)
	_ = r404.Body.Close()
	if r404.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown webhook, got %d", r404.StatusCode)
	}

	// delete single
	if _, err := clientDelete(ts.URL+"/api/v1/webhook/"+id, ""); err != nil {
		t.Fatalf("DELETE /api/v1/webhook/%s: %v", id, err)
	}
	data, _ = clientGet(ts.URL + "/api/v1/webhooks")
	_ = json.Unmarshal(data, &resp)
	if resp.Total != 0 {
		t.Fatalf("expected 0 after delete, got %d", resp.Total)
	}
}

func TestAPIv1WebhooksDeleteAll(t *testing.T) {
	setup()
	defer storage.Close()

	r := apiRoutes()
	ts := httptest.NewServer(r)
	defer ts.Close()

	for i := range 5 {
		path := "/hook/" + string(rune('a'+i))
		if _, err := storage.StoreWebhook("POST", path, "", nil, []byte("body"), "text/plain", "127.0.0.1"); err != nil {
			t.Fatalf("StoreWebhook: %v", err)
		}
	}

	data, _ := clientGet(ts.URL + "/api/v1/webhooks")
	resp := apiv1.WebhookRequestsSummary{}
	_ = json.Unmarshal(data, &resp)
	if resp.Total != 5 {
		t.Fatalf("expected 5 before delete all, got %d", resp.Total)
	}

	if _, err := clientDelete(ts.URL+"/api/v1/webhooks", ""); err != nil {
		t.Fatalf("DELETE /api/v1/webhooks: %v", err)
	}

	data, _ = clientGet(ts.URL + "/api/v1/webhooks")
	_ = json.Unmarshal(data, &resp)
	if resp.Total != 0 {
		t.Fatalf("expected 0 after delete all, got %d", resp.Total)
	}
}

func TestAPIv1SMSPagination(t *testing.T) {
	setup()
	defer storage.Close()

	r := apiRoutes()
	ts := httptest.NewServer(r)
	defer ts.Close()

	// seed 10 messages
	for i := range 10 {
		if _, err := storage.StoreSMS("+1555000"+fmt.Sprintf("%04d", i), "+15552223333", fmt.Sprintf("Message %d", i), ""); err != nil {
			t.Fatalf("StoreSMS: %v", err)
		}
	}

	// page 1: limit=3
	data, err := clientGet(ts.URL + "/api/v1/sms/messages?limit=3")
	if err != nil {
		t.Fatalf("GET page 1: %v", err)
	}
	resp := apiv1.SMSMessagesSummary{}
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("unmarshal page 1: %v", err)
	}
	if resp.Total != 10 {
		t.Fatalf("expected total 10, got %d", resp.Total)
	}
	if len(resp.Messages) != 3 {
		t.Fatalf("expected 3 messages on page 1, got %d", len(resp.Messages))
	}
	if resp.Start != 0 {
		t.Fatalf("expected start 0, got %d", resp.Start)
	}

	page1IDs := []string{resp.Messages[0].ID, resp.Messages[1].ID, resp.Messages[2].ID}

	// page 2: start=3, limit=3
	data, err = clientGet(ts.URL + "/api/v1/sms/messages?start=3&limit=3")
	if err != nil {
		t.Fatalf("GET page 2: %v", err)
	}
	resp = apiv1.SMSMessagesSummary{}
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("unmarshal page 2: %v", err)
	}
	if len(resp.Messages) != 3 {
		t.Fatalf("expected 3 messages on page 2, got %d", len(resp.Messages))
	}
	if resp.Start != 3 {
		t.Fatalf("expected start 3, got %d", resp.Start)
	}

	// no overlap between pages
	for _, id := range page1IDs {
		for _, msg := range resp.Messages {
			if msg.ID == id {
				t.Fatalf("page 2 contains ID %s from page 1", id)
			}
		}
	}

	// last page: start=9, limit=3 — only 1 message remains
	data, err = clientGet(ts.URL + "/api/v1/sms/messages?start=9&limit=3")
	if err != nil {
		t.Fatalf("GET last page: %v", err)
	}
	resp = apiv1.SMSMessagesSummary{}
	_ = json.Unmarshal(data, &resp)
	if len(resp.Messages) != 1 {
		t.Fatalf("expected 1 message on last page, got %d", len(resp.Messages))
	}
}

func TestAPIv1WebhooksPagination(t *testing.T) {
	setup()
	defer storage.Close()

	r := apiRoutes()
	ts := httptest.NewServer(r)
	defer ts.Close()

	// seed 10 webhooks
	for i := range 10 {
		path := fmt.Sprintf("/hook/%d", i)
		if _, err := storage.StoreWebhook("POST", path, "", nil, []byte("body"), "text/plain", "127.0.0.1"); err != nil {
			t.Fatalf("StoreWebhook: %v", err)
		}
	}

	// page 1: limit=4
	data, err := clientGet(ts.URL + "/api/v1/webhooks?limit=4")
	if err != nil {
		t.Fatalf("GET page 1: %v", err)
	}
	resp := apiv1.WebhookRequestsSummary{}
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("unmarshal page 1: %v", err)
	}
	if resp.Total != 10 {
		t.Fatalf("expected total 10, got %d", resp.Total)
	}
	if len(resp.Messages) != 4 {
		t.Fatalf("expected 4 messages on page 1, got %d", len(resp.Messages))
	}
	if resp.Start != 0 {
		t.Fatalf("expected start 0, got %d", resp.Start)
	}

	page1IDs := make(map[string]bool)
	for _, m := range resp.Messages {
		page1IDs[m.ID] = true
	}

	// page 2: start=4, limit=4
	data, err = clientGet(ts.URL + "/api/v1/webhooks?start=4&limit=4")
	if err != nil {
		t.Fatalf("GET page 2: %v", err)
	}
	resp = apiv1.WebhookRequestsSummary{}
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("unmarshal page 2: %v", err)
	}
	if len(resp.Messages) != 4 {
		t.Fatalf("expected 4 messages on page 2, got %d", len(resp.Messages))
	}
	if resp.Start != 4 {
		t.Fatalf("expected start 4, got %d", resp.Start)
	}
	for _, m := range resp.Messages {
		if page1IDs[m.ID] {
			t.Fatalf("page 2 contains ID %s from page 1", m.ID)
		}
	}

	// last page: start=8, limit=4 — only 2 remain
	data, err = clientGet(ts.URL + "/api/v1/webhooks?start=8&limit=4")
	if err != nil {
		t.Fatalf("GET last page: %v", err)
	}
	resp = apiv1.WebhookRequestsSummary{}
	_ = json.Unmarshal(data, &resp)
	if len(resp.Messages) != 2 {
		t.Fatalf("expected 2 messages on last page, got %d", len(resp.Messages))
	}
}

func TestAPIv1SMSSearchPagination(t *testing.T) {
	setup()
	defer storage.Close()

	r := apiRoutes()
	ts := httptest.NewServer(r)
	defer ts.Close()

	for i := range 8 {
		if _, err := storage.StoreSMS("+1555", "+1666", fmt.Sprintf("Searchable content %d", i), ""); err != nil {
			t.Fatalf("StoreSMS: %v", err)
		}
	}

	// page 1
	data, err := clientGet(ts.URL + "/api/v1/sms/search?query=Searchable&limit=3")
	if err != nil {
		t.Fatalf("GET search page 1: %v", err)
	}
	result := apiv1.SMSSearchResult{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal page 1: %v", err)
	}
	if result.Total != 8 {
		t.Fatalf("expected total 8, got %d", result.Total)
	}
	if len(result.Messages) != 3 {
		t.Fatalf("expected 3 results on page 1, got %d", len(result.Messages))
	}

	page1IDs := make(map[string]bool)
	for _, m := range result.Messages {
		page1IDs[m.ID] = true
	}

	// page 2
	data, err = clientGet(ts.URL + "/api/v1/sms/search?query=Searchable&start=3&limit=3")
	if err != nil {
		t.Fatalf("GET search page 2: %v", err)
	}
	result = apiv1.SMSSearchResult{}
	_ = json.Unmarshal(data, &result)
	if len(result.Messages) != 3 {
		t.Fatalf("expected 3 results on page 2, got %d", len(result.Messages))
	}
	for _, m := range result.Messages {
		if page1IDs[m.ID] {
			t.Fatalf("page 2 contains ID %s from page 1", m.ID)
		}
	}
}

func TestAPIv1WebhooksSearchPagination(t *testing.T) {
	setup()
	defer storage.Close()

	r := apiRoutes()
	ts := httptest.NewServer(r)
	defer ts.Close()

	for i := range 8 {
		path := fmt.Sprintf("/searchable/%d", i)
		if _, err := storage.StoreWebhook("POST", path, "", nil, nil, "", "127.0.0.1"); err != nil {
			t.Fatalf("StoreWebhook: %v", err)
		}
	}

	// page 1
	data, err := clientGet(ts.URL + "/api/v1/webhooks/search?query=/searchable/&limit=3")
	if err != nil {
		t.Fatalf("GET search page 1: %v", err)
	}
	result := apiv1.WebhookSearchResult{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal page 1: %v", err)
	}
	if result.Total != 8 {
		t.Fatalf("expected total 8, got %d", result.Total)
	}
	if len(result.Messages) != 3 {
		t.Fatalf("expected 3 results on page 1, got %d", len(result.Messages))
	}

	page1IDs := make(map[string]bool)
	for _, m := range result.Messages {
		page1IDs[m.ID] = true
	}

	// page 2
	data, err = clientGet(ts.URL + "/api/v1/webhooks/search?query=/searchable/&start=3&limit=3")
	if err != nil {
		t.Fatalf("GET search page 2: %v", err)
	}
	result = apiv1.WebhookSearchResult{}
	_ = json.Unmarshal(data, &result)
	if len(result.Messages) != 3 {
		t.Fatalf("expected 3 results on page 2, got %d", len(result.Messages))
	}
	for _, m := range result.Messages {
		if page1IDs[m.ID] {
			t.Fatalf("page 2 contains ID %s from page 1", m.ID)
		}
	}
}

func TestAPIv1SMSSearch(t *testing.T) {
	setup()
	defer storage.Close()

	r := apiRoutes()
	ts := httptest.NewServer(r)
	defer ts.Close()

	// missing query param returns error
	resp, err := http.Get(ts.URL + "/api/v1/sms/search")
	if err != nil {
		t.Fatalf("GET /api/v1/sms/search no query: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing query, got %d", resp.StatusCode)
	}

	// seed messages
	_, _ = storage.StoreSMS("+15550001111", "+15552223333", "Hello from Alice", "")
	_, _ = storage.StoreSMS("+15559998888", "+15552223333", "Hello from Bob", "")
	_, _ = storage.StoreSMS("+15550001111", "+15557776666", "Unrelated content here", "")

	// search matching two messages
	data, err := clientGet(ts.URL + "/api/v1/sms/search?query=Hello")
	if err != nil {
		t.Fatalf("GET /api/v1/sms/search?query=Hello: %v", err)
	}
	result := apiv1.SMSSearchResult{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal SMSSearchResult: %v", err)
	}
	if result.Total != 2 {
		t.Fatalf("expected total 2, got %d", result.Total)
	}
	if len(result.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result.Messages))
	}

	// search matching one message
	data, err = clientGet(ts.URL + "/api/v1/sms/search?query=Unrelated")
	if err != nil {
		t.Fatalf("GET /api/v1/sms/search?query=Unrelated: %v", err)
	}
	result = apiv1.SMSSearchResult{}
	_ = json.Unmarshal(data, &result)
	if result.Total != 1 {
		t.Fatalf("expected total 1, got %d", result.Total)
	}

	// search with no matches returns empty list (not null)
	data, err = clientGet(ts.URL + "/api/v1/sms/search?query=zzz_nomatch")
	if err != nil {
		t.Fatalf("GET /api/v1/sms/search?query=zzz_nomatch: %v", err)
	}
	result = apiv1.SMSSearchResult{}
	_ = json.Unmarshal(data, &result)
	if result.Total != 0 {
		t.Fatalf("expected total 0, got %d", result.Total)
	}
	// ensure Messages is [] not null in JSON
	if !bytes.Contains(data, []byte(`"messages":[]`)) {
		t.Fatalf("expected messages to be empty array, got: %s", string(data))
	}
}

func TestAPIv1WebhooksSearch(t *testing.T) {
	setup()
	defer storage.Close()

	r := apiRoutes()
	ts := httptest.NewServer(r)
	defer ts.Close()

	// missing query param returns error
	resp, err := http.Get(ts.URL + "/api/v1/webhooks/search")
	if err != nil {
		t.Fatalf("GET /api/v1/webhooks/search no query: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing query, got %d", resp.StatusCode)
	}

	// seed webhooks
	_, _ = storage.StoreWebhook("POST", "/api/orders", "", nil, []byte(`{"item":"book"}`), "application/json", "10.0.0.1")
	_, _ = storage.StoreWebhook("GET", "/api/health", "status=ok", nil, nil, "", "10.0.0.2")
	_, _ = storage.StoreWebhook("DELETE", "/api/orders/42", "", nil, nil, "", "192.168.1.5")

	// search matching two webhooks by path
	data, err := clientGet(ts.URL + "/api/v1/webhooks/search?query=/api/orders")
	if err != nil {
		t.Fatalf("GET /api/v1/webhooks/search?query=/api/orders: %v", err)
	}
	result := apiv1.WebhookSearchResult{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal WebhookSearchResult: %v", err)
	}
	if result.Total != 2 {
		t.Fatalf("expected total 2, got %d", result.Total)
	}

	// search by method
	data, err = clientGet(ts.URL + "/api/v1/webhooks/search?query=DELETE")
	if err != nil {
		t.Fatalf("GET /api/v1/webhooks/search?query=DELETE: %v", err)
	}
	result = apiv1.WebhookSearchResult{}
	_ = json.Unmarshal(data, &result)
	if result.Total != 1 {
		t.Fatalf("expected total 1, got %d", result.Total)
	}
	if result.Messages[0].Method != "DELETE" {
		t.Fatalf("expected DELETE method, got %s", result.Messages[0].Method)
	}

	// no matches — messages must be [] not null
	data, err = clientGet(ts.URL + "/api/v1/webhooks/search?query=zzz_nomatch")
	if err != nil {
		t.Fatalf("GET /api/v1/webhooks/search?query=zzz_nomatch: %v", err)
	}
	result = apiv1.WebhookSearchResult{}
	_ = json.Unmarshal(data, &result)
	if result.Total != 0 {
		t.Fatalf("expected total 0, got %d", result.Total)
	}
	if !bytes.Contains(data, []byte(`"messages":[]`)) {
		t.Fatalf("expected messages to be empty array, got: %s", string(data))
	}
}

func TestSendGridEndpoint(t *testing.T) {
	setup()
	defer storage.Close()

	r := apiRoutes()
	r.HandleFunc("/v3/mail/send", sendgrid.CreateMessage).Methods("POST")
	ts := httptest.NewServer(r)
	defer ts.Close()

	payload := `{"from":{"email":"sender@example.com"},"subject":"Integration Test","personalizations":[{"to":[{"email":"to@example.com"}]}],"content":[{"type":"text/plain","value":"hello"}]}`

	_, err := clientPostExpect(ts.URL+"/v3/mail/send", payload, http.StatusAccepted)
	if err != nil {
		t.Fatalf("POST /v3/mail/send: %v", err)
	}

	msgs, err := storage.List(0, 0, 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message stored after SendGrid POST, got %d", len(msgs))
	}
}

func setup() {
	logger.NoLogging = true
	config.MaxMessages = 0
	config.Database = os.Getenv("MP_DATABASE")

	if err := storage.InitDB(); err != nil {
		panic(err)
	}

	if err := storage.DeleteAllMessages(); err != nil {
		panic(err)
	}
}

func assertStatsEqual(t *testing.T, uri string, unread, total int) {
	m := apiv1.MessagesSummary{}

	data, err := clientGet(uri)
	if err != nil {
		t.Error(err.Error())
		return
	}

	if err := json.Unmarshal(data, &m); err != nil {
		t.Error(err.Error())
		return
	}

	assertEqual(t, uint64(unread), m.Unread, "wrong unread count")
	assertEqual(t, uint64(total), m.Total, "wrong total count")
}

func assertSearchEqual(t *testing.T, uri, query string, count int) {
	t.Logf("Test search: %s", query)
	m := apiv1.MessagesSummary{}

	limit := fmt.Sprintf("%d", count)

	data, err := clientGet(uri + "?query=" + url.QueryEscape(query) + "&limit=" + limit)
	if err != nil {
		t.Error(err.Error())
		return
	}

	if err := json.Unmarshal(data, &m); err != nil {
		t.Error(err.Error())
		return
	}

	assertEqual(t, uint64(count), m.MessagesCount, "wrong search results count")
}

func insertEmailData(t *testing.T) {
	for i := range 100 {
		msg := enmime.Builder().
			From(fmt.Sprintf("From %d", i), fmt.Sprintf("from-%d@example.com", i)).
			Subject(fmt.Sprintf("Subject line %d end", i)).
			Text(fmt.Appendf(nil, "This is the email body %d <jdsauk;dwqmdqw;>.", i)).
			To(fmt.Sprintf("To %d", i), fmt.Sprintf("to-%d@example.com", i))

		env, err := msg.Build()
		if err != nil {
			t.Log("error ", err)
			t.Fail()
		}

		buf := new(bytes.Buffer)

		if err := env.Encode(buf); err != nil {
			t.Log("error ", err)
			t.Fail()
		}

		bufBytes := buf.Bytes()

		id, err := storage.Store(&bufBytes, nil)
		if err != nil {
			t.Log("error ", err)
			t.Fail()
		}

		if _, err := storage.SetMessageTags(id, []string{fmt.Sprintf("Test tag %03d", i)}); err != nil {
			t.Log("error ", err)
			t.Fail()
		}
	}
}

func fetchMessage(url string) (storage.Message, error) {
	m := storage.Message{}

	data, err := clientGet(url)
	if err != nil {
		return m, err
	}

	if err := json.Unmarshal(data, &m); err != nil {
		return m, err
	}

	return m, nil
}

func fetchMessages(url string) (apiv1.MessagesSummary, error) {
	m := apiv1.MessagesSummary{}

	data, err := clientGet(url)
	if err != nil {
		return m, err
	}

	if err := json.Unmarshal(data, &m); err != nil {
		return m, err
	}

	return m, nil
}

func clientGet(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned status %d", url, resp.StatusCode)
	}

	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)

	return data, err
}

func clientDelete(url, body string) ([]byte, error) {
	client := new(http.Client)

	b := strings.NewReader(body)
	req, err := http.NewRequest("DELETE", url, b)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned status %d", url, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)

	return data, err
}

func clientPut(url, body string) ([]byte, error) {
	client := new(http.Client)

	b := strings.NewReader(body)
	req, err := http.NewRequest("PUT", url, b)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned status %d", url, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)

	return data, err
}

func clientPost(url, body string) ([]byte, error) {
	client := new(http.Client)

	b := strings.NewReader(body)
	req, err := http.NewRequest("POST", url, b)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned status %d", url, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)

	return data, err
}

func clientPostExpect(url, body string, wantStatus int) ([]byte, error) {
	client := new(http.Client)

	b := strings.NewReader(body)
	req, err := http.NewRequest("POST", url, b)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != wantStatus {
		return nil, fmt.Errorf("%s returned status %d, want %d", url, resp.StatusCode, wantStatus)
	}

	data, err := io.ReadAll(resp.Body)

	return data, err
}

func clientPostWithAuth(url, body, username, password string) ([]byte, error) {
	client := new(http.Client)

	b := strings.NewReader(body)
	req, err := http.NewRequest("POST", url, b)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(username, password)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned status %d", url, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)

	return data, err
}

func clientGetWithAuth(url, username, password string) ([]byte, error) {
	client := new(http.Client)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(username, password)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned status %d", url, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)

	return data, err
}

func assertEqual(t *testing.T, a any, b any, message string) {
	if a == b {
		return
	}
	message = fmt.Sprintf("%s: \"%v\" != \"%v\"", message, a, b)
	t.Fatal(message)
}
