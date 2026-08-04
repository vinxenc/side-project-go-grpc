# side-project-go-grpc

A monorepo of Go microservices exploring HTTP and gRPC service design.

## Services

| Service                         | Description                                    | Stack             |
| ------------------------------- | ---------------------------------------------- | ----------------- |
| [auth-service](./auth-service)  | HTTP service with a modular architecture; currently exposes a health endpoint. | Go, `net/http`    |

## Getting started

Each service is a self-contained Go module. See the service's own README for
run and build instructions. For example:

```bash
cd auth-service
go run ./src
```

## Layout

```text
side-project-go-grpc/
├── auth-service/     # HTTP auth service (see auth-service/README.md)
└── docs/             # Design references and notes
```

## Docs

- [Limen API endpoints](./docs/limen-endpoints.md) — reference notes on the
  [thecodearcher/limen](https://github.com/thecodearcher/limen) auth library's
  HTTP API, kept as a design reference for `auth-service`.
