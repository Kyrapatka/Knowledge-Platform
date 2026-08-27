CREATE TABLE users (
                       id UUID PRIMARY KEY,
                       nickname VARCHAR(16) NOT NULL,
                       nickname_normalized VARCHAR(16) NOT NULL,
                       password_hash TEXT NOT NULL,
                       status VARCHAR(16) NOT NULL DEFAULT 'active',
                       created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX ux_users_nickname_normalized
    ON users(nickname_normalized);