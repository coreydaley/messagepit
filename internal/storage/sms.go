package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/coreydaley/messagepit/internal/logger"
	"github.com/leporo/sqlf"
	"github.com/lithammer/shortuuid/v4"
)

// StoreSMS saves an inbound SMS message to the database.
// Returns the database ID of the saved message.
func StoreSMS(from, to, body, accountSID string) (string, error) {
	id := shortuuid.New()
	created := time.Now()

	_, err := db.Exec(
		fmt.Sprintf( // #nosec
			`INSERT INTO %s (ID, FromNumber, ToNumber, Body, AccountSID, Read, Created) VALUES (?, ?, ?, ?, ?, 0, ?)`,
			tenant("sms_mailbox"),
		),
		id, from, to, body, accountSID, created.UnixMilli(),
	)
	if err != nil {
		return "", err
	}

	c := &SMSMessageSummary{
		ID:      id,
		From:    from,
		To:      to,
		Body:    body,
		Read:    false,
		Created: created,
	}

	broadcast("sms", c)
	sendWebhook(c)

	dbLastAction = time.Now()

	logger.Log().Debugf("[db] saved SMS %s from %s", id, from)

	return id, nil
}

// ListSMS returns a subset of SMS messages, sorted latest to oldest.
func ListSMS(start, limit int) ([]SMSMessageSummary, error) {
	results := []SMSMessageSummary{}

	q := sqlf.From(tenant("sms_mailbox")).
		Select(`ID, FromNumber, ToNumber, Body, Read, Created`).
		OrderBy("Created DESC").
		Limit(limit).
		Offset(start)

	if err := q.QueryAndClose(context.TODO(), db, func(row *sql.Rows) {
		var msg SMSMessageSummary
		var created float64
		var read int

		if err := row.Scan(&msg.ID, &msg.From, &msg.To, &msg.Body, &read, &created); err != nil {
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

// GetSMS returns a single SMS message by ID.
func GetSMS(id string) (SMSMessage, error) {
	var msg SMSMessage
	var created float64
	var read int

	err := db.QueryRow(
		fmt.Sprintf(`SELECT ID, FromNumber, ToNumber, Body, AccountSID, Read, Created FROM %s WHERE ID = ?`, tenant("sms_mailbox")), // #nosec
		id,
	).Scan(&msg.ID, &msg.From, &msg.To, &msg.Body, &msg.AccountSID, &read, &created)
	if err != nil {
		return msg, err
	}

	msg.Read = read == 1
	msg.Created = time.UnixMilli(int64(created))

	return msg, nil
}

// MarkSMSRead marks one or more SMS messages as read.
func MarkSMSRead(ids []string) error {
	for _, id := range ids {
		if _, err := db.Exec(
			fmt.Sprintf(`UPDATE %s SET Read = 1 WHERE ID = ?`, tenant("sms_mailbox")), // #nosec
			id,
		); err != nil {
			return err
		}

		d := struct {
			ID   string
			Read bool
		}{ID: id, Read: true}
		broadcast("update", d)
	}

	return nil
}

// DeleteSMS deletes a single SMS message by ID.
func DeleteSMS(id string) error {
	_, err := db.Exec(
		fmt.Sprintf(`DELETE FROM %s WHERE ID = ?`, tenant("sms_mailbox")), // #nosec
		id,
	)
	if err != nil {
		return err
	}

	broadcast("sms_delete", id)
	dbLastAction = time.Now()

	return nil
}

// DeleteAllSMS deletes all SMS messages.
func DeleteAllSMS() error {
	_, err := db.Exec(fmt.Sprintf(`DELETE FROM %s`, tenant("sms_mailbox"))) // #nosec
	if err != nil {
		return err
	}

	broadcast("sms_truncate", nil)
	dbLastAction = time.Now()

	return nil
}

// GetSMSMailboxStats returns total and unread SMS counts.
func GetSMSMailboxStats() (SMSMailboxStats, error) {
	var stats SMSMailboxStats

	if err := db.QueryRow(
		fmt.Sprintf(`SELECT COUNT(*), COALESCE(SUM(CASE WHEN Read = 0 THEN 1 ELSE 0 END), 0) FROM %s`, tenant("sms_mailbox")), // #nosec
	).Scan(&stats.Total, &stats.Unread); err != nil {
		return stats, err
	}

	return stats, nil
}
