-- CREATE SMS mailbox table
CREATE TABLE IF NOT EXISTS {{ tenant "sms_mailbox" }} (
	Sort INTEGER PRIMARY KEY AUTOINCREMENT,
	ID TEXT NOT NULL,
	FromNumber TEXT NOT NULL,
	ToNumber TEXT NOT NULL,
	Body TEXT NOT NULL,
	AccountSID TEXT NOT NULL DEFAULT '',
	Read INTEGER NOT NULL DEFAULT 0,
	Created INTEGER NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS {{ tenant "idx_sms_id" }} ON {{ tenant "sms_mailbox" }} (ID);
CREATE INDEX IF NOT EXISTS {{ tenant "idx_sms_read" }} ON {{ tenant "sms_mailbox" }} (Read);
CREATE INDEX IF NOT EXISTS {{ tenant "idx_sms_created" }} ON {{ tenant "sms_mailbox" }} (Created);
