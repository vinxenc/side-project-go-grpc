#!/usr/bin/env bash
# Unit-test coverage for the auth-service *business logic* under ./src.
#
# Scope (what "coverage for src" means here):
#   - Instruments every package under ./src EXCEPT the `main` package
#     (auth-service/src) — main() is the process entrypoint/wiring.
#   - Additionally drops the Postgres connection bootstrap (auth.postgres.go)
#     from the profile: it opens a real database, so its connection-setup/ping
#     paths are covered by the e2e suite (tests/), not unit tests.
#
# Fails if the resulting statement coverage is below THRESHOLD.
set -euo pipefail

THRESHOLD="${COVERAGE_THRESHOLD:-90}"
PROFILE="${COVERAGE_PROFILE:-coverage.out}"

cd "$(dirname "$0")/.."

# Packages to instrument: all of ./src/... except the main package.
pkgs="$(go list ./src/... | grep -v '/src$' | paste -sd, -)"

# Run unit tests, measuring coverage across the selected packages.
go test -race -covermode=atomic -coverpkg="$pkgs" -coverprofile="$PROFILE.raw" ./src/...

# Exclude the Postgres bootstrap (e2e-covered) from the reported profile.
head -1 "$PROFILE.raw" >"$PROFILE"
grep -v 'auth-service/src/modules/auth/auth.postgres.go' "$PROFILE.raw" | tail -n +2 >>"$PROFILE"
rm -f "$PROFILE.raw"

# Report per-function and the total.
go tool cover -func="$PROFILE"

total="$(go tool cover -func="$PROFILE" | awk '/^total:/ {sub(/%/,"",$3); print $3}')"
echo "----------------------------------------"
echo "src coverage: ${total}% (threshold ${THRESHOLD}%)"

# Compare as integers-with-decimals using awk (portable, no bc dependency).
if awk -v t="$total" -v th="$THRESHOLD" 'BEGIN { exit !(t+0 < th+0) }'; then
  echo "FAIL: coverage ${total}% is below the ${THRESHOLD}% threshold" >&2
  exit 1
fi
echo "OK: coverage meets the threshold"
