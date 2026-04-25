-- users: aligns with openapi.yaml UserResponse / CreateUserRequest / UpdateUserRequest
-- Public id matches components.schemas.UserResponse.properties.id (usr- + alphanumeric).

CREATE TABLE users (
    id TEXT PRIMARY KEY
        CONSTRAINT users_id_format CHECK (id ~ '^usr-[A-Za-z0-9]+$'),

    name TEXT NOT NULL,

    address_line1 TEXT NOT NULL,
    address_line2 TEXT,
    address_line3 TEXT,
    town            TEXT NOT NULL,
    county          TEXT NOT NULL,
    postcode        TEXT NOT NULL,

    phone_number TEXT NOT NULL
        CONSTRAINT users_phone_e164 CHECK (phone_number ~ '^\+[1-9][0-9]{1,14}$'),

    email TEXT NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT users_email_unique UNIQUE (email)
);

CREATE INDEX idx_users_created_at ON users (created_at);

COMMENT ON TABLE users IS 'Bank customers; fields mirror OpenAPI UserResponse (address nested object flattened to columns).';
