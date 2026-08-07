# auth-service

A small HTTP service built with Go's standard `net/http`. It uses a lightweight
module system so features can be added as self-contained packages that plug into
a shared server.

## Requirements

- Go 1.26+
- Docker (for Postgres — **required**; `DATABASE_URL` is mandatory)

## Quick start

1. Start Postgres and auto-apply the schema via Docker Compose (repo root):

   ```bash
   docker compose up -d
   # Wait for the healthcheck to report "healthy"
   docker compose ps
   ```

   The `migrations/` directory is mounted into `/docker-entrypoint-initdb.d` so
   Postgres applies the schema on first volume init automatically.

2. Copy the example env file:

   ```bash
   cd auth-service
   cp .example.env .env
   # .env is git-ignored — never commit it.
   ```

3. Run the service:

   ```bash
   go run ./src
   ```

4. Smoke-test the 11 auth endpoints (see **Endpoints** below).

### Manual migration (if not using Docker Compose)

Apply the schema directly against a running Postgres instance:

```bash
psql "$DATABASE_URL" -f auth-service/migrations/0001_init_limen.up.sql
```

The DDL uses `CREATE TABLE IF NOT EXISTS` / `CREATE INDEX IF NOT EXISTS` so it
is idempotent and safe to run on every deployment.

## Environment variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `DATABASE_URL` | **Yes** | — | Postgres DSN (`postgres://user:pass@host:port/db?sslmode=disable`). **No in-memory fallback**; startup fails fast if unset. |
| `LIMEN_SECRET` | Yes* | — | Signing secret — must be exactly 32 bytes. *Startup **fails closed** if unset unless `AUTH_ALLOW_DEV_SECRET=true`. |
| `AUTH_ALLOW_DEV_SECRET` | No | `false` | Local-dev only. When `true`, permits the built-in insecure dev secret if `LIMEN_SECRET` is unset. Never set outside local dev. |
| `AUTH_BASE_URL` | No | `http://localhost:8080` | Base URL used for cookies and links. |

Configuration is loaded by `setting.Load()` (package `auth-service/src/setting`),
which calls `godotenv.Load()` then validates all required fields via
`github.com/caarlos0/env` before the service starts.

## Build

```bash
cd auth-service
go build ./...
```

## Test

There are two independent test layers:

### Unit tests (no Postgres)

Unit tests run **without Postgres** using a pure-Go in-memory SQLite database
(no CGO, no Docker). The test seam `auth.NewWithDB(db, secret, baseURL)` in
`export_test.go` bypasses the Postgres connection entirely, and
`auth.NewWithHandler(h)` injects a synthetic upstream to exercise controller
success/error/decode branches.

```bash
cd auth-service
go test ./...
```

**Coverage** is focused on the `./src` business logic and gated at **≥ 90%**:

```bash
cd auth-service
./scripts/coverage.sh          # prints per-func report + total, fails under 90%
```

The report intentionally scopes to `./src` and excludes two things that unit
tests cannot meaningfully cover: the `main` entrypoint (`src/main.go`) and the
Postgres connection bootstrap (`src/modules/auth/auth.postgres.go`, which opens
a real database and is covered by the e2e suite instead).

### End-to-end tests (real stack)

Black-box e2e tests run the service as a **real container** (built from
`Dockerfile`) against a **live Postgres**, driving the HTTP API over the
network. They live in `tests/` and are guarded by the `e2e` build tag, so they
are excluded from the unit run above.

```bash
docker compose --profile e2e up -d --build   # from the repo root
cd auth-service
go test -tags=e2e -v ./tests/...
docker compose --profile e2e down -v
```

See [`tests/README.md`](tests/README.md) for details.

## CI

On every pull request to `master` that touches `auth-service/` (or
`docker-compose.yml`), the `auth-pipeline` in
[`.github/workflows/ci.yml`](../.github/workflows/ci.yml) runs **sequentially**
(fail-fast — each stage gates the next):

1. **lint** — `golangci-lint` (v2, standard set).
2. **unit tests** — `scripts/coverage.sh`; fails if `./src` coverage < 90%.
3. **e2e** — `docker compose --profile e2e up` + the tagged suite.
4. **trivy** — filesystem scan (vuln + secret + misconfig), fails on CRITICAL/HIGH.

## Endpoints

### Health

| Method | Path      | Description                          |
| ------ | --------- | ------------------------------------ |
| GET    | `/health` | Liveness check. Returns service status and current UTC time. |

```bash
curl -s http://localhost:8080/health
# {"status":"ok","time":"..."}
```

### Auth (11 endpoints)

