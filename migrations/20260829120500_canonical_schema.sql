-- +goose Up
-- +goose StatementBegin
-- Canonical fresh-install schema for go-chat-system.
-- This migration replaces the previously split, unapplied migration set.
-- The Down section is destructive and must not be used as a production rollback.

CREATE TABLE users (
    id UUID PRIMARY KEY,
    username TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'user',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    modified_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ DEFAULT NULL,
    verified_at TIMESTAMPTZ DEFAULT NULL
);

CREATE TABLE friends (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    friend_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    modified_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ DEFAULT NULL,
    PRIMARY KEY (user_id, friend_id),
    CONSTRAINT no_self_friend CHECK (user_id <> friend_id)
);

CREATE TABLE blocks (
    blocker_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    blocked_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    modified_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ DEFAULT NULL,
    PRIMARY KEY (blocker_id, blocked_id),
    CONSTRAINT no_self_block CHECK (blocker_id <> blocked_id)
);

CREATE TABLE friend_requests (
    id UUID PRIMARY KEY,
    sender_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    receiver_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    modified_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ DEFAULT NULL,
    CONSTRAINT no_self_request CHECK (sender_id <> receiver_id),
    CONSTRAINT friend_requests_status_check
        CHECK (status IN ('pending', 'accepted', 'rejected', 'blocked'))
);

CREATE TABLE sessions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token_hash BYTEA NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    replaced_at TIMESTAMPTZ,
    CONSTRAINT sessions_expiry_check CHECK (expires_at > created_at),
    CONSTRAINT sessions_revoke_order_check
        CHECK (revoked_at IS NULL OR revoked_at >= created_at),
    CONSTRAINT sessions_replace_order_check
        CHECK (replaced_at IS NULL OR replaced_at >= created_at)
);

CREATE TABLE account_tokens (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    purpose TEXT NOT NULL,
    token_hash BYTEA NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    consumed_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    CONSTRAINT account_tokens_purpose_check
        CHECK (purpose IN ('verification', 'password_reset')),
    CONSTRAINT account_tokens_expiry_check CHECK (expires_at > created_at),
    CONSTRAINT account_tokens_consumed_order_check
        CHECK (consumed_at IS NULL OR consumed_at >= created_at),
    CONSTRAINT account_tokens_revoked_order_check
        CHECK (revoked_at IS NULL OR revoked_at >= created_at),
    CONSTRAINT account_tokens_one_terminal_state
        CHECK (NOT (consumed_at IS NOT NULL AND revoked_at IS NOT NULL))
);

CREATE TABLE conversations (
    id UUID PRIMARY KEY,
    kind TEXT NOT NULL DEFAULT 'direct',
    user_one_id UUID REFERENCES users(id) ON DELETE RESTRICT,
    user_two_id UUID REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    modified_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT conversations_kind_check CHECK (kind IN ('direct')),
    CONSTRAINT conversations_direct_pair_check CHECK (
        kind <> 'direct' OR
        (user_one_id IS NOT NULL AND user_two_id IS NOT NULL)
    )
);

CREATE TABLE conversation_members (
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    left_at TIMESTAMPTZ,
    PRIMARY KEY (conversation_id, user_id),
    CONSTRAINT conversation_members_leave_order_check
        CHECK (left_at IS NULL OR left_at >= joined_at)
);

CREATE TABLE messages (
    id UUID PRIMARY KEY,
    sender_id UUID NOT NULL REFERENCES users(id),
    receiver_id UUID NOT NULL REFERENCES users(id),
    body TEXT NOT NULL,
    is_group BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    modified_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ DEFAULT NULL,
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE RESTRICT,
    client_message_id UUID,
    edited_at TIMESTAMPTZ,
    reply_to_message_id UUID REFERENCES messages(id) ON DELETE SET NULL,
    CONSTRAINT messages_reply_not_self_check
        CHECK (reply_to_message_id IS NULL OR reply_to_message_id <> id)
);

CREATE TABLE message_deliveries (
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    recipient_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'sent',
    status_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    failure_code TEXT,
    PRIMARY KEY (message_id, recipient_id),
    CONSTRAINT message_deliveries_status_check
        CHECK (status IN ('sent', 'delivered', 'read', 'failed')),
    CONSTRAINT message_deliveries_failure_check
        CHECK ((status = 'failed') = (failure_code IS NOT NULL))
);

