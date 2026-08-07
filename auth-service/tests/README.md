# auth-service e2e tests

Black-box end-to-end tests. The auth-service runs as a **real container**
(built from [`../Dockerfile`](../Dockerfile)) backed by a **live Postgres**, and
the suite drives its HTTP API over the published port with a real `http.Client`
and cookie jar. The `limen_session` cookie set on sign-up/sign-in flows across
requests automatically, so multi-step journeys (sign up → me → sessions → sign
out, sign in, change password, …) are exercised exactly as a client would.

The suite itself starts no server and touches no database directly — it is a
pure HTTP client. Compose owns the stack; Postgres applies the schema on init.

These differ from the handler-level unit tests under `src/modules/auth`, which
use an in-memory SQLite database and need no external services.

## Layout

The suites mirror the service modules under `src/modules/`:

```text
tests/
├── doc.go            # package docs + no-tag sentinel
├── harness.go        # shared harness (e2e-tagged): Client, Ready, helpers
├── README.md
└── modules/
    ├── auth/         # auth API e2e suite (package auth_test)
    │   ├── doc.go
    │   └── auth_e2e_test.go
    └── health/       # health API e2e suite (package health_test)
        ├── doc.go
        └── health_e2e_test.go
```

Each module suite imports the shared harness (`auth-service/tests`) for the HTTP
client, the readiness gate, and helpers. To add a suite for a new module, create
`tests/modules/<name>/` with a no-tag `doc.go` and an e2e-tagged `<name>_e2e_test.go`
whose `TestMain` calls `tests.Ready()`.

## Build tag

The suites are guarded by the `e2e` build tag, so they are **excluded** from the
DB-free unit run (`go test ./...`). Every package carries a no-tag `doc.go` so it
always has one buildable file when the tag is absent.

## Running locally

Bring the full stack up (from the repo root), then run the tagged suite:

```bash
docker compose --profile e2e up -d --build   # postgres + auth-service on :8080
cd auth-service
go test -tags=e2e -v ./tests/...
docker compose --profile e2e down -v         # tear down when done
```

The plain `docker compose up -d` (no profile) still starts **Postgres only**,
as documented in the top-level README — the app container is gated behind the
`e2e` profile so local dev is unaffected.

## Configuration

| Env var | Default | Purpose |
|---|---|---|
| `BASE_URL` | `http://localhost:8080` | Origin of the running auth-service the suite drives. |

Each suite's `TestMain` calls `tests.Ready()`, which polls `GET /health` until
the service is ready (up to ~90s) before running any test, so it tolerates a
container that is still starting.

> **Re-running locally:** limen enforces a request rate limit whose counters
> persist in Postgres, so repeatedly running the suite against the *same*
> long-lived stack can eventually return `429 Too Many Requests`. Start from a
> fresh database (`docker compose --profile e2e down -v` then `up`) between runs.
> CI is unaffected — it always spins up a clean volume. Auth-rejection tests
> assert a 4xx (not an exact code) so a rate-limit 429 never fails them.

## CI

The `auth-pipeline / e2e` job in [`.github/workflows/ci.yml`](../../.github/workflows/ci.yml)
runs `docker compose --profile e2e up -d --build`, executes this suite against
the container, prints the container logs, and tears the stack down — on every PR
to `master` that touches `auth-service/` (or `docker-compose.yml`).
