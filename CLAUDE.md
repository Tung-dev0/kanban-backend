# kanban-backend

Go HTTP API for the Kanban app — chi router, Postgres (pgx/v5), JWT, optional Google OAuth.

> This repo is a **submodule** of [kanban-control-plane](https://github.com/Tung-dev0/kanban-control-plane) (control plane). Org-level CLAUDE.md, agents, slash commands, and MCP config live there.

## Quick start
```bash
make tidy
DATABASE_URL=postgres://todo:todo@localhost:5432/todo?sslmode=disable \
JWT_SECRET=dev make run            # listens on :8080
```

Or via the parent compose (preferred): see `kanban-control-plane`.

## API
See `doc/spec.md` (in the control plane) or `README.md` here for the endpoint table.

## Layout
```
cmd/api/             entrypoint, route wiring
internal/config      env loader
internal/db          postgres + versioned migrations
internal/auth        JWT signer, bcrypt
internal/model       User, Column, Card, ColumnWithCards, Board
internal/repository  user.go, column.go, card.go
internal/service     auth.go, board.go (bootstrap), column.go, card.go
internal/handler     auth.go (+ Me), oauth.go, board.go, column.go, card.go
internal/middleware  RequireAuth
internal/httpx       JSON helpers
internal/oauth       Google OAuth client
migrations/          0001_init, 0002_oauth_columns, 0003_kanban
```

Conventions: handler → service → repository, downward only. All SQL uses `$N`. pgx `23505` → domain sentinels.