CREATE TABLE conversation_read_state (
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    last_read_message_id UUID REFERENCES messages(id) ON DELETE SET NULL,
    last_read_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (conversation_id, user_id),
    CONSTRAINT conversation_read_state_message_time_check
        CHECK (last_read_at IS NOT NULL)
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_friends_user ON friends(user_id);
CREATE INDEX idx_blocks_blocker ON blocks(blocker_id);
CREATE UNIQUE INDEX unique_pending_request_unordered
    ON friend_requests (
        LEAST(sender_id, receiver_id),
        GREATEST(sender_id, receiver_id)
    ) WHERE status = 'pending';
CREATE INDEX idx_friend_requests_receiver
    ON friend_requests(receiver_id) WHERE status = 'pending';
CREATE INDEX idx_sessions_user_active
    ON sessions(user_id, expires_at) WHERE revoked_at IS NULL;
CREATE INDEX idx_users_username_active
    ON users(lower(username)) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_email_active
    ON users(lower(email)) WHERE deleted_at IS NULL;
CREATE INDEX idx_account_tokens_lookup
    ON account_tokens(user_id, purpose, expires_at)
    WHERE consumed_at IS NULL AND revoked_at IS NULL;
CREATE UNIQUE INDEX conversations_direct_pair_unique
    ON conversations (
        LEAST(user_one_id, user_two_id),
        GREATEST(user_one_id, user_two_id)
    ) WHERE kind = 'direct';
CREATE INDEX idx_conversation_members_user
    ON conversation_members(user_id, conversation_id) WHERE left_at IS NULL;
CREATE INDEX idx_messages_receiver_created
    ON messages(receiver_id, created_at DESC);
CREATE INDEX idx_messages_sender_created
    ON messages(sender_id, created_at DESC);
CREATE INDEX idx_messages_conversation_chronological
    ON messages(conversation_id, created_at, id);
CREATE UNIQUE INDEX messages_client_idempotency_unique
    ON messages(conversation_id, sender_id, client_message_id)
    WHERE client_message_id IS NOT NULL;
CREATE INDEX idx_messages_reply
    ON messages(reply_to_message_id) WHERE reply_to_message_id IS NOT NULL;
CREATE INDEX idx_message_deliveries_recipient_status
    ON message_deliveries(recipient_id, status, status_at);
CREATE INDEX idx_conversation_read_state_user
    ON conversation_read_state(user_id, conversation_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- This teardown is intentionally destructive and removes all application data.
DROP INDEX IF EXISTS idx_conversation_read_state_user;
DROP INDEX IF EXISTS idx_message_deliveries_recipient_status;
DROP INDEX IF EXISTS idx_messages_reply;
DROP INDEX IF EXISTS messages_client_idempotency_unique;
DROP INDEX IF EXISTS idx_messages_conversation_chronological;
DROP INDEX IF EXISTS idx_messages_sender_created;
DROP INDEX IF EXISTS idx_messages_receiver_created;
DROP INDEX IF EXISTS idx_conversation_members_user;
DROP INDEX IF EXISTS conversations_direct_pair_unique;
DROP INDEX IF EXISTS idx_account_tokens_lookup;
DROP INDEX IF EXISTS idx_users_email_active;
DROP INDEX IF EXISTS idx_users_username_active;
DROP INDEX IF EXISTS idx_sessions_user_active;
DROP INDEX IF EXISTS idx_friend_requests_receiver;
DROP INDEX IF EXISTS unique_pending_request_unordered;
DROP INDEX IF EXISTS idx_blocks_blocker;
DROP INDEX IF EXISTS idx_friends_user;
DROP INDEX IF EXISTS idx_users_email;

DROP TABLE IF EXISTS conversation_read_state;
DROP TABLE IF EXISTS message_deliveries;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS conversation_members;
DROP TABLE IF EXISTS conversations;
DROP TABLE IF EXISTS account_tokens;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS friend_requests;
DROP TABLE IF EXISTS blocks;
DROP TABLE IF EXISTS friends;
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
