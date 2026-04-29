# Eagle Bank API

REST API for Eagle Bank, implemented against [`openapi.yaml`](openapi.yaml) — covering users, authentication, bank accounts and transactions.

## Prerequisites

- [Go](https://go.dev/dl/) 1.25 or newer
- [Docker](https://docs.docker.com/get-docker/) with Compose v2

## Setup

### 1) Start the database

```bash
docker compose up -d
```

This starts only Postgres (`db` service).

### 2) Start the backend API

```bash
go run .
```

### 3) Manually test with curl

Create a user (unauthenticated endpoint):

```bash
curl -i -X POST http://localhost:8080/v1/users \
  -H 'Content-Type: application/json' \
  -d '{
    "name":"Alice Example",
    "address":{"line1":"1 High St","town":"London","county":"Greater London","postcode":"SW1A 1AA"},
    "phoneNumber":"+441234567890",
    "email":"alice@example.com",
    "password":"correct horse battery staple"
  }'
```

Expected: `201 Created`.

### Runtime configuration

The server listens on `localhost:8080` by default. Configuration is via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `HTTP_ADDR` | `:8080` | Listen address |
| `DATABASE_URL` | `postgres://eagle:eagle@localhost:5432/eagle_bank?sslmode=disable` | Postgres DSN |
| `JWT_SECRET` | `dev-secret-change-me` | HMAC key for JWT signing |

## Tests

### Unit tests

Fast, in-memory tests that need no external dependencies:

```bash
./scripts/test-summary.sh ./...
```

### Integration tests

Integration tests run against a real Postgres instance, gated by the `integration` build tag:

```bash
docker compose up -d
export TEST_DATABASE_URL='postgres://eagle:eagle@localhost:5432/eagle_bank?sslmode=disable'
./scripts/test-summary.sh -tags=integration ./internal/users/postgres/... ./internal/httpapi/...
```

If `TEST_DATABASE_URL` is unset, integration tests skip automatically.

## Project layout

| Path | Purpose |
|------|---------|
| `openapi.yaml` | OpenAPI contract |
| `main.go` | Application entrypoint and HTTP routing |
| `docker-compose.yml` | Local Postgres |
| `db/initdb/` | SQL migrations executed on first-time volume init |
| `scripts/` | Developer tooling (test summary) |
| `internal/auth` | JWT issuing and verification (HS256) |
| `internal/httpapi` | HTTP handlers, middleware and API-level tests |
| `internal/accounts` | `Repository` interface and domain types for bank accounts and transactions |
| `internal/accounts/postgres` | Postgres implementation |
| `internal/accounts/memory` | In-memory implementation for unit tests |
| `internal/users` | `Repository` interface and domain types for users |
| `internal/users/postgres` | Postgres implementation |
| `internal/users/memory` | In-memory implementation for unit tests |

## Database

### Connection

| Setting | Value |
|---------|-------|
| Host | `localhost` |
| Port | `5432` |
| Database | `eagle_bank` |
| User | `eagle` |
| Password | `eagle` |

DSN: `postgres://eagle:eagle@localhost:5432/eagle_bank?sslmode=disable`

### Schema

| File | Tables |
|------|--------|
| [`db/initdb/001_users.sql`](db/initdb/001_users.sql) | `users`, `user_credentials` |
| [`db/initdb/002_bank_accounts.sql`](db/initdb/002_bank_accounts.sql) | `bank_accounts` |
| [`db/initdb/003_transactions.sql`](db/initdb/003_transactions.sql) | `transactions` |

Monetary values (`balance`, `amount`) are stored as `BIGINT` in minor units (pence) to avoid floating-point rounding. The HTTP layer converts to/from decimal pounds at the API boundary.

### Reset

Drop the volume and re-run init SQL:

```bash
docker compose down -v && docker compose up -d
```

If port 5432 is already in use, change the host mapping in `docker-compose.yml` (e.g. `5433:5432`) and update your DSN accordingly.

## Authentication

`POST /v1/users` is unauthenticated so new users can sign up. The request includes a `password`; the response does not include password material.

`POST /v1/auth/token` authenticates with email and password and returns a JWT. The token subject (`sub`) is the user ID and must be passed to protected endpoints as `Authorization: Bearer <jwt>`.

All other endpoints require a valid bearer token. Authentication is enforced by `AuthMiddleware`, which verifies the token and injects claims into the request context.

## API usage examples

### Sign up

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

Expected: `201 Created` with a `UserResponse`.

### Authenticate

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/v1/auth/token \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"correct horse battery staple"}' \
  | jq -r '.token')
```

Expected: `200 OK` with `{"token": "<jwt>"}`.

### Create a bank account

```bash
curl -i -X POST http://localhost:8080/v1/accounts \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name": "Current Account", "accountType": "personal"}'
```

Expected: `201 Created` with a `BankAccountResponse`.

### Deposit funds

```bash
curl -i -X POST http://localhost:8080/v1/accounts/<accountNumber>/transactions \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"amount": 50.00, "currency": "GBP", "type": "deposit", "reference": "Top up"}'
```

Expected: `201 Created` with a `TransactionResponse`.

### Withdraw funds

```bash
curl -i -X POST http://localhost:8080/v1/accounts/<accountNumber>/transactions \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"amount": 20.00, "currency": "GBP", "type": "withdrawal", "reference": "ATM"}'
```

Expected: `201 Created`. Returns `422 Unprocessable Entity` if balance is insufficient.

### Fetch account balance

```bash
curl -i http://localhost:8080/v1/accounts/<accountNumber> \
  -H "Authorization: Bearer $TOKEN"
```

Expected: `200 OK` with the current balance.
