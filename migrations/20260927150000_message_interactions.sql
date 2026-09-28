-- +goose Up
-- +goose StatementBegin
ALTER TABLE messages
    ADD COLUMN forwarded_from_message_id UUID;

ALTER TABLE messages
    ADD CONSTRAINT messages_forwarded_from_message_id_fkey
        FOREIGN KEY (forwarded_from_message_id) REFERENCES messages(id) ON DELETE SET NULL;

CREATE INDEX idx_messages_forwarded_from ON messages(forwarded_from_message_id)
    WHERE forwarded_from_message_id IS NOT NULL;

CREATE TABLE message_reactions (
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reaction TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (message_id, user_id, reaction),
    CONSTRAINT message_reactions_reaction_check CHECK (
        reaction = btrim(reaction) AND char_length(reaction) BETWEEN 1 AND 16
    )
);

CREATE INDEX idx_message_reactions_message ON message_reactions(message_id, reaction);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM messages WHERE forwarded_from_message_id IS NOT NULL)
       OR EXISTS (SELECT 1 FROM message_reactions) THEN
        RAISE EXCEPTION 'cannot roll back message interactions while interaction data exists';
    END IF;
END;
$$;

DROP TABLE message_reactions;
DROP INDEX IF EXISTS idx_messages_forwarded_from;
ALTER TABLE messages
    DROP CONSTRAINT messages_forwarded_from_message_id_fkey,
    DROP COLUMN forwarded_from_message_id;
-- +goose StatementEnd
