# Eagle Bank API

REST API for Eagle Bank, implemented against [`openapi.yaml`](openapi.yaml) (accounts, transactions, users).

## Prerequisites

- [Go](https://go.dev/dl/)
- [Docker](https://docs.docker.com/get-docker/) with Compose v2

## Setup

From the repo root:

```bash
docker compose up -d
go test ./...
```

This starts local Postgres and runs unit tests.

## Local database (PostgreSQL)

The repo ships a small Compose stack: Postgres with a bind-mounted `db/initdb` folder (scripts run **once** when the data volume is first created).

Start DB:

```bash
docker compose up -d
```

Connection settings:

| Setting | Value |
|--------|--------|
| Host | `localhost` |
| Port | `5432` |
| Database | `eagle_bank` |
| User | `eagle` |
| Password | `eagle` |

DSN:

```text
postgres://eagle:eagle@localhost:5432/eagle_bank?sslmode=disable
```

Reset DB (drops volume and reruns init SQL on next startup):

```bash
docker compose down -v && docker compose up -d
```

If Compose fails with **port 5432 already allocated**, another Postgres (or this stack from another clone) is using that port. Either stop it or change the host mapping in `docker-compose.yml` (for example `5433:5432`) and use `localhost:5433` in your DSN.

## Schema

| File | Purpose |
|------|---------|
| [`db/initdb/001_users.sql`](db/initdb/001_users.sql) | `users` table |

The `users` table matches the OpenAPI **UserResponse** / **CreateUserRequest** shape:

- **`id`** — `TEXT` primary key; must match `^usr-[A-Za-z0-9]+$` (same as `UserResponse.id`). Your service should generate values before insert.
- **`name`**, **`phone_number`** (E.164), **`email`** — as in the spec; `email` is unique at the DB level.
- **Address** — nested `address` object is stored as **`address_line1`**, **`address_line2`**, **`address_line3`**, **`town`**, **`county`**, **`postcode`**.
- **`created_at`**, **`updated_at`** — `timestamptz`, default `now()` on insert; app code should refresh `updated_at` on updates.

Validation from OpenAPI (for example `format: email`) should still be enforced in application code.

## Tests

### Unit tests

Fast tests (in-memory store, no Postgres):

```bash
go test ./...
```

### Integration tests (real Postgres)

Integration tests live under [`internal/users/postgres`](internal/users/postgres), use the `integration` build tag, and read `TEST_DATABASE_URL`.

Run:

```bash
docker compose up -d
export TEST_DATABASE_URL='postgres://eagle:eagle@localhost:5432/eagle_bank?sslmode=disable'
go test -tags=integration ./internal/users/postgres/...
```

If you publish Postgres on another host port, update `TEST_DATABASE_URL` accordingly (for example `localhost:5433`).

If `TEST_DATABASE_URL` is unset, integration tests skip.

## Project layout

- `openapi.yaml` — contract for the take-home task
- `docker-compose.yml` — local Postgres
- `db/initdb/` — SQL executed on first-time volume initialization
- `main.go` — application entrypoint (placeholder until the API is implemented)
- `internal/users` — `Repository` interface and domain types
- `internal/users/postgres` — Postgres implementation (`database/sql` + `pgx` driver)
- `internal/users/memory` — in-memory `Repository` for fast unit tests
