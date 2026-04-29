-- Bank accounts owned by users. Aligns with openapi.yaml BankAccountResponse.

CREATE TABLE bank_accounts (
    account_number TEXT PRIMARY KEY
        CONSTRAINT bank_accounts_number_format CHECK (account_number ~ '^01[0-9]{6}$'),
    user_id TEXT NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    sort_code TEXT NOT NULL DEFAULT '10-10-10'
        CONSTRAINT bank_accounts_sort_code_allowed CHECK (sort_code = '10-10-10'),
    name TEXT NOT NULL,
    account_type TEXT NOT NULL
        CONSTRAINT bank_accounts_type_allowed CHECK (account_type = 'personal'),
    balance BIGINT NOT NULL DEFAULT 0
        CONSTRAINT bank_accounts_balance_range CHECK (balance >= 0 AND balance <= 1000000),
    currency TEXT NOT NULL DEFAULT 'GBP'
        CONSTRAINT bank_accounts_currency_allowed CHECK (currency = 'GBP'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_bank_accounts_user_id ON bank_accounts (user_id);

COMMENT ON TABLE bank_accounts IS 'Bank accounts owned by Eagle Bank users.';
