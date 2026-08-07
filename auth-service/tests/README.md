# auth-service e2e tests

End-to-end tests that drive the real HTTP API (`health` + `auth` modules) over a
loopback socket against a **live Postgres**, using the production `auth.New`
wiring. A real `http.Client` with a cookie jar carries the `limen_session`
cookie across requests, so multi-step journeys (sign up → me → sessions → sign
out, sign in, change password, …) are exercised exactly as a client would.

These differ from the handler-level unit tests under `src/modules/auth`, which
use an in-memory SQLite database and need no external services.

## Build tag

The suite is guarded by the `e2e` build tag, so it is **excluded** from the
DB-free unit run (`go test ./...`). `doc.go` carries no tag so the package
always has one buildable file.

## Running locally

Start Postgres (from the repo root) and run the tagged suite:

```bash
docker compose up -d          # Postgres on localhost:5432 (see docker-compose.yml)
cd auth-service
go test -tags=e2e -v ./tests/...
```

The suite applies `migrations/0001_init_limen.up.sql` itself (idempotent) and
boots the server in-process — no manual schema step required.

## Configuration

| Env var | Default | Purpose |
|---|---|---|
| `DATABASE_URL` | `postgres://auth:auth@localhost:5432/authdb?sslmode=disable` | Postgres DSN the server and migrator connect to. |
| `AUTH_MIGRATIONS_PATH` | `../migrations/0001_init_limen.up.sql` | Schema file applied before the suite runs. |

## CI

The `auth-pipeline / e2e` job in [`.github/workflows/ci.yml`](../../.github/workflows/ci.yml)
runs this suite against a `postgres:16-alpine` service container on every PR to
`master` that touches `auth-service/`.
