# Limen API Endpoints (reference)

Reference notes on the HTTP API exposed by [thecodearcher/limen](https://github.com/thecodearcher/limen),
a modular, plugin-first authentication library for Go. Kept here as a design
reference for `auth-service`.

Routes below were read directly from limen's source (`route_builder.go`,
`limen_handlers.go`, and each `plugins/*/`), not just the docs.

## What limen is

- A **plugin-first auth library for Go** (Go 1.25+), inspired by better-auth.
  Philosophy: *"bring your own database, bring your own framework."*
- **Core** ships interfaces, session management, cookies, CSRF / trusted-origin
  checks, and security primitives, and exposes an `http.Handler` you mount into
  any `net/http`-compatible router: `mux.Handle("/auth/", auth.Handler())`.
- **Each auth method is a separate importable module** under `plugins/`; plugins
  register their own routes via a `RouteBuilder`.
- **Adapters** for persistence (`adapters/gorm`, `adapters/sql`) and a
  **TypeScript client SDK** (`clients/typescript`) with React/Vue/Svelte/Solid
  bindings.
- Plugins: credential-password, magic-link, two-factor, session-jwt, and OAuth
  (generic + Google, GitHub, Apple, Discord, Twitch, Facebook, Twitter,
  LinkedIn, Spotify, Microsoft, ConsentKeys).

## Conventions

- Global base path defaults to **`/auth`** (`http_config.go`). Every path below is
  shown with that prefix; plugin base paths nest under it. All base paths are
  configurable (`WithHTTPBasePath` globally; per-plugin overrides), so `/auth/...`
  is just the default.
- **Protected** routes require an authenticated session. Auth is carried by the
  session cookie **`limen_session`** (set automatically), or via
  **`Authorization: Bearer <token>`** when Bearer support is enabled
  (`WithBearerEnabled`). The `session-jwt` plugin always uses
  `Authorization: Bearer <access_token>`.
- Any endpoint with a JSON body needs **`Content-Type: application/json`**.
  State-changing requests are also subject to Origin / trusted-origin (CSRF) checks.
- `?` marks optional body fields.
- Individual routes can be disabled by route ID or path
  (`RouteBuilder.isRouteDisabled` / `disabledPaths`).

## Core (always registered)

| Method | Path | Auth | Body |
|---|---|---|---|
| GET  | `/auth/me` | session | — |
| GET  | `/auth/sessions` | session | — |
| POST | `/auth/signout` | session | — |
| POST | `/auth/revoke-sessions` | session | — |
| POST | `/auth/verify-email` | public *(if email verification enabled)* | `{ token }` |
| POST | `/auth/email-verifications` | session *(if email verification enabled)* | — |

## credential-password (mounts at `/auth`)

| Method | Path | Auth | Body |
|---|---|---|---|
| POST | `/auth/signup/credential` | public | `{ email, password, username?, ...additionalFields }` |
| POST | `/auth/signin/credential` | public | `{ credential, password, remember_me? }` |
| POST | `/auth/passwords/request-reset` | public | `{ email }` |
| POST | `/auth/passwords/reset` | public | `{ token, new_password }` |
| POST | `/auth/passwords/change` | session | `{ current_password, new_password, revoke_other_sessions? }` |
| PUT  | `/auth/passwords` | session | `{ new_password, revoke_other_sessions? }` |
| POST | `/auth/usernames/check` | public | `{ username }` |

## OAuth (base `/oauth` → `/auth/oauth`)

`:provider` is the provider slug, e.g. `google`.

| Method | Path | Auth | Params / Body |
|---|---|---|---|
| GET    | `/auth/oauth/:provider/authorize` | public | query: `redirect_uri?`, `error_redirect_uri?` |
| GET    | `/auth/oauth/:provider/callback` | public | query: `code`, `state`, `error?` (from provider) |
| POST   | `/auth/oauth/:provider/callback` | public | form-post callback (`response_mode=form_post` providers) |
| GET    | `/auth/oauth/:provider/link` | session | query: `redirect_uri?`, `error_redirect_uri?` |
| GET    | `/auth/oauth/accounts` | session | — |
| DELETE | `/auth/oauth/:provider/unlink` | session | — |
| GET    | `/auth/oauth/:provider/tokens` | session | — |
| POST   | `/auth/oauth/:provider/tokens/refresh` | session | — |

OAuth flow uses a state cookie `limen_oauth`.

## magic-link (base `/magic-link` → `/auth/magic-link`)

| Method | Path | Auth | Params / Body |
|---|---|---|---|
| POST | `/auth/magic-link/signin` | public | `{ email, meta?, redirect_uri?, new_user_redirect_uri?, error_redirect_uri? }` |
| GET  | `/auth/magic-link/verify` | public | query: `token` (required), `redirect_uri?`, `new_user_redirect_uri?`, `error_redirect_uri?` |

## two-factor (base `/two-factor` → `/auth/two-factor`)

| Method | Path | Auth | Body |
|---|---|---|---|
| POST | `/auth/two-factor/initiate-setup` | session | `{ password }` |
| POST | `/auth/two-factor/finalize-setup` | session | `{ code }` |
| POST | `/auth/two-factor/disable` | session | `{ password }` |
| POST | `/auth/two-factor/verify` | public *(2FA challenge cookie `session_2fa`)* | `{ code, method? }` — `method` is `totp` (default) or `otp` |
| GET  | `/auth/two-factor/totp/uri` | session | — |
| POST | `/auth/two-factor/otp/send` | public | `{ email }` |
| GET  | `/auth/two-factor/backup-codes` | session | — |
| PUT  | `/auth/two-factor/backup-codes` | session | — (regenerates) |

## session-jwt (base `/` → `/auth`)

| Method | Path | Auth | Body |
|---|---|---|---|
| POST | `/auth/refresh` | public | `{ refresh_token }` |

With `session-jwt`, protected routes expect `Authorization: Bearer <access_token>`.
