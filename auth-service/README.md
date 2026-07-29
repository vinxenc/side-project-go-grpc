# auth-service

A small HTTP service built with Go's standard `net/http`. It uses a lightweight
module system so features can be added as self-contained packages that plug into
a shared server.

## Requirements

- Go 1.26+

## Run

```bash
cd auth-service
go run ./src
```

The server listens on `:8080`.

## Build

```bash
cd auth-service
go build ./...
```

## Endpoints

| Method | Path      | Description                          |
| ------ | --------- | ------------------------------------ |
| GET    | `/health` | Liveness check. Returns service status and current UTC time. Non-GET methods return `405 Method Not Allowed`. |

### `GET /health`

```bash
curl -s http://localhost:8080/health
```

```json
{ "status": "ok", "time": "2026-07-29T03:00:03.231394Z" }
```

## Project structure

```text
auth-service/
├── go.mod
├── core/                         # shared, reusable building blocks
│   ├── module.go                 # Module interface + RegisterModules(mux, ...)
│   └── controller.go             # BaseController with JSON / Error helpers
└── src/
    ├── main.go                   # boots the server, registers modules
    └── modules/
        └── health/               # health-check module
            ├── health.dto.go         # request/response types
            ├── health.controller.go  # handlers (embeds core.BaseController)
            └── health.module.go      # Module + New() + RegisterRoutes
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

- **`core.RegisterModules(mux, ...)`** — wires a list of modules onto the server.
  `main.go` registers modules in one place:

  ```go
  core.RegisterModules(mux,
      health.New(),
      // auth.New(), ...
  )
  ```

- **`core.BaseController`** — shared HTTP helpers (`JSON`, `Error`) that module
  controllers embed to avoid boilerplate:

  ```go
  type controller struct {
      core.BaseController
  }
  ```

### Adding a module

1. Create `src/modules/<name>/` with three files:
   `<name>.dto.go`, `<name>.controller.go`, `<name>.module.go`.
2. Give the module a `Module` type with `New()` and a `RegisterRoutes(mux)` method
   (this satisfies `core.Module`).
3. Embed `core.BaseController` in the module's `controller` for the `JSON`/`Error`
   helpers.
4. Add `<name>.New()` to the `core.RegisterModules(...)` call in `src/main.go`.
