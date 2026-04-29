# Eagle Bank API

REST API for Eagle Bank, implemented against [`openapi.yaml`](openapi.yaml) (accounts, transactions, users).

## Prerequisites

- [Go](https://go.dev/dl/)
- [Docker](https://docs.docker.com/get-docker/) with Compose v2

## Setup

From the repo root:

```bash
docker compose up -d
./scripts/test-summary.sh
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
| [`db/initdb/001_users.sql`](db/initdb/001_users.sql) | `users` and `user_credentials` tables |

The `users` table matches the OpenAPI **UserResponse** / **CreateUserRequest** shape:

- **`id`** — `TEXT` primary key; must match `^usr-[A-Za-z0-9]+$` (same as `UserResponse.id`). Your service should generate values before insert.
- **`name`**, **`phone_number`** (E.164), **`email`** — as in the spec; `email` is unique at the DB level.
- **Address** — nested `address` object is stored as **`address_line1`**, **`address_line2`**, **`address_line3`**, **`town`**, **`county`**, **`postcode`**.
- **`created_at`**, **`updated_at`** — `timestamptz`, default `now()` on insert; app code should refresh `updated_at` on updates.

Authentication data is stored separately in `user_credentials`:

- **`user_id`** — references `users.id` and cascades on delete.
- **`password_hash`** — bcrypt hash only; raw passwords are never stored or returned.

Validation from OpenAPI (for example `format: email`) should still be enforced in application code.

## Authentication

`POST /v1/users` is unauthenticated so new users can sign up. The request includes a `password`; the response is still the normal `UserResponse` and does not include password material.

`POST /v1/auth/token` authenticates with email and password:

```json
{
  "email": "alice@example.com",
  "password": "correct horse battery staple"
}
```

On success it returns:

```json
{
  "token": "<jwt>"
}
```

The JWT is signed with HS256 by `internal/auth`. The token subject (`sub`) is the user id and should be passed to protected endpoints as `Authorization: Bearer <jwt>`.

## Manual auth flow testing

Start Postgres first:

```bash
docker compose up -d
```

Then start the API on `localhost:8080`:

```bash
export DATABASE_URL='postgres://eagle:eagle@localhost:5432/eagle_bank?sslmode=disable'
export JWT_SECRET='local-dev-secret'
go run .
```

`DATABASE_URL` defaults to the local Compose DSN above if unset. `JWT_SECRET` also has a development default, but setting it explicitly makes the auth flow clearer.

### Sign up

`POST /v1/users` does not require a bearer token:

```bash
curl -i -X POST http://localhost:8080/v1/users \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Alice Example",
    "address": {
      "line1": "1 High St",
      "town": "London",
      "county": "Greater London",
      "postcode": "SW1A 1AA"
    },
    "phoneNumber": "+441234567890",
    "email": "alice@example.com",
    "password": "correct horse battery staple"
  }'
```

Expected response: `201 Created` with a `UserResponse`. The password is not returned.

### Authenticate

```bash
curl -i -X POST http://localhost:8080/v1/auth/token \
  -H 'Content-Type: application/json' \
  -d '{
    "email": "alice@example.com",
    "password": "correct horse battery staple"
  }'
```

Expected response: `200 OK` with:

```json
{
  "token": "<jwt>"
}
```

With `jq`, capture the token:

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/v1/auth/token \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"correct horse battery staple"}' \
  | jq -r '.token')
```

### Use the bearer token

Pass the token to protected endpoints:

```bash
curl -i http://localhost:8080/v1/users/usr-example \
  -H "Authorization: Bearer $TOKEN"
```

### Failure cases

Wrong password:

```bash
curl -i -X POST http://localhost:8080/v1/auth/token \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"wrong password"}'
```

Expected response: `401 Unauthorized`.

Missing signup password:

```bash
curl -i -X POST http://localhost:8080/v1/users \
  -H 'Content-Type: application/json' \
  -d '{"name":"Alice Example","email":"alice@example.com"}'
```

Expected response: `400 Bad Request`.

## Tests

A summary script gives per-package pass/fail counts:

```bash
./scripts/test-summary.sh
```

### Unit tests

Fast tests (in-memory store, no Postgres):

```bash
./scripts/test-summary.sh ./...
```

### Integration tests (real Postgres)

Integration tests use the `integration` build tag and read `TEST_DATABASE_URL`.

Run:

```bash
docker compose up -d
export TEST_DATABASE_URL='postgres://eagle:eagle@localhost:5432/eagle_bank?sslmode=disable'
./scripts/test-summary.sh -tags=integration ./internal/users/postgres/... ./internal/httpapi/...
```

If you publish Postgres on another host port, update `TEST_DATABASE_URL` accordingly (for example `localhost:5433`).

If `TEST_DATABASE_URL` is unset, integration tests skip.

## Project layout

- `openapi.yaml` — contract for the take-home task
- `docker-compose.yml` — local Postgres
- `db/initdb/` — SQL executed on first-time volume initialization
- `main.go` — application entrypoint (placeholder until the API is implemented)
- `internal/auth` — JWT issuing and verification
- `internal/httpapi` — HTTP handlers and API-level tests
- `internal/users` — `Repository` interface and domain types
- `internal/users/postgres` — Postgres implementation (`database/sql` + `pgx` driver)
- `internal/users/memory` — in-memory `Repository` for fast unit tests
