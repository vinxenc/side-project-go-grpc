// Package tests holds the end-to-end (e2e) test suite for auth-service.
//
// The e2e tests are black-box: the auth-service runs as a real container backed
// by a live Postgres (docker-compose.yml, profile "e2e"), and the suite drives
// its HTTP API over the network with a real client + cookie jar, exercising
// multi-step user journeys exactly as a client would. They are therefore
// guarded by the "e2e" build tag and require the stack to be up:
//
//	docker compose --profile e2e up -d --build   # from the repo root
//	go test -tags=e2e ./tests/...                # BASE_URL defaults to :8080
//
// Without the tag the suite is excluded, so the DB-free unit run
// (`go test ./...`) is unaffected. This file carries no build constraint so the
// package always contains at least one buildable Go file, keeping
// `go build ./...` and `go test ./...` happy when the tag is absent.
package tests
