// Package health holds the black-box e2e suite for the health module. The tests
// live in e2e-tagged files and import the shared harness (auth-service/tests).
// This file carries no build constraint so the package stays buildable — and
// `go build ./...` / `go test ./...` stay happy — when the "e2e" tag is absent.
package health
