# MessagePit

A **digital twin universe** for third-party messaging APIs. Each provider stub is a faithful local replica of the real provider API — same endpoints, same auth semantics, same response shapes, same error codes — so developers build against real SDKs without touching real services.

## Project Layout

```
internal/
  mailadapter/     shared security helpers consumed by all email providers
  sendgrid/        SendGrid v3 Mail Send API stub         (port 8100)
  twilio/          Twilio SMS Messages API stub           (port 8200)
  webhookd/        generic HTTP capture server            (port 8300)
config/            application configuration
server/            HTTP server bootstrap (one listener per provider)
cmd/               CLI entry point (cobra)
docs/sprints/      sprint plans, retros, and the sprint ledger (sprints.tsv)
```

## Port Scheme

Ports are grouped by provider type. The next provider in a range gets the next sequential number — no gaps.

| Range     | Type                | Assignments                          |
|-----------|---------------------|--------------------------------------|
| 1025      | SMTP                | email ingest                         |
| 1110      | POP3                | POP3 server                          |
| 8025      | HTTP                | Web UI + management API              |
| 8100–8199 | Email API stubs     | SendGrid=8100                        |
| 8200–8299 | SMS/messaging stubs | Twilio=8200                          |
| 8300–8399 | Capture/utility     | Webhook capture=8300                 |

## Provider Pattern

Every provider follows this structure exactly.

### Files to create

| File | Purpose |
|---|---|
| `internal/<provider>/<provider>.go` | Handler — calls mailadapter helpers, decodes body, validates, stores |
| `internal/<provider>/<provider>_test.go` | Hand-written behavioral tests |

### Files to update

| File | What to add |
|---|---|
| `config/config.go` | `<Provider>Listen` default (next port in range), `<Provider>APIKey`; validation block in `VerifyConfig()` |
| `cmd/root.go` | `--<provider>` flag, `MP_<PROVIDER>_BIND_ADDR` env var, `go server.Listen<Provider>()` bootstrap |
| `server/server.go` | `Listen<Provider>()` — exact same pattern as `ListenSendGrid()` |
| `Dockerfile` | `<port>/tcp` in `EXPOSE` |
| `Makefile` | `--<provider> 0.0.0.0:<port>` in `run` and `dev` targets |
| `README.md` | Ports table, integration section, config table, docker-compose example |

### Handler skeleton

Every email provider handler starts with:

```go
func CreateMessage(w http.ResponseWriter, r *http.Request) {
    mailadapter.LimitBody(w, r)
    if !mailadapter.BearerAuth(w, r, config.<Provider>APIKey, "[<provider>]") {
        return
    }
    // decode body, validate fields, call storage.Store(...)
}
```

## Shared Helpers (`internal/mailadapter/`)

All email providers must use these. Never reimplement in a provider package.

| Helper | Contract |
|---|---|
| `LimitBody` | 10 MiB cap via `http.MaxBytesReader` |
| `BearerAuth` | constant-time compare; bypasses when `key == ""` — preserves local dev flow |
| `FormatAddress` | `mail.ParseAddress` + `mime.QEncoding.Encode` for display names |
| `SanitizeHeaderValue` | strips `\r` and `\n` independently — not as a pair |
| `JSONError` | sets `Content-Type: application/json` **before** `WriteHeader` |

## Testing

- Use real in-memory SQLite — `storage.InitDB()` in `setup()`. Never mock the database.
- Suppress log output — `logger.NoLogging = true` in `setup()` or `init()`.
- Header injection assertions check `"\n<Header>: value"` not bare substring.
- `go test ./...` must be green before every commit.

### Test file ownership

| File | Owner | Purpose |
|---|---|---|
| `<provider>_test.go` | human | behavioral decisions, edge cases, integration |
| `mailadapter_test.go` | human | helper unit tests |

## Digital Twin Philosophy

Provider stubs aim for full behavioral fidelity with the real API, not just "good enough." An application configured to point at MessagePit instead of the real provider should behave identically during development.

## Sprint Workflow

Planned work is tracked in `docs/sprints/`. Use `/sprint-plan` to plan and `/sprint-work` to execute. The ledger is at `docs/sprints/sprints.tsv`.
