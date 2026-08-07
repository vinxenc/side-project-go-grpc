// Package tests holds the end-to-end (e2e) test suite for auth-service.
//
// The e2e tests exercise the real production wiring — auth.New opening a live
// Postgres connection, the full Huma/net-http stack, and a real HTTP client
// with a cookie jar driving multi-step user journeys over a socket. They are
// therefore guarded by the "e2e" build tag and require a running Postgres:
//
//	go test -tags=e2e ./tests/...
//
// Without the tag the suite is excluded, so the DB-free unit run
// (`go test ./...`) is unaffected. This file carries no build constraint so the
// package always contains at least one buildable Go file, keeping
// `go build ./...` and `go test ./...` happy when the tag is absent.
package tests
