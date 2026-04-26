# MessagePit

An email **and SMS** testing tool for developers. Send test emails and SMS messages from your application and inspect them in a clean web UI — nothing reaches real inboxes or real phones.

MessagePit is a fork of [Mailpit](https://github.com/axllent/mailpit) extended with a Twilio-compatible SMS ingest endpoint, an SMS inbox UI, and a dedicated SMS ingest server.

## Features

- **Email**: SMTP server, web UI, REST API, WebSocket live updates, search, tagging, POP3 server
- **SMS**: Twilio-compatible HTTP ingest, SMS inbox with read/unread tracking, live WebSocket updates
- **Shared**: Multi-arch Docker image, optional HTTP basic auth, Prometheus metrics

## Ports

| Port | Protocol | Purpose |
|------|----------|---------|
| 1025 | SMTP | Email ingest (mirrors port 25) |
| 1110 | POP3 | POP3 server (optional) |
| 1775 | HTTP | SMS ingest — Twilio-compatible (mirrors SMPP port 2775) |
| 8025 | HTTP | Web UI and management API |

## Quick Start

```bash
# Docker
docker run -p 1025:1025 -p 1775:1775 -p 8025:8025 ghcr.io/coreydaley/messagepit

# From source
make run
```

Open [http://localhost:8025](http://localhost:8025) in your browser.

## SMS Integration

Point your application's Twilio SDK at the SMS ingest server instead of `api.twilio.com`:

```
http://localhost:1775
```

The SMS server implements the Twilio Messages API endpoint:

```
POST /2010-04-01/Accounts/{AccountSid}/Messages.json
```

Required form fields: `From`, `To`, `Body`.

### Signature validation

If `--sms-auth-token` is set, the server validates the `X-Twilio-Signature` HMAC-SHA1 header on every inbound request. Leave it unset (the default) to accept all messages without validation — suitable for local development.

## Building

Requires Go 1.21+ and Node 22+.

```bash
make run     # build UI + binary and run with dev defaults
make install # build UI + binary and install to $GOPATH/bin
make test    # run Go test suite
make ui      # build frontend assets only
make build   # compile the binary only (requires ui assets)
```

## Configuration

All flags can also be set via environment variables (e.g. `--smtp` → `MP_SMTP_BIND_ADDR`, `--sms` → `MP_SMS_BIND_ADDR`).

| Flag | Default | Description |
|------|---------|-------------|
| `--smtp` | `0.0.0.0:1025` | SMTP bind address |
| `--sms` | `0.0.0.0:1775` | SMS ingest bind address |
| `--listen` | `0.0.0.0:8025` | HTTP UI/API bind address |
| `--db` | *(in-memory)* | SQLite database file path |

Run `messagepit --help` for the full list.

## API

The REST API is documented at [http://localhost:8025/api/v1](http://localhost:8025/api/v1).

SMS endpoints:

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/sms/messages` | List SMS messages |
| GET | `/api/v1/sms/message/{id}` | Get single SMS message |
| PUT | `/api/v1/sms/message/{id}/read` | Mark as read |
| DELETE | `/api/v1/sms/message/{id}` | Delete message |
| DELETE | `/api/v1/sms/messages` | Delete all messages |

## Docker

Images are published to the GitHub Container Registry on every push to `main` and on tagged releases:

```bash
docker pull ghcr.io/coreydaley/messagepit:latest
```

### docker-compose example

```yaml
services:
  messagepit:
    image: ghcr.io/coreydaley/messagepit:latest
    ports:
      - "1025:1025"   # SMTP
      - "1775:1775"   # SMS ingest
      - "8025:8025"   # Web UI
    environment:
      MP_DATABASE: /data/messagepit.db
    volumes:
      - messagepit_data:/data

volumes:
  messagepit_data:
```

## License

MIT — see [LICENSE](LICENSE).

Portions of this project are derived from [Mailpit](https://github.com/axllent/mailpit) by Ralph Slooten, also MIT licensed.
