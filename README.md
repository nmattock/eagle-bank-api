# Eagle Bank API

REST API for Eagle Bank, implemented against [`openapi.yaml`](openapi.yaml) (accounts, transactions, users).

## Prerequisites

- [Go](https://go.dev/dl/)
- [Docker](https://docs.docker.com/get-docker/) with Compose v2

## Local database (PostgreSQL)

The repo ships a small Compose stack: Postgres with a bind-mounted `db/initdb` folder (scripts run **once** when the data volume is first created—same pattern as mounting SQL into the official DB image).

```bash
docker compose up -d
```

Wait until the container is healthy (Compose runs `pg_isready` on an interval). Then connect with:

| Setting | Value |
|--------|--------|
| Host | `localhost` |
| Port | `5432` |
| Database | `eagle_bank` |
| User | `eagle` |
| Password | `eagle` |

Example DSN for Go:

```text
postgres://eagle:eagle@localhost:5432/eagle_bank?sslmode=disable
```

**Reset the database** (drops the volume so init scripts run again from a clean state):

```bash
docker compose down -v && docker compose up -d
```

If Compose fails with **port 5432 already allocated**, another Postgres (or this stack from another clone) is using that port. Either stop it or change the host mapping in `docker-compose.yml` (for example `5433:5432`) and use `localhost:5433` in your DSN.

## Schema

| File | Purpose |
|------|---------|
| [`db/initdb/001_users.sql`](db/initdb/001_users.sql) | `users` table |

The `users` table matches the OpenAPI **UserResponse** / **CreateUserRequest** shape:

- **`id`** — `TEXT` primary key; must match `^usr-[A-Za-z0-9]+$` (same as `UserResponse.id`). Your service should generate values (for example random alphanumeric suffixes) before insert.
- **`name`**, **`phone_number`** (E.164), **`email`** — as in the spec; `email` is unique at the DB level.
- **Address** — nested `address` object in JSON is stored as **`address_line1`**, **`address_line2`**, **`address_line3`**, **`town`**, **`county`**, **`postcode`** (required vs optional lines follow the spec: line2/line3 nullable).
- **`created_at`**, **`updated_at`** — `timestamptz`, default `now()` on insert; the API should refresh **`updated_at`** on successful updates to match `updatedTimestamp`.

Validation in the OpenAPI (for example `format: email`) should still be enforced in application code; the SQL file adds minimal checks (`id` pattern, phone pattern) so bad rows are harder to insert by mistake.

## Tests

### Unit tests

Fast tests (in-memory store, no Postgres):

```bash
go test ./...
```

That runs packages such as [`internal/users/memory`](internal/users/memory); Postgres integration sources are **not** compiled unless you pass the `integration` build tag (see below).

### Integration tests (real Postgres)

Integration tests live under [`internal/users/postgres`](internal/users/postgres) and use the **`integration` build tag**. They connect with **`TEST_DATABASE_URL`** and exercise the real `users` table.

1. Start Postgres with the schema applied (for example local Compose from [Local database](#local-database-postgresql)).
2. Run:

```bash
export TEST_DATABASE_URL='postgres://eagle:eagle@localhost:5432/eagle_bank?sslmode=disable'
go test -tags=integration ./internal/users/postgres/...
```

If you publish Postgres on another host port, change the URL accordingly (for example `localhost:5433`).

If `TEST_DATABASE_URL` is unset, integration tests **skip** so CI or laptops without Docker still succeed.

## Project layout

- `openapi.yaml` — contract for the take-home task
- `docker-compose.yml` — local Postgres
- `db/initdb/` — SQL executed on first-time volume initialization
- `main.go` — application entrypoint (placeholder until the API is implemented)
- `internal/users` — `Repository` interface and domain types
- `internal/users/postgres` — Postgres implementation (`database/sql` + `pgx` driver: import `_ "github.com/jackc/pgx/v5/stdlib"` and `sql.Open("pgx", dsn)`)
- `internal/users/memory` — in-memory `Repository` for fast unit tests (`go test ./internal/users/memory/...`)
