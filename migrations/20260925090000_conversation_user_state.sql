-- +goose Up
-- +goose StatementBegin
CREATE TABLE conversation_user_state (
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    archived_at TIMESTAMPTZ,
    hidden_at TIMESTAMPTZ,
    pinned_at TIMESTAMPTZ,
    muted_until TIMESTAMPTZ,
    modified_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (conversation_id, user_id)
);

CREATE INDEX idx_conversation_user_state_active
    ON conversation_user_state(user_id, archived_at, pinned_at)
    WHERE hidden_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_conversation_user_state_active;
DROP TABLE IF EXISTS conversation_user_state;
-- +goose StatementEnd
