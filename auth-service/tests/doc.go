// Package tests is the shared black-box e2e harness for auth-service.
//
// The e2e suites live under tests/modules/<name>/ (one per service module) and
// import this package for the HTTP client, readiness gate, and helpers. The
// suites are black-box: the service runs as a real container backed by a live
// Postgres (docker-compose.yml, profile "e2e"), driven over the network:
//
//	docker compose --profile e2e up -d --build   # from the repo root
//	go test -tags=e2e ./tests/...                # BASE_URL defaults to :8080
//
// The harness itself (harness.go) is guarded by the "e2e" build tag. This file
// carries no build constraint so the package always has one buildable Go file,
// keeping `go build ./...` and `go test ./...` happy when the tag is absent.
package tests
