-- CREATE webhook_requests table
CREATE TABLE IF NOT EXISTS {{ tenant "webhook_requests" }} (
	Sort INTEGER PRIMARY KEY AUTOINCREMENT,
	ID TEXT NOT NULL,
	Method TEXT NOT NULL,
	Path TEXT NOT NULL DEFAULT '/',
	Query TEXT NOT NULL DEFAULT '',
	Headers TEXT NOT NULL DEFAULT '{}',
	Body BLOB,
	BodySize INTEGER NOT NULL DEFAULT 0,
	ContentType TEXT NOT NULL DEFAULT '',
	SourceIP TEXT NOT NULL DEFAULT '',
	Read INTEGER NOT NULL DEFAULT 0,
	Created INTEGER NOT NULL,
	Snippet TEXT NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX IF NOT EXISTS {{ tenant "idx_webhook_id" }} ON {{ tenant "webhook_requests" }} (ID);
CREATE INDEX IF NOT EXISTS {{ tenant "idx_webhook_read" }} ON {{ tenant "webhook_requests" }} (Read);
CREATE INDEX IF NOT EXISTS {{ tenant "idx_webhook_created" }} ON {{ tenant "webhook_requests" }} (Created);
