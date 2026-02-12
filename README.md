# stoic

A Go web app template.

[Skip to the Quick Start](#quick-start)

## Why Stoic

Modern web development is buried under layers of tooling, transpilers, bundlers, ORMs, and frameworks that do too much. Stoic takes a different approach, and minimizes what's  between you and your code.

Stoic boot straps your project with OIDC auth, a Postgres data store, server-rendered HTML templates, and Server-Sent-Events (SSE) to pushed data to clients. Wired together in ~1200 lines of code you own. No framework. No magic. Clone it, rename it, and build on it.

### Why Go

- **Compile-time safety.** A wrong type, a missing field, an unused import — caught before the code runs.
- **One goroutine per request.** Sequential code that handles concurrency.
- **Single binary deployment.** `go build` produces one file with zero dependencies.
- **Predictable performance.** Sub-millisecond GC pauses. Tens of thousands of concurrent connections on modest hardware.
- **Stdlib does the work.** `net/http` is a production server, not a dev placeholder. `crypto`, `encoding/json`, `html/template` — battle-tested, no third-party replacements needed.
- **Fast cold starts, small footprint.** Starts in milliseconds, idles at 10-30MB. Your containers stay light and your cloud bill stays low.

### Why a Template, Not a Library

- ~1200 lines of glue code over well-known Go libraries (gorilla/mux, pgx, go-oidc, SQLC). The value is in the wiring decisions, not abstraction.
- "Opinionated" and "library" are in tension — adding interfaces for pluggability undoes the opinions.
- Go developers prefer to own their code. You can read and modify 249 lines of auth code. That's a feature.
- No semver, no backwards compatibility, no API docs to maintain. Just code you control.

## What's Included

- OIDC authentication (Keycloak, Auth0, Okta, or any provider)
- Postgres with auto-migrations and type-safe queries (SQLC)
- Server-rendered HTML templates with hot-reload in dev
- Server-Sent Events for real-time updates
- Session management with automatic token refresh
- Centralized config loaded once at startup

## Quick Start

1. Clone this template:
   ```
   gh repo create my-project --template antonkarounis/stoic
   ```

2. Rename the module:
   ```
   make rename
   ```

3. Start dev services (Postgres + pgAdmin + Keucloak):
   ```
   make dev
   ```

4. Copy over the config (has a working example configs):
   ```
   cp .env.example .env
   ```

5. Run:
   ```
   make run
   ```

6. Open http://localhost:8080

7. Log in with `dev@test.com` and `password`

## Project Structure

```
internal/platform/   — Framework plumbing (auth, db, templates). Rarely modified.
internal/app/        — Your application code. Start here.
  routes.go          — Register your routes
  handlers/          — Your request handlers
  templates/         — Your HTML templates
  migrations/        — Your database migrations
```

## Adding a Page

1. Create a handler in `internal/app/handlers/`
2. Create a template in `internal/app/templates/www/`
3. Register the route in `internal/app/routes.go`
4. Done.

## Adding a Database Table

1. Write a migration in `internal/app/migrations/`
2. Write queries in `internal/platform/db/queries/`
3. Run `make sqlc` to regenerate
4. Use the generated functions in your handlers

