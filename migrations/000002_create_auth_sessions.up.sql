CREATE TABLE auth_sessions (
                               id UUID PRIMARY KEY,
                               user_id UUID NOT NULL,

                               refresh_token_hash TEXT NOT NULL,
                               user_agent TEXT,
                               ip_address INET,

                               created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                               expires_at TIMESTAMPTZ NOT NULL,
                               revoked_at TIMESTAMPTZ,
                               last_used_at TIMESTAMPTZ NOT NULL,

                               CONSTRAINT fk_auth_sessions_user
                                   FOREIGN KEY (user_id)
                                       REFERENCES users(id)
                                       ON DELETE CASCADE
                                       ON UPDATE CASCADE,

                               CONSTRAINT auth_sessions_refresh_token_hash_not_empty_check
                                   CHECK (refresh_token_hash <> ''),

                               CONSTRAINT auth_sessions_expiration_check
                                   CHECK (expires_at > created_at),

                               CONSTRAINT auth_sessions_revoked_at_check
                                   CHECK (revoked_at IS NULL OR revoked_at >= created_at),

                               CONSTRAINT auth_sessions_last_used_at_check
                                   CHECK (last_used_at >= created_at)
);

CREATE INDEX ix_auth_sessions_user_id
    ON auth_sessions(user_id);

CREATE INDEX ix_auth_sessions_expires_at
    ON auth_sessions(expires_at);

CREATE UNIQUE INDEX ux_auth_sessions_refresh_token_hash
    ON auth_sessions(refresh_token_hash);