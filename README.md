# MessagePit

An email **and SMS** testing tool for developers. Send test emails and SMS messages from your application and inspect them in a clean web UI — nothing reaches real inboxes or real phones.

MessagePit is a fork of [Mailpit](https://github.com/axllent/mailpit) extended with a Twilio-compatible SMS ingest endpoint, a SendGrid v3 Mail Send API stub, SMS/email delivery status callbacks, an SMS inbox UI, and a dedicated SMS ingest server.

## Features

- **Email**: SMTP server, SendGrid v3 API stub, web UI, REST API, WebSocket live updates, search, tagging, POP3 server
- **SMS**: Twilio-compatible HTTP ingest, SMS inbox with read/unread tracking, live WebSocket updates
- **Delivery callbacks**: Signed SMS status callbacks and SendGrid-style email event webhooks for end-to-end delivery tracking
- **Shared**: Multi-arch Docker image, optional HTTP basic auth, Prometheus metrics

## Ports

| Port | Protocol | Purpose |
|------|----------|---------|
| 1025 | SMTP | Email ingest (mirrors port 25) |
| 1110 | POP3 | POP3 server (optional) |
| 1775 | HTTP | SMS ingest — Twilio-compatible (mirrors SMPP port 2775) |
| 8025 | HTTP | Web UI, management API, and SendGrid v3 `/v3/mail/send` stub |

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

Required form fields: `From`, `To`, `Body`. Authentication uses HTTP Basic Auth (`AccountSid`:`AuthToken`).

### SMS delivery callbacks

When `MP_SMS_WEBHOOK_URL` is set (or a per-request `StatusCallback` form field is provided), MessagePit fires a signed `POST` to that URL after capturing each message — mirroring how Twilio notifies your app of delivery status.

The callback body is `application/x-www-form-urlencoded` with `MessageSid`, `MessageStatus`, `To`, and `From`. When `MP_SMS_AUTH_TOKEN` is set the request includes an `X-Twilio-Signature` HMAC-SHA1 header so your webhook handler can validate it with the standard Twilio SDK.

**Priority**: the `StatusCallback` field in the send request takes precedence over the global `MP_SMS_WEBHOOK_URL`.

## Email Integration (SendGrid v3)

MessagePit exposes a SendGrid v3 Mail Send stub:

```
POST /v3/mail/send
Authorization: Bearer <MP_SENDGRID_API_KEY>
Content-Type: application/json
```

The endpoint accepts the standard SendGrid v3 JSON payload (`personalizations`, `from`, `subject`, `content`, `custom_args`) and stores each message in the MessagePit mailbox. Authentication is skipped when `MP_SENDGRID_API_KEY` is empty.

### Email delivery webhooks

When `MP_EMAIL_WEBHOOK_URL` is set, MessagePit fires a SendGrid-style event webhook after capturing each email that contains a `notification_id` key in `custom_args`. The webhook payload is a JSON array of event objects:

```json
[
  {
    "notification_id": "<value from custom_args>",
    "event": "delivered",
    "email": "recipient@example.com",
    "timestamp": 1714000000
  }
]
```

Webhooks are signed using ECDSA P-256 / SHA-256, with the signature and timestamp in the same headers real SendGrid uses:

| Header | Description |
|--------|-------------|
| `X-Twilio-Email-Event-Webhook-Signature` | Base64-encoded ECDSA signature |
| `X-Twilio-Email-Event-Webhook-Timestamp` | Unix timestamp as a string |

The payload that is signed is `timestamp + body` (timestamp string concatenated with the raw JSON body).

#### Key pair setup

Provide a stable key pair so the public key doesn't change across restarts:

```bash
# Generate private key (SEC1 DER, base64) — set as MP_EMAIL_WEBHOOK_SIGNING_KEY
openssl ecparam -name prime256v1 -genkey -noout \
  | openssl ec -outform DER 2>/dev/null \
  | base64

# Derive the matching public key (PKIX DER, base64) — set as SENDGRID_WEBHOOK_PUBLIC_KEY in your app
openssl ecparam -name prime256v1 -genkey -noout \
  | openssl ec -outform DER 2>/dev/null \
  | openssl ec -inform DER -pubout -outform DER 2>/dev/null \
  | base64
```

If `MP_EMAIL_WEBHOOK_SIGNING_KEY` is empty and `MP_EMAIL_WEBHOOK_URL` is set, MessagePit auto-generates a key pair at startup and logs the public key — useful for one-off testing but not stable across restarts.

#### SMTP vs. SendGrid v3

The email webhook only fires for messages received via the `/v3/mail/send` endpoint, since only that path carries `custom_args`. Emails delivered over SMTP do not carry `custom_args` and will not trigger the webhook.

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

| Flag | Env var | Default | Description |
|------|---------|---------|-------------|
| `--smtp` | `MP_SMTP_BIND_ADDR` | `0.0.0.0:1025` | SMTP bind address |
| `--sms` | `MP_SMS_BIND_ADDR` | `0.0.0.0:1775` | SMS ingest bind address |
| `--listen` | `MP_UI_BIND_ADDR` | `0.0.0.0:8025` | HTTP UI/API bind address |
| `--db` | `MP_DATABASE` | *(in-memory)* | SQLite database file path |
| `--sms-auth-token` | `MP_SMS_AUTH_TOKEN` | | Twilio auth token — validates Basic Auth on inbound SMS; signs outgoing delivery callbacks |
| `--sms-webhook-url` | `MP_SMS_WEBHOOK_URL` | | URL to POST SMS delivery callbacks to (fallback when no per-request `StatusCallback`) |
| `--sendgrid-api-key` | `MP_SENDGRID_API_KEY` | | Expected Bearer token for `/v3/mail/send` (skipped when empty) |
| `--email-webhook-url` | `MP_EMAIL_WEBHOOK_URL` | | URL to POST email delivery event webhooks to |
| `--email-webhook-signing-key` | `MP_EMAIL_WEBHOOK_SIGNING_KEY` | | Base64-encoded SEC1 DER ECDSA P-256 private key (auto-generated when empty) |

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

The example below enables full delivery callback support for both SMS and email. The `extra_hosts` entry is required on Linux so `host.docker.internal` resolves to the host machine (Docker Desktop handles this automatically on Mac and Windows).

```yaml
services:
  messagepit:
    image: ghcr.io/coreydaley/messagepit:latest
    restart: always
    extra_hosts:
      - "host.docker.internal:host-gateway"
    ports:
      - "1025:1025"   # SMTP
      - "1775:1775"   # SMS ingest (Twilio-compatible)
      - "8025:8025"   # Web UI + SendGrid v3 stub
    environment:
      # SMS — must match TWILIO_AUTH_TOKEN in your app
      MP_SMS_AUTH_TOKEN: your-twilio-auth-token
      # Fallback SMS callback URL (per-request StatusCallback takes priority)
      MP_SMS_WEBHOOK_URL: http://host.docker.internal:3000/webhooks/v1/sms

      # SendGrid v3 stub — must match SENDGRID_API_KEY in your app
      MP_SENDGRID_API_KEY: your-sendgrid-api-key
      # Email delivery webhook URL
      MP_EMAIL_WEBHOOK_URL: http://host.docker.internal:3000/webhooks/v1/email
      # ECDSA private key for signing email webhooks (generate with openssl, see above)
      MP_EMAIL_WEBHOOK_SIGNING_KEY: "<base64-encoded SEC1 DER private key>"
    healthcheck:
      test: ["CMD", "/messagepit", "readyz"]
      interval: 10s
      timeout: 5s
      retries: 5
```

## Rails Integration

MessagePit is designed to be a drop-in local replacement for Twilio and SendGrid, using the same official gems your production app uses.

### Gems

```ruby
# Gemfile
gem "twilio-ruby", "~> 7"
gem "sendgrid-actionmailer"
```

### Environment variables (`.env.development`)

```bash
# SMS
TWILIO_ACCOUNT_SID=test
TWILIO_AUTH_TOKEN=test           # must match MP_SMS_AUTH_TOKEN
TWILIO_ACCOUNT_NUMBER=+15550000000
TWILIO_API_URL=http://localhost:1775
# Callback URL must use host.docker.internal so MessagePit (in Docker)
# can reach your Rails app running on the host.
TWILIO_STATUS_CALLBACK_URL=http://host.docker.internal:3000/webhooks/v1/sms

# Email
SENDGRID_API_KEY=test            # must match MP_SENDGRID_API_KEY
SENDGRID_API_URL=http://localhost:8025
# PKIX DER base64 public key matching MP_EMAIL_WEBHOOK_SIGNING_KEY
SENDGRID_WEBHOOK_PUBLIC_KEY=<base64-encoded PKIX DER public key>
```

### Action Mailer (`config/environments/development.rb`)

```ruby
config.action_mailer.delivery_method = :sendgrid_actionmailer
config.action_mailer.sendgrid_actionmailer_settings = {
  api_key:               ENV.fetch("SENDGRID_API_KEY", "test"),
  host:                  ENV.fetch("SENDGRID_API_URL", "https://api.sendgrid.com"),
  raise_delivery_errors: true
}
```

### SMS sender

Route `twilio-ruby` to MessagePit by rewriting the API base URL. When `TWILIO_API_URL` is not set or equals `https://api.twilio.com` the real Twilio API is used.

```ruby
require "twilio-ruby"

class SmsSender
  def self.call(to:, body:)
    sid      = ENV.fetch("TWILIO_ACCOUNT_SID")
    token    = ENV.fetch("TWILIO_AUTH_TOKEN")
    from     = ENV.fetch("TWILIO_ACCOUNT_NUMBER")
    callback = ENV["TWILIO_STATUS_CALLBACK_URL"]

    client = build_client(sid, token)
    params = { body: body, from: from, to: to }
    params[:status_callback] = callback if callback.present?

    message = client.messages.create(**params)
    { status: "Sent", code: 201, sid: message.sid }
  end

  def self.build_client(sid, token)
    base_url = ENV.fetch("TWILIO_API_URL", "https://api.twilio.com")
    return Twilio::REST::Client.new(sid, token) if base_url == "https://api.twilio.com"

    Twilio::REST::Client.new(sid, token, nil, nil, ProxyHttpClient.new(base_url))
  end

  # Rewrites the Twilio API host so the gem can target MessagePit.
  class ProxyHttpClient < Twilio::HTTP::Client
    def initialize(base_url)
      super()
      uri         = URI.parse(base_url)
      @proxy_host = "#{uri.scheme}://#{uri.host}"
      @proxy_base = "#{uri.scheme}://#{uri.host}:#{uri.port}"
      @proxy_port = uri.port
    end

    def request(host, port, method, url, params = {}, data = {}, headers = {}, auth = nil, timeout = nil)
      rewritten_url = url.sub(%r{\Ahttps?://[^/]+}, @proxy_base)
      super(@proxy_host, @proxy_port, method, rewritten_url, params, data, headers, auth, timeout)
    end
  end
end
```

### Email delivery tracking

Set `custom_args: { notification_id: record.id.to_s }` in your `mail()` call. MessagePit extracts this value and includes it in the webhook payload so your app can update the delivery status on the corresponding record.

```ruby
mail(
  to:          recipient,
  subject:     "Your subject",
  custom_args: { notification_id: @notification.id.to_s }
)
```

### Webhook controllers

**SMS** — validate with `Twilio::Security::RequestValidator`:

```ruby
require "twilio-ruby"

class SmsWebhookController < ApplicationController
  skip_before_action :verify_authenticity_token

  before_action :verify_twilio_signature

  def update
    notification = Notification.find_by(sms_id: params[:MessageSid])
    notification&.update_columns(sms_delivery_status: params[:MessageStatus])
    head :ok
  end

  private

  def verify_twilio_signature
    token     = ENV["TWILIO_AUTH_TOKEN"]
    validator = Twilio::Security::RequestValidator.new(token)
    unless validator.validate(request.original_url, request.POST, request.headers["X-Twilio-Signature"].to_s)
      head :forbidden
    end
  end
end
```

**Email** — validate with ECDSA using the `SENDGRID_WEBHOOK_PUBLIC_KEY`:

```ruby
class EmailWebhookController < ApplicationController
  skip_before_action :verify_authenticity_token

  before_action :verify_sendgrid_signature

  def update
    (params["_json"] || []).each do |event|
      next unless event["event"] == "delivered" && event["notification_id"].present?
      Notification.find_by(id: event["notification_id"])
                  &.update_columns(email_delivery_status: "delivered")
    end
    head :ok
  end

  private

  def verify_sendgrid_signature
    public_key = OpenSSL::PKey.read(Base64.decode64(ENV["SENDGRID_WEBHOOK_PUBLIC_KEY"]))
    signature  = Base64.decode64(request.headers["X-Twilio-Email-Event-Webhook-Signature"].to_s)
    payload    = request.headers["X-Twilio-Email-Event-Webhook-Timestamp"].to_s + request.raw_post
    head :forbidden unless public_key.verify(OpenSSL::Digest::SHA256.new, signature, payload)
  rescue OpenSSL::PKey::PKeyError, ArgumentError
    head :forbidden
  end
end
```

## License

MIT — see [LICENSE](LICENSE).

Portions of this project are derived from [Mailpit](https://github.com/axllent/mailpit) by Ralph Slooten, also MIT licensed.
