-- Transactions applied to a bank account. Aligns with openapi.yaml TransactionResponse.

CREATE TABLE transactions (
    id TEXT PRIMARY KEY
        CONSTRAINT transactions_id_format CHECK (id ~ '^tan-[A-Za-z0-9]+$'),
    account_number TEXT NOT NULL REFERENCES bank_accounts (account_number) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    amount BIGINT NOT NULL
        CONSTRAINT transactions_amount_range CHECK (amount >= 0 AND amount <= 1000000),
    currency TEXT NOT NULL
        CONSTRAINT transactions_currency_allowed CHECK (currency = 'GBP'),
    type TEXT NOT NULL
        CONSTRAINT transactions_type_allowed CHECK (type IN ('deposit', 'withdrawal')),
    reference TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_transactions_account_number ON transactions (account_number);
CREATE INDEX idx_transactions_user_id ON transactions (user_id);

COMMENT ON TABLE transactions IS 'Deposit and withdrawal events applied to Eagle Bank accounts.';
