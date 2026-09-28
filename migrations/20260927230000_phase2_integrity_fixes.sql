-- +goose Up
-- +goose StatementBegin
ALTER TABLE messages ADD CONSTRAINT messages_id_conversation_unique UNIQUE (id, conversation_id);

ALTER TABLE notifications
    ADD CONSTRAINT notifications_message_conversation_fkey
    FOREIGN KEY (message_id, conversation_id)
    REFERENCES messages(id, conversation_id) ON DELETE CASCADE;

CREATE FUNCTION enforce_message_reference_conversations() RETURNS trigger AS $$
BEGIN
    IF NEW.reply_to_message_id IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM messages parent
        WHERE parent.id = NEW.reply_to_message_id
          AND parent.conversation_id = NEW.conversation_id
    ) THEN
        RAISE EXCEPTION 'reply target must belong to the same conversation' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER messages_reference_conversations
BEFORE INSERT OR UPDATE OF conversation_id, reply_to_message_id ON messages
FOR EACH ROW EXECUTE FUNCTION enforce_message_reference_conversations();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER messages_reference_conversations ON messages;
DROP FUNCTION enforce_message_reference_conversations();
ALTER TABLE notifications DROP CONSTRAINT notifications_message_conversation_fkey;
ALTER TABLE messages DROP CONSTRAINT messages_id_conversation_unique;
-- +goose StatementEnd
