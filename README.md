# todo-backend

Go HTTP API for a multi-user Todo list. PostgreSQL storage, JWT auth, optional Google OAuth.

## Stack
- Go 1.22, [chi](https://github.com/go-chi/chi) router
- PostgreSQL via [pgx/v5](https://github.com/jackc/pgx) (pure Go, no CGO)
- JWT (HS256) via `github.com/golang-jwt/jwt/v5`
- `bcrypt` from `golang.org/x/crypto`
- Google OAuth 2.0 via `golang.org/x/oauth2` (optional)

## Quick start

### Docker Compose (preferred)
```bash
# From the repo root — starts the API + a Postgres container
docker compose up
```

### Local dev (no Docker)
```bash
make tidy   # fetch / tidy deps

# Start a local Postgres, then:
DATABASE_URL=postgres://todo:todo@localhost:5432/todo?sslmode=disable \
JWT_SECRET=dev-secret \
make run    # starts on :8080, auto-applies migrations on startup
```

Copy `.env.example` to `.env` and adjust values; the server reads env vars directly (no auto-loading of `.env` — use `export` or a tool like `direnv`).

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `ADDR` | `:8080` | TCP address the server listens on |
| `DATABASE_URL` | — | Postgres DSN, e.g. `postgres://todo:todo@localhost:5432/todo?sslmode=disable` |
| `MIGRATIONS_DIR` | `migrations` | Path to directory containing `*.sql` migration files |
| `JWT_SECRET` | dev placeholder | HMAC-SHA256 signing secret — **set a strong random value in non-dev envs** |
| `CORS_ORIGIN` | `http://localhost:5173` | Allowed CORS origin(s), comma-separated |
| `FRONTEND_URL` | `http://localhost:5173` | Redirect target after Google OAuth callback |
| `GOOGLE_CLIENT_ID` | `` | Google OAuth client ID — leave empty to disable Google sign-in |
| `GOOGLE_CLIENT_SECRET` | `` | Google OAuth client secret |
| `GOOGLE_REDIRECT_URL` | `http://localhost:8080/api/auth/google/callback` | Registered OAuth redirect URI |

## API

All endpoints exchange JSON. Authenticated routes require `Authorization: Bearer <token>`.

User objects may include optional fields `email`, `display_name`, and `avatar_url` when the account was created or linked via Google OAuth.

| Method | Path | Auth | Body | Returns |
|--------|------|------|------|---------|
| POST | `/api/auth/register` | no | `{username, password}` | `201 {token, expires_at, user}` |
| POST | `/api/auth/login` | no | `{username, password}` | `200 {token, expires_at, user}` |
| POST | `/api/auth/logout` | yes | — | `200 {status: "logged_out"}` |
| GET | `/api/auth/me` | yes | — | `200 {user}` |
| GET | `/api/auth/google/start` | no | — | `302` redirect to Google consent |
| GET | `/api/auth/google/callback` | no | — | `302` redirect to `FRONTEND_URL` with token |
| GET | `/api/todos` | yes | — | `200 {todos: [...]}` |
| POST | `/api/todos` | yes | `{title}` | `201 {todo}` |
| PUT | `/api/todos/{id}` | yes | `{title?, completed?}` (>=1 field) | `200 {todo}` |
| DELETE | `/api/todos/{id}` | yes | — | `204` |

The two `/api/auth/google/*` routes are only registered when `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET` are non-empty.

Validation:
- `username`: 3-32 chars, `[A-Za-z0-9_.-]`
- `password`: 6-128 chars
- `title`: 1-200 chars (trimmed)

JWT lifetime: 24h. Logout is stateless — the client discards the token; the server endpoint acknowledges the request.

## Smoke test (curl)

Start the server (see Quick start above), then:

```bash
# Register a new account
curl -s -X POST localhost:8080/api/auth/register \
  -H 'content-type: application/json' \
  -d '{"username":"alice","password":"hunter2"}'

# Login and capture the token
TOKEN=$(curl -s -X POST localhost:8080/api/auth/login \
  -H 'content-type: application/json' \
  -d '{"username":"alice","password":"hunter2"}' | jq -r .token)

# Inspect the current user (response includes email/display_name/avatar_url if set via OAuth)
curl -s localhost:8080/api/auth/me \
  -H "authorization: Bearer $TOKEN"

# Create a todo
curl -s -X POST localhost:8080/api/todos \
  -H "authorization: Bearer $TOKEN" \
  -H 'content-type: application/json' \
  -d '{"title":"buy milk"}'

# List todos
curl -s localhost:8080/api/todos -H "authorization: Bearer $TOKEN"
```

## Layout

```
cmd/api/             entrypoint, route wiring
internal/config/     env loader
internal/db/         postgres open + migrate
internal/auth/       JWT signer, bcrypt helpers
internal/model/      User, Todo structs
internal/repository/ SQL queries (UserRepo, TodoRepo)
internal/service/    business rules + validation (AuthService, TodoService)
internal/handler/    HTTP handlers (AuthHandler, TodoHandler, OAuthHandler)
internal/middleware/ RequireAuth
internal/httpx/      JSON helpers
internal/oauth/      Google OAuth 2.0 client
migrations/          schema SQL (applied in order at startup)
```

Requests flow: **handler -> service -> repository -> Postgres**. Handlers own HTTP concerns (parsing, status codes, JSON encoding). Services own business rules and validation. Repositories own SQL and return domain models.
