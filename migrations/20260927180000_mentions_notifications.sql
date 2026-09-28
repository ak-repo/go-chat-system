-- +goose Up
-- +goose StatementBegin
CREATE TABLE message_mentions (
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('user', 'everyone')),
    start_offset INTEGER NOT NULL CHECK (start_offset >= 0),
    length INTEGER NOT NULL CHECK (length > 0),
    PRIMARY KEY (message_id, start_offset),
    CONSTRAINT message_mentions_target_check CHECK (
        (kind = 'user' AND user_id IS NOT NULL) OR
        (kind = 'everyone' AND user_id IS NULL)
    )
);

CREATE INDEX idx_message_mentions_user ON message_mentions(user_id, message_id)
    WHERE user_id IS NOT NULL;

CREATE TABLE notification_preferences (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    message_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    reply_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    mention_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    modified_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE notifications (
    id UUID PRIMARY KEY,
    recipient_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    actor_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK (type IN ('message', 'reply', 'mention')),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(payload) = 'object'),
    dedupe_key TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    read_at TIMESTAMPTZ,
    CHECK (recipient_id <> actor_id)
);

CREATE INDEX idx_notifications_recipient_created ON notifications(recipient_id, created_at DESC, id DESC);
CREATE INDEX idx_notifications_recipient_unread ON notifications(recipient_id, created_at DESC) WHERE read_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM message_mentions)
       OR EXISTS (SELECT 1 FROM notifications)
       OR EXISTS (SELECT 1 FROM notification_preferences) THEN
        RAISE EXCEPTION 'cannot roll back mentions/notifications while feature data exists';
    END IF;
END;
$$;

DROP TABLE notifications;
DROP TABLE notification_preferences;
DROP TABLE message_mentions;
-- +goose StatementEnd
