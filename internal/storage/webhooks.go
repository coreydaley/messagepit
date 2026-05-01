package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/coreydaley/messagepit/internal/logger"
	"github.com/leporo/sqlf"
	"github.com/lithammer/shortuuid/v4"
)

// StoreWebhook saves an inbound HTTP request to the database.
// Returns the database ID of the saved request.
func StoreWebhook(method, path, query string, headers map[string][]string, body []byte, contentType, sourceIP string) (string, error) {
	id := shortuuid.New()
	created := time.Now()

	headersJSON, _ := json.Marshal(headers)

	snippet := ""
	if len(body) > 250 {
		snippet = string(body[:250])
	} else if len(body) > 0 {
		snippet = string(body)
	}

	_, err := db.Exec(
		fmt.Sprintf( // #nosec
			`INSERT INTO %s (ID, Method, Path, Query, Headers, Body, BodySize, ContentType, SourceIP, Read, Created, Snippet) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?)`,
			tenant("webhook_requests"),
		),
		id, method, path, query, string(headersJSON), body, int64(len(body)), contentType, sourceIP, created.UnixMilli(), snippet,
	)
	if err != nil {
		return "", err
	}

	c := &WebhookRequestSummary{
		ID:          id,
		Method:      method,
		Path:        path,
		ContentType: contentType,
		SourceIP:    sourceIP,
		BodySize:    int64(len(body)),
		Snippet:     snippet,
		Read:        false,
		Created:     created,
	}

	broadcast("webhook", c)
	sendWebhook(c)

	dbLastAction = time.Now()

	logger.Log().Debugf("[db] saved webhook %s %s %s", id, method, path)

	return id, nil
}

// ListWebhooks returns a subset of webhook requests, sorted latest to oldest.
func ListWebhooks(start, limit int) ([]WebhookRequestSummary, error) {
	results := []WebhookRequestSummary{}

	q := sqlf.From(tenant("webhook_requests")).
		Select(`ID, Method, Path, ContentType, SourceIP, BodySize, Snippet, Read, Created`).
		OrderBy("Created DESC").
		Limit(limit).
		Offset(start)

	if err := q.QueryAndClose(context.TODO(), db, func(row *sql.Rows) {
		var msg WebhookRequestSummary
		var created float64
		var read int

		if err := row.Scan(&msg.ID, &msg.Method, &msg.Path, &msg.ContentType, &msg.SourceIP, &msg.BodySize, &msg.Snippet, &read, &created); err != nil {
			logger.Log().Errorf("[db] %s", err.Error())
			return
		}

		msg.Read = read == 1
		msg.Created = time.UnixMilli(int64(created))
		results = append(results, msg)
	}); err != nil {
		return results, err
	}

	return results, nil
}

// GetWebhook returns a single webhook request by ID.
func GetWebhook(id string) (WebhookRequest, error) {
	var msg WebhookRequest
	var created float64
	var read int
	var headersJSON string
	var body []byte

	err := db.QueryRow(
		fmt.Sprintf(`SELECT ID, Method, Path, Query, Headers, Body, BodySize, ContentType, SourceIP, Read, Created FROM %s WHERE ID = ?`, tenant("webhook_requests")), // #nosec
		id,
	).Scan(&msg.ID, &msg.Method, &msg.Path, &msg.Query, &headersJSON, &body, &msg.BodySize, &msg.ContentType, &msg.SourceIP, &read, &created)
	if err != nil {
		return msg, err
	}

	msg.Read = read == 1
	msg.Created = time.UnixMilli(int64(created))
	msg.Body = string(body)
	if headersJSON != "" {
		_ = json.Unmarshal([]byte(headersJSON), &msg.Headers)
	}

	return msg, nil
}

// MarkWebhookRead marks a webhook request as read.
func MarkWebhookRead(id string) error {
	if _, err := db.Exec(
		fmt.Sprintf(`UPDATE %s SET Read = 1 WHERE ID = ?`, tenant("webhook_requests")), // #nosec
		id,
	); err != nil {
		return err
	}

	d := struct {
		ID   string
		Read bool
	}{ID: id, Read: true}
	broadcast("update", d)

	return nil
}

// DeleteWebhook deletes a single webhook request by ID.
func DeleteWebhook(id string) error {
	_, err := db.Exec(
		fmt.Sprintf(`DELETE FROM %s WHERE ID = ?`, tenant("webhook_requests")), // #nosec
		id,
	)
	if err != nil {
		return err
	}

	broadcast("webhook_delete", id)
	dbLastAction = time.Now()

	return nil
}

// DeleteAllWebhooks deletes all webhook requests.
func DeleteAllWebhooks() error {
	_, err := db.Exec(fmt.Sprintf(`DELETE FROM %s`, tenant("webhook_requests"))) // #nosec
	if err != nil {
		return err
	}

	broadcast("webhook_truncate", nil)
	dbLastAction = time.Now()

	return nil
}

// SearchWebhooks returns webhook requests matching query across Method, Path, Query,
// ContentType, SourceIP and Snippet fields. Returns the matching page and total match count.
func SearchWebhooks(query string, start, limit int) ([]WebhookRequestSummary, int, error) {
	like := "%" + query + "%"
	results := []WebhookRequestSummary{}

	q := sqlf.From(tenant("webhook_requests")).
		Select(`ID, Method, Path, ContentType, SourceIP, BodySize, Snippet, Read, Created`).
		Where(`(Method LIKE ? OR Path LIKE ? OR Query LIKE ? OR ContentType LIKE ? OR SourceIP LIKE ? OR Snippet LIKE ?)`,
			like, like, like, like, like, like).
		OrderBy("Created DESC").
		Limit(limit).
		Offset(start)

	if err := q.QueryAndClose(context.TODO(), db, func(row *sql.Rows) {
		var msg WebhookRequestSummary
		var created float64
		var read int
		if err := row.Scan(&msg.ID, &msg.Method, &msg.Path, &msg.ContentType, &msg.SourceIP, &msg.BodySize, &msg.Snippet, &read, &created); err != nil {
			logger.Log().Errorf("[db] %s", err.Error())
			return
		}
		msg.Read = read == 1
		msg.Created = time.UnixMilli(int64(created))
		results = append(results, msg)
	}); err != nil {
		return results, 0, err
	}

	var count int
	if err := db.QueryRow( // #nosec
		fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE (Method LIKE ? OR Path LIKE ? OR Query LIKE ? OR ContentType LIKE ? OR SourceIP LIKE ? OR Snippet LIKE ?)`, tenant("webhook_requests")),
		like, like, like, like, like, like,
	).Scan(&count); err != nil {
		return results, 0, err
	}

	return results, count, nil
}

// GetWebhookMailboxStats returns total and unread webhook request counts.
func GetWebhookMailboxStats() (WebhookMailboxStats, error) {
	var stats WebhookMailboxStats

	if err := db.QueryRow(
		fmt.Sprintf(`SELECT COUNT(*), COALESCE(SUM(CASE WHEN Read = 0 THEN 1 ELSE 0 END), 0) FROM %s`, tenant("webhook_requests")), // #nosec
	).Scan(&stats.Total, &stats.Unread); err != nil {
		return stats, err
	}

	return stats, nil
}
