# auth-service

A minimal HTTP service providing a health endpoint. Built with the Go standard library only.

## Run

```sh
# Default port 8080
go run ./cmd/server

# Custom port
PORT=9090 go run ./cmd/server
```

## Build

```sh
go build -o auth-server ./cmd/server
./auth-server
```

## Test

```sh
go test ./... -race
```

## Endpoints

| Method | Path      | Description              |
|--------|-----------|--------------------------|
| GET    | /healthz  | Returns `{"status":"ok"}` with HTTP 200 |

## Configuration

| Variable | Default | Description                  |
|----------|---------|------------------------------|
| `PORT`   | `8080`  | TCP port the server listens on |
