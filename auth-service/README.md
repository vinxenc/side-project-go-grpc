# auth-service

A small HTTP service built with Go's standard `net/http`. It uses a lightweight
module system so features can be added as self-contained packages that plug into
a shared server.

## Requirements

- Go 1.26+
- Docker (optional — for Postgres persistence)

## Quick start (in-memory, no Docker)

The service falls back to an in-memory store when `DATABASE_URL` is not set.
Data is lost on restart; useful for local dev/testing without Docker.

```bash
cd auth-service
go run ./src
```

The server listens on `:8080`.

## Quick start with Postgres (persistent)

1. Start Postgres via Docker Compose (repo root):

   ```bash
   docker compose up -d
   # Wait for the healthcheck to report "healthy"
   docker compose ps
   ```

2. Copy the example env file and (optionally) edit it:

   ```bash
   cd auth-service
   cp .example.env .env
   # .env is git-ignored — never commit it.
   ```

3. Run the service:

   ```bash
   go run ./src
   ```

   You should see log lines like:

   ```text
   INFO: loaded .env file
   INFO: DATABASE_URL set — connecting to Postgres
   INFO: Postgres adapter ready, migrations applied
   ```

4. Smoke-test all 11 auth endpoints (see **Endpoints** below). Verify
   persistence by restarting `go run ./src` — users and sessions created in
   step 4 must still be accessible.

5. Re-run `go run ./src` a second time to confirm migration idempotency
   (startup must not error on an already-migrated database).

## Environment variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `DATABASE_URL` | No | — | Postgres DSN (`postgres://user:pass@host:port/db?sslmode=disable`). When absent, the in-memory adapter is used. |
| `LIMEN_SECRET` | No | hardcoded dev fallback (insecure) | Signing secret — must be exactly 32 bytes. Override in any non-local environment. |
| `AUTH_BASE_URL` | No | `http://localhost:8080` | Base URL used for cookies and links. |

## Build

```bash
cd auth-service
go build ./...
```

## Test

Tests run **without Postgres** (DATABASE_URL unset → in-memory adapter):

```bash
cd auth-service
go test ./...
```

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
├── core/                         # shared, reusable building blocks
│   ├── module.go                 # Module interface + RegisterModules(mux, ...)
│   └── controller.go             # BaseController with JSON / Error helpers
└── src/
    ├── main.go                   # boots the server, loads .env, registers modules
    └── modules/
        ├── health/               # health-check module
        │   ├── health.dto.go
        │   ├── health.controller.go
        │   └── health.module.go
        └── auth/                 # authentication module (limen-backed)
            ├── auth.dto.go
            ├── auth.controller.go
            ├── auth.limen.go     # limen wiring + adapter selection
            ├── auth.module.go
            └── limenstore/
                ├── memory.go          # in-memory adapter (default / tests)
                ├── postgres.go        # GORM/Postgres adapter
                ├── migrate.go         # embedded DDL migration
                └── migrations/
                    ├── 0001_init_limen.up.sql
                    └── 0001_init_limen.down.sql
```

## Architecture

The service is organized around **feature modules**. Each module lives under
`src/modules/<name>/` and owns its routes, handlers, and data types.

- **`core.Module`** — an interface every module implements:

  ```go
  type Module interface {
      RegisterRoutes(mux *http.ServeMux)
  }
  ```

- **`core.RegisterModules(api huma.API, ...)`** — wires a list of modules onto
  the huma API. Each module's `Controller()` implements
  `RegisterRoutes(api huma.API)`. `main.go` registers modules in one place:

  ```go
  core.RegisterModules(api,
      health.New(),
      authModule,
  )
  ```

- **`core.BaseController`** — shared HTTP helpers (`JSON`, `Error`) that module
  controllers embed to avoid boilerplate.

### Adapter selection

`auth.newDatabaseAdapter()` reads `DATABASE_URL` at startup:

- **Set** → opens a `*gorm.DB` (Postgres via `gorm.io/driver/postgres`), runs
  idempotent DDL migrations, wraps with `limen/adapters/gorm`.
- **Unset/empty** → uses the in-memory `limenstore.MemoryAdapter` (no external
  deps; safe for tests and local iteration).

### Adding a module

1. Create `src/modules/<name>/` with three files:
   `<name>.dto.go`, `<name>.controller.go`, `<name>.module.go`.
2. Give the module a `Module` type with `New()` and a `RegisterRoutes(mux)` method
   (this satisfies `core.Module`).
3. Embed `core.BaseController` in the module's `controller` for the `JSON`/`Error`
   helpers.
4. Add `<name>.New()` to the `core.RegisterModules(...)` call in `src/main.go`.