| Method | Path | Description |
|---|---|---|
| `POST` | `/auth/signup/credential` | Register a new user with email + password (+ optional username). |
| `POST` | `/auth/signin/credential` | Sign in; returns a `limen_session` cookie. |
| `POST` | `/auth/passwords/request-reset` | Request a password-reset token (sent/returned in response). |
| `POST` | `/auth/passwords/reset` | Reset password using the token from the previous step. |
| `POST` | `/auth/passwords/change` | Change password for the currently authenticated user. |
| `PUT`  | `/auth/passwords` | Set a new password (requires existing session). |
| `POST` | `/auth/usernames/check` | Check whether a username is available. |
| `GET`  | `/auth/me` | Return the current user's profile (requires session cookie). |
| `GET`  | `/auth/sessions` | List all active sessions for the current user. |
| `POST` | `/auth/signout` | Invalidate the current session. |
| `POST` | `/auth/revoke-sessions` | Revoke all sessions for the current user. |

Example round-trip:

```bash
# Sign up
curl -c cookies.txt -s -X POST http://localhost:8080/auth/signup/credential \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"secret123","username":"alice"}'

# Get current user (uses cookie from sign-up)
curl -b cookies.txt -s http://localhost:8080/auth/me

# List sessions
curl -b cookies.txt -s http://localhost:8080/auth/sessions

# Sign out
curl -b cookies.txt -s -X POST http://localhost:8080/auth/signout
```

## Project structure

```text
auth-service/
├── go.mod
├── .example.env                  # committed env template (copy to .env)
├── Dockerfile                    # multi-stage build → distroless (used by e2e + prod)
├── .dockerignore
├── scripts/
│   └── coverage.sh               # src coverage report + ≥90% gate
├── migrations/                   # plain-SQL schema (applied externally)
│   ├── 0001_init_limen.up.sql
│   └── 0001_init_limen.down.sql
├── tests/                        # black-box e2e suite (build tag: e2e)
│   ├── doc.go
│   ├── harness_test.go           # HTTP client + /health readiness wait
│   ├── auth_e2e_test.go
│   └── README.md
└── src/
    ├── main.go                   # boots the server, calls setting.Load(), registers modules
    ├── core/                     # shared, reusable building blocks
    │   └── module.go             # Module interface + RegisterModules(api, ...)
    ├── setting/                  # typed, validated service configuration
    │   └── setting.go            # Setting struct, Load(), resolveSecret() (caarlos0/env)
    └── modules/
        ├── health/               # health-check module
        │   ├── health.dto.go
        │   ├── health.controller.go
        │   └── health.module.go
        └── auth/                 # authentication module (limen-backed)
            ├── auth.dto.go
            ├── auth.controller.go
            ├── auth.postgres.go  # LimenConfig, LimenModule.New — opens Postgres (e2e-covered)
            ├── auth.limen.go     # newLimen — pure limen wiring via GORM adapter (unit-tested)
            ├── auth.module.go    # Config, auth.New(cfg) — delegates to LimenModule.New
            ├── export_test.go    # NewWithDB / NewWithHandler test seams
            └── testdata/
                └── limen_schema_sqlite.sql  # SQLite schema for tests
```

## Architecture

The service is organized around **feature modules**. Each module lives under
`src/modules/<name>/` and owns its routes, handlers, and data types.

- **`core.Module`** — an interface every module implements:

  ```go
  type Module interface {
      Controller() Controller
  }
  ```

- **`core.RegisterModules(api huma.API, ...)`** — wires a list of modules onto
  the huma API.

- **`setting.Load()`** — validates all required environment variables and
  returns a typed `setting.Setting` struct. Called once at startup before any
  module is initialised.

- **`auth.New(cfg auth.Config)`** — delegates to `LimenModule.New`
  (`auth.postgres.go`), which opens Postgres (with a 5-second ping deadline to
  fail fast on bad credentials) and then calls the internal `newLimen` builder
  (`auth.limen.go`) to wrap the connection with limen's official GORM adapter
  (`gormadapter.New(db)`) and construct the limen instance. The split keeps the
  DB-opening bootstrap (e2e-covered) separate from the pure, unit-tested wiring.

### Migrations

The schema is applied externally — **no Go migration code, no AutoMigrate**.

- **Docker Compose**: `auth-service/migrations/` is mounted into
  `/docker-entrypoint-initdb.d` so Postgres runs it on first volume init.
- **Manual**: `psql "$DATABASE_URL" -f auth-service/migrations/0001_init_limen.up.sql`

### Adding a module

1. Create `src/modules/<name>/` with three files:
   `<name>.dto.go`, `<name>.controller.go`, `<name>.module.go`.
2. Give the module a `Module` type with `New()` and a `Controller() core.Controller` method.
3. Add `<name>.New()` to the `core.RegisterModules(...)` call in `src/main.go`.
