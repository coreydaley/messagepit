# MessagePit

An email **and SMS** testing tool for developers. Send test emails and SMS messages from your application and inspect them in a clean web UI — nothing reaches real inboxes or real phones.

MessagePit is a fork of [Mailpit](https://github.com/axllent/mailpit) extended with a Twilio-compatible SMS ingest endpoint, a SendGrid v3 Mail Send API stub, SMS/email delivery status callbacks, an SMS inbox UI, and a dedicated SMS ingest server.

## Features

- **Email**: SMTP server, SendGrid v3 API stub, web UI, REST API, WebSocket live updates, search, tagging, POP3 server
- **SMS**: Twilio-compatible HTTP ingest, SMS inbox with read/unread tracking, live WebSocket updates
- **Webhook capture**: Dedicated HTTP server that captures any incoming request on any path/method and displays it in the UI — useful for inspecting outbound webhook calls from your app in development
- **Delivery callbacks**: Signed SMS status progressions (`queued` → `sent` → `delivered`) and full SendGrid event lifecycles, with magic numbers and scenario triggers for the failure branches
- **Shared**: Multi-arch Docker image, optional HTTP basic auth, Prometheus metrics

## Ports

| Port | Protocol | Purpose |
|------|----------|---------|
| 1025 | SMTP | Email ingest (mirrors port 25) |
| 1110 | POP3 | POP3 server (optional) |
| 8025 | HTTP | Web UI and management API |
| 8100 | HTTP | SendGrid v3 Mail Send stub — `POST /v3/mail/send` |
| 8200 | HTTP | SMS ingest — Twilio-compatible |
| 8300 | HTTP | Webhook capture — accepts any request on any path/method |

## Quick Start

```bash
# Docker
docker run -p 1025:1025 -p 8025:8025 -p 8100:8100 -p 8200:8200 -p 8300:8300 ghcr.io/coreydaley/messagepit

# From source
make run
```

Open [http://localhost:8025](http://localhost:8025) in your browser.

## SMS Integration

Point your application's Twilio SDK at the SMS ingest server instead of `api.twilio.com`:

```
http://localhost:8200
```

The SMS server implements the Twilio Messages API endpoint:

```
POST /2010-04-01/Accounts/{AccountSid}/Messages.json
```

Required form fields: `From`, `To`, `Body`. Authentication uses HTTP Basic Auth (`AccountSid`:`AuthToken`).

### SMS status callbacks

When `MP_TWILIO_WEBHOOK_URL` is set (or a per-request `StatusCallback` form field is provided), MessagePit replays the message's full **status progression** to that URL — one signed `POST` per state change, exactly as Twilio does:

```
queued → sent → delivered
```

The `POST /Messages.json` response still reports `"status": "queued"`, matching real Twilio: the terminal state only ever arrives via callback.

Each callback body is `application/x-www-form-urlencoded`:

| Field | Notes |
|---|---|
| `MessageSid` / `SmsSid` | Stable across the whole progression |
| `MessageStatus` / `SmsStatus` | `queued`, `sent`, `delivered`, `undelivered`, `failed` |
| `To`, `From` | As submitted |
| `AccountSid` | From the request path |
| `ApiVersion` | `2010-04-01` |
| `ErrorCode`, `ErrorMessage` | Present only on `failed` / `undelivered` |

When `MP_TWILIO_AUTH_TOKEN` is set, every callback carries an `X-Twilio-Signature` HMAC-SHA1 header so your handler can validate it with the standard Twilio SDK.

**Priority**: the `StatusCallback` field in the send request takes precedence over the global `MP_TWILIO_WEBHOOK_URL`.

Set `--twilio-callback-delay` (`MP_TWILIO_CALLBACK_DELAY`, e.g. `750ms`) to space the callbacks out. The default is `0` — the states still arrive in order, just without the wall-clock gap.

#### Magic numbers

Twilio reserves the `+1500555xxxx` range for test numbers. MessagePit honours the real ones and adds its own in the same range for the delivery-failure branches, which real Twilio has no way to trigger on demand.

**API rejections** (real Twilio behaviour) — the send returns HTTP 400 with a Twilio error code, nothing is stored, and no callback fires:

| Number | Field | Code | Meaning |
|---|---|---|---|
| `+15005550001` | `To` | 21211 | Not a valid phone number |
| `+15005550002` | `To` | 21612 | Not currently reachable via SMS |
| `+15005550003` | `To` | 21408 | Region not enabled for SMS |
| `+15005550004` | `To` | 21610 | Unsubscribed recipient |
| `+15005550009` | `To` | 21614 | Not a valid mobile number |
| `+15005550001` | `From` | 21212 | Not a valid sender |
| `+15005550007` | `From` | 21606 | Not an SMS-capable number on this account |
| `+15005550008` | `From` | 21611 | Sender queue full |

**Delivery failures** (MessagePit extension) — the send succeeds; the failure surfaces through the callback progression:

| `To` number | Progression | `ErrorCode` |
|---|---|---|
| `+15005550010` | `queued` → `failed` | 30008 (unknown error) |
| `+15005550011` | `queued` → `sent` → `undelivered` | 30003 (unreachable handset) |
| `+15005550012` | `queued` → `sent` → `undelivered` | 30005 (unknown handset) |
| `+15005550013` | `queued` → `sent` → `undelivered` | 30006 (landline / unreachable carrier) |
| `+15005550014` | `queued` → `sent` | — (carrier never confirms) |

## Webhook Capture

MessagePit runs a dedicated HTTP server (port 8300 by default) that accepts any incoming HTTP request on any path and method, stores it, and displays it in the **Webhooks** tab of the UI. This is useful for developing and testing outbound webhook delivery from your application without needing a public endpoint.

Point your webhook URL at the capture server:

```
http://localhost:8300/any/path/you/like
```

Any HTTP method works: `POST`, `GET`, `PUT`, `PATCH`, `DELETE`. The full request — method, path, headers, body, source IP — is captured and displayed in real time via WebSocket.

The capture server is enabled by default. Set `--webhook ""` (or `MP_WEBHOOK_BIND_ADDR=""`) to disable it.

### Webhook capture API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/webhooks` | List captured requests (paginated) |
| GET | `/api/v1/webhooks/search` | Search captured requests |
| GET | `/api/v1/webhook/{id}` | Get a single captured request |
| DELETE | `/api/v1/webhook/{id}` | Delete a single captured request |
| DELETE | `/api/v1/webhooks` | Delete all captured requests |

## Email Integration (SendGrid v3)

MessagePit exposes a SendGrid v3 Mail Send stub on port 8100:

```
POST /v3/mail/send
Authorization: Bearer <MP_SENDGRID_API_KEY>
Content-Type: application/json
```

Point your application's SendGrid SDK at the stub by setting the API base URL to `http://localhost:8100`. The endpoint accepts the standard SendGrid v3 JSON payload (`personalizations`, `from`, `subject`, `content`, `custom_args`) and stores each message in the MessagePit mailbox. Authentication is skipped when `MP_SENDGRID_API_KEY` is empty.

The SendGrid server defaults to `127.0.0.1:8100` (loopback only). Set `MP_SENDGRID_BIND_ADDR=0.0.0.0:8100` to expose it on all interfaces (e.g. inside Docker). Set `MP_SENDGRID_BIND_ADDR=""` to disable it entirely.

### Email event webhooks

When `MP_EMAIL_WEBHOOK_URL` is set, MessagePit replays the **full SendGrid event lifecycle** for every captured email — one `POST` per event, in order:

```
processed → delivered
```

Real SendGrid fires its event webhook for everything it accepts, over both the v3 API and its SMTP relay, so MessagePit does the same. No `custom_args` are required.

Each request body is a JSON array holding a single event (SendGrid batches events that land in the same window; MessagePit sends them individually so the ordering stays visible in development):

```json
[
  {
    "email": "recipient@example.com",
    "timestamp": 1714000000,
    "smtp-id": "<abc123@messagepit>",
    "event": "delivered",
    "response": "250 2.0.0 OK",
    "category": ["welcome"],
    "sg_event_id": "rbtnWrG1DVDGGGFHFdun0A",
    "sg_message_id": "142d9f3f351f2dad77c8.messagepit",
    "notification_id": "<value from custom_args>"
  }
]
```

Every key in `custom_args` is flattened onto each event as a top-level field, exactly as real SendGrid does — so `notification_id` keeps working unchanged. `categories` from the v3 payload arrive as `category`.

Event-specific fields match the real API: `response` on `delivered`/`deferred`, `reason` + `status` + `type` + `bounce_classification` on `bounce`, `reason` + `status` on `dropped`, `useragent` + `ip` on `open`/`click`, and `url` + `url_offset` on `click`.

#### Delivery scenarios

The default lifecycle is `processed → delivered`. To exercise the failure and engagement branches, select a scenario either with a `mp_scenario` custom arg or by addressing the recipient with a matching local part or plus-tag (`bounce@example.com`, `user+bounce@example.com`). An explicit `mp_scenario` wins over the address.

| Scenario | Event sequence |
|---|---|
| `delivered` (default) | `processed` → `delivered` |
| `open` | `processed` → `delivered` → `open` |
| `click` | `processed` → `delivered` → `open` → `click` |
| `deferred` | `processed` → `deferred` → `delivered` |
| `bounce` | `processed` → `bounce` (`type: bounce`, 5.1.1) |
| `blocked` | `processed` → `bounce` (`type: blocked`, 5.7.1) |
| `dropped` | `processed` → `dropped` |
| `spamreport` | `processed` → `delivered` → `spamreport` |
| `unsubscribe` | `processed` → `delivered` → `unsubscribe` |
| `group_unsubscribe` | `processed` → `delivered` → `group_unsubscribe` |
| `group_resubscribe` | `processed` → `delivered` → `group_resubscribe` |

```json
{
  "personalizations": [{ "to": [{ "email": "user@example.com" }] }],
  "custom_args": { "notification_id": "42", "mp_scenario": "bounce" }
}
```

Set `--email-webhook-event-delay` (`MP_EMAIL_WEBHOOK_EVENT_DELAY`, e.g. `2s`) to space the events out. The default is `0`.

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

Events fire for **both** paths, mirroring SendGrid's own SMTP relay. Messages sent over SMTP simply have no `custom_args` to flatten onto the events unless you set them yourself: MessagePit reads SendGrid's `X-SMTPAPI` header (`{"category":[...],"unique_args":{...}}`), which the v3 handler also uses internally to carry that metadata through storage. Scenario selection by recipient address works over SMTP too, or set `X-MessagePit-Scenario` directly.

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

All flags can also be set via environment variables (e.g. `--smtp` → `MP_SMTP_BIND_ADDR`, `--twilio` → `MP_TWILIO_BIND_ADDR`).

| Flag | Env var | Default | Description |
|------|---------|---------|-------------|
| `--smtp` | `MP_SMTP_BIND_ADDR` | `0.0.0.0:1025` | SMTP bind address |
| `--sendgrid` | `MP_SENDGRID_BIND_ADDR` | `127.0.0.1:8100` | SendGrid v3 Mail Send API bind address (empty to disable) |
| `--sendgrid-api-key` | `MP_SENDGRID_API_KEY` | | Expected Bearer token for `/v3/mail/send` (skipped when empty) |
| `--twilio` | `MP_TWILIO_BIND_ADDR` | `[::]:8200` | Twilio SMS ingest bind address |
| `--twilio-auth-token` | `MP_TWILIO_AUTH_TOKEN` | | Twilio auth token — validates Basic Auth on inbound SMS; signs outgoing delivery callbacks |
| `--twilio-webhook-url` | `MP_TWILIO_WEBHOOK_URL` | | URL to POST SMS status callbacks to (fallback when no per-request `StatusCallback`) |
| `--twilio-callback-delay` | `MP_TWILIO_CALLBACK_DELAY` | `0s` | Delay between callbacks in the `queued`/`sent`/`delivered` progression |
| `--webhook` | `MP_WEBHOOK_BIND_ADDR` | `[::]:8300` | HTTP webhook capture bind address (empty to disable) |
| `--listen` | `MP_UI_BIND_ADDR` | `0.0.0.0:8025` | HTTP UI/API bind address |
| `--db` | `MP_DATABASE` | *(in-memory)* | SQLite database file path |
| `--email-webhook-url` | `MP_EMAIL_WEBHOOK_URL` | | URL to POST email delivery event webhooks to |
| `--email-webhook-signing-key` | `MP_EMAIL_WEBHOOK_SIGNING_KEY` | | Base64-encoded SEC1 DER ECDSA P-256 private key (auto-generated when empty) |
| `--email-webhook-event-delay` | `MP_EMAIL_WEBHOOK_EVENT_DELAY` | `0s` | Delay between events in the email delivery lifecycle |

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
      - "8025:8025"   # Web UI
      - "8100:8100"   # SendGrid v3 Mail Send stub
      - "8200:8200"   # SMS ingest (Twilio-compatible)
      - "8300:8300"   # Webhook capture
    environment:
      # SendGrid v3 stub — must match SENDGRID_API_KEY in your app
      MP_SENDGRID_API_KEY: your-sendgrid-api-key
      # Expose SendGrid on all interfaces inside the container
      MP_SENDGRID_BIND_ADDR: "0.0.0.0:8100"

      # Twilio SMS — must match TWILIO_AUTH_TOKEN in your app
      MP_TWILIO_AUTH_TOKEN: your-twilio-auth-token
      # Fallback SMS callback URL (per-request StatusCallback takes priority)
      MP_TWILIO_WEBHOOK_URL: http://host.docker.internal:3000/webhooks/v1/sms
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
TWILIO_AUTH_TOKEN=test           # must match MP_TWILIO_AUTH_TOKEN
TWILIO_ACCOUNT_NUMBER=+15550000000
TWILIO_API_URL=http://localhost:8200
# Callback URL must use host.docker.internal so MessagePit (in Docker)
# can reach your Rails app running on the host.
TWILIO_STATUS_CALLBACK_URL=http://host.docker.internal:3000/webhooks/v1/sms

# Email
SENDGRID_API_KEY=test            # must match MP_SENDGRID_API_KEY
SENDGRID_API_URL=http://localhost:8100
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

Set `custom_args: { notification_id: record.id.to_s }` in your `mail()` call. MessagePit flattens every custom arg onto each event in the lifecycle so your app can update the delivery status on the corresponding record. Add `mp_scenario` to drive a bounce, deferral, or open/click instead of a plain delivery.

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

  # Called once per state change: queued, sent, then delivered — or failed /
  # undelivered with an ErrorCode.
  def update
    notification = Notification.find_by(sms_id: params[:MessageSid])
    notification&.update_columns(
      sms_delivery_status: params[:MessageStatus],
      sms_error_code:      params[:ErrorCode]
    )
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

  # Every event in the lifecycle arrives here — processed, delivered, bounce,
  # dropped, deferred, open, click — so record the status rather than matching
  # on a single event type.
  TERMINAL_EVENTS = %w[delivered bounce dropped spamreport].freeze

  def update
    (params["_json"] || []).each do |event|
      next unless TERMINAL_EVENTS.include?(event["event"])
      next if event["notification_id"].blank?

      Notification.find_by(id: event["notification_id"])
                  &.update_columns(email_delivery_status: event["event"])
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
