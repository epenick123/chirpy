-- +goose Up
CREATE TABLE refresh_tokens (
    token TEXT PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    user_id UUID NOT NULL REFERENCES users(id),
    expires_at TIMESTAMP NOT NULL,
    revoked_at TIMESTAMP  -- null if not revoked
);

-- +goose Down
DROP TABLE refresh_tokens;