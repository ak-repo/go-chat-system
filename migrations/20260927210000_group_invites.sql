-- +goose Up
-- +goose StatementBegin
CREATE TABLE group_invites (
    id UUID PRIMARY KEY,
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    creator_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    token_hash BYTEA NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    max_uses INTEGER,
    use_count INTEGER NOT NULL DEFAULT 0,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT group_invites_expiry_check CHECK (expires_at > created_at),
    CONSTRAINT group_invites_max_uses_check CHECK (max_uses IS NULL OR max_uses > 0),
    CONSTRAINT group_invites_use_count_check CHECK (use_count >= 0 AND (max_uses IS NULL OR use_count <= max_uses)),
    CONSTRAINT group_invites_revoked_order_check CHECK (revoked_at IS NULL OR revoked_at >= created_at)
);

CREATE INDEX idx_group_invites_conversation_created
    ON group_invites(conversation_id, created_at DESC, id);
CREATE INDEX idx_group_invites_active_expiry
    ON group_invites(expires_at) WHERE revoked_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM group_invites) THEN
        RAISE EXCEPTION 'cannot roll back group invites migration while invite records exist';
    END IF;
END;
$$;

DROP TABLE group_invites;
-- +goose StatementEnd
