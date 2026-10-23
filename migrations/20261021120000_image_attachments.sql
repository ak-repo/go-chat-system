-- +goose Up
ALTER TABLE messages ADD COLUMN message_type TEXT NOT NULL DEFAULT 'text'
    CHECK (message_type IN ('text','image'));
CREATE TABLE attachments (
    id UUID PRIMARY KEY,
    owner_id UUID NOT NULL REFERENCES users(id),
    conversation_id UUID NOT NULL REFERENCES conversations(id),
    filename TEXT NOT NULL,
    declared_mime TEXT NOT NULL,
    declared_size BIGINT NOT NULL CHECK (declared_size > 0 AND declared_size <= 10485760),
    mime_type TEXT NOT NULL DEFAULT '',
    size_bytes BIGINT NOT NULL DEFAULT 0,
    width INTEGER NOT NULL DEFAULT 0,
    height INTEGER NOT NULL DEFAULT 0,
    staging_key TEXT NOT NULL UNIQUE,
    full_key TEXT NOT NULL UNIQUE,
    thumbnail_key TEXT NOT NULL UNIQUE,
    state TEXT NOT NULL DEFAULT 'pending' CHECK (state IN ('pending','queued','processing','ready','failed','deleting','deleted')),
    error TEXT NOT NULL DEFAULT '',
    staging_cleaned BOOLEAN NOT NULL DEFAULT FALSE,
    attempts INTEGER NOT NULL DEFAULT 0,
    lease_until TIMESTAMPTZ,
    upload_expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX attachments_work_idx ON attachments(state, updated_at);
CREATE TABLE message_attachments (
    message_id UUID PRIMARY KEY REFERENCES messages(id) ON DELETE CASCADE,
    attachment_id UUID NOT NULL REFERENCES attachments(id)
);
CREATE INDEX message_attachments_attachment_idx ON message_attachments(attachment_id);

-- +goose Down
-- Rollback is guarded: remove image messages/media through the application first.
-- +goose StatementBegin
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM messages WHERE message_type='image')
       OR EXISTS (SELECT 1 FROM attachments WHERE state <> 'deleted') THEN
        RAISE EXCEPTION 'Remove image messages and finish media cleanup before rollback';
    END IF;
END $$;
-- +goose StatementEnd
DROP TABLE message_attachments;
DROP TABLE attachments;
ALTER TABLE messages DROP COLUMN message_type;
