-- +goose Up
-- +goose StatementBegin
ALTER TABLE conversations
    ADD COLUMN name TEXT,
    ADD COLUMN description TEXT,
    ADD COLUMN creator_id UUID,
    ADD COLUMN avatar_url TEXT;

ALTER TABLE conversations
    ADD CONSTRAINT conversations_creator_id_fkey
        FOREIGN KEY (creator_id) REFERENCES users(id) ON DELETE RESTRICT,
    ADD CONSTRAINT conversations_name_length_check
        CHECK (name IS NULL OR (name = btrim(name) AND char_length(name) BETWEEN 1 AND 100)) NOT VALID,
    ADD CONSTRAINT conversations_description_length_check
        CHECK (description IS NULL OR char_length(description) <= 2000) NOT VALID,
    ADD CONSTRAINT conversations_avatar_url_length_check
        CHECK (avatar_url IS NULL OR char_length(avatar_url) <= 2048) NOT VALID;

ALTER TABLE conversations VALIDATE CONSTRAINT conversations_name_length_check;
ALTER TABLE conversations VALIDATE CONSTRAINT conversations_description_length_check;
ALTER TABLE conversations VALIDATE CONSTRAINT conversations_avatar_url_length_check;

ALTER TABLE conversations
    DROP CONSTRAINT conversations_kind_check,
    DROP CONSTRAINT conversations_direct_pair_check;

ALTER TABLE conversations
    ADD CONSTRAINT conversations_kind_check
        CHECK (kind IN ('direct', 'group')) NOT VALID,
    ADD CONSTRAINT conversations_kind_shape_check
        CHECK (
            (
                kind = 'direct'
                AND user_one_id IS NOT NULL
                AND user_two_id IS NOT NULL
                AND user_one_id <> user_two_id
                AND name IS NULL
                AND description IS NULL
                AND creator_id IS NULL
                AND avatar_url IS NULL
            )
            OR
            (
                kind = 'group'
                AND user_one_id IS NULL
                AND user_two_id IS NULL
                AND name IS NOT NULL
                AND creator_id IS NOT NULL
            )
        ) NOT VALID;

ALTER TABLE conversations VALIDATE CONSTRAINT conversations_kind_check;
ALTER TABLE conversations VALIDATE CONSTRAINT conversations_kind_shape_check;

ALTER TABLE conversation_members
    ADD COLUMN role TEXT NOT NULL DEFAULT 'member',
    ADD COLUMN added_by UUID,
    ADD COLUMN modified_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

ALTER TABLE conversation_members
    ADD CONSTRAINT conversation_members_role_check
        CHECK (role IN ('owner', 'admin', 'member')) NOT VALID,
    ADD CONSTRAINT conversation_members_added_by_fkey
        FOREIGN KEY (added_by) REFERENCES users(id) ON DELETE SET NULL,
    ADD CONSTRAINT conversation_members_modified_order_check
        CHECK (modified_at >= joined_at) NOT VALID;

ALTER TABLE conversation_members VALIDATE CONSTRAINT conversation_members_role_check;
ALTER TABLE conversation_members VALIDATE CONSTRAINT conversation_members_modified_order_check;

ALTER TABLE messages ALTER COLUMN receiver_id DROP NOT NULL;

ALTER TABLE messages
    ADD CONSTRAINT messages_sender_not_receiver_check
        CHECK (receiver_id IS NULL OR sender_id <> receiver_id) NOT VALID,
    ADD CONSTRAINT messages_group_receiver_shape_check
        CHECK (
            (is_group = FALSE AND receiver_id IS NOT NULL)
            OR (is_group = TRUE AND receiver_id IS NULL)
        ) NOT VALID,
    ADD CONSTRAINT messages_sender_membership_fkey
        FOREIGN KEY (conversation_id, sender_id)
        REFERENCES conversation_members(conversation_id, user_id)
        ON DELETE RESTRICT NOT VALID,
    ADD CONSTRAINT messages_receiver_membership_fkey
        FOREIGN KEY (conversation_id, receiver_id)
        REFERENCES conversation_members(conversation_id, user_id)
        ON DELETE RESTRICT NOT VALID;

ALTER TABLE messages VALIDATE CONSTRAINT messages_sender_not_receiver_check;
ALTER TABLE messages VALIDATE CONSTRAINT messages_group_receiver_shape_check;
ALTER TABLE messages VALIDATE CONSTRAINT messages_sender_membership_fkey;
ALTER TABLE messages VALIDATE CONSTRAINT messages_receiver_membership_fkey;

CREATE FUNCTION enforce_message_conversation_shape()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    conversation_kind TEXT;
BEGIN
    SELECT kind INTO conversation_kind
    FROM conversations
    WHERE id = NEW.conversation_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'message conversation % does not exist', NEW.conversation_id
            USING ERRCODE = '23503';
    END IF;

    IF conversation_kind = 'direct' THEN
        IF NEW.receiver_id IS NULL OR NEW.is_group THEN
            RAISE EXCEPTION 'direct messages require receiver_id and is_group=false'
                USING ERRCODE = '23514', CONSTRAINT = 'messages_conversation_kind_check';
        END IF;
    ELSIF conversation_kind = 'group' THEN
        IF NEW.receiver_id IS NOT NULL OR NOT NEW.is_group THEN
            RAISE EXCEPTION 'group messages require receiver_id=NULL and is_group=true'
                USING ERRCODE = '23514', CONSTRAINT = 'messages_conversation_kind_check';
        END IF;
    ELSE
        RAISE EXCEPTION 'unsupported conversation kind %', conversation_kind
            USING ERRCODE = '23514', CONSTRAINT = 'messages_conversation_kind_check';
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER messages_conversation_kind_check
BEFORE INSERT OR UPDATE OF conversation_id, receiver_id, is_group
ON messages
FOR EACH ROW
EXECUTE FUNCTION enforce_message_conversation_shape();

CREATE INDEX idx_conversations_group_modified
    ON conversations(modified_at DESC, id) WHERE kind = 'group';
CREATE INDEX idx_conversation_members_active_role
    ON conversation_members(conversation_id, role, user_id) WHERE left_at IS NULL;
CREATE INDEX idx_conversation_members_active_visibility
    ON conversation_members(conversation_id, user_id, joined_at) WHERE left_at IS NULL;
CREATE INDEX idx_conversation_members_added_by
    ON conversation_members(added_by, conversation_id) WHERE added_by IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM conversations WHERE kind = 'group') THEN
        RAISE EXCEPTION 'cannot roll back group migration while group conversations exist';
    END IF;
    IF EXISTS (SELECT 1 FROM messages WHERE receiver_id IS NULL OR is_group) THEN
        RAISE EXCEPTION 'cannot roll back group migration while group messages exist';
    END IF;
    IF EXISTS (SELECT 1 FROM conversation_members WHERE role <> 'member' OR added_by IS NOT NULL) THEN
        RAISE EXCEPTION 'cannot roll back group migration while group membership metadata exists';
    END IF;
END;
$$;

DROP INDEX IF EXISTS idx_conversation_members_added_by;
DROP INDEX IF EXISTS idx_conversation_members_active_visibility;
DROP INDEX IF EXISTS idx_conversation_members_active_role;
DROP INDEX IF EXISTS idx_conversations_group_modified;

DROP TRIGGER IF EXISTS messages_conversation_kind_check ON messages;
DROP FUNCTION IF EXISTS enforce_message_conversation_shape();

ALTER TABLE messages
    DROP CONSTRAINT IF EXISTS messages_receiver_membership_fkey,
    DROP CONSTRAINT IF EXISTS messages_sender_membership_fkey,
    DROP CONSTRAINT IF EXISTS messages_group_receiver_shape_check,
    DROP CONSTRAINT IF EXISTS messages_sender_not_receiver_check;
ALTER TABLE messages ALTER COLUMN receiver_id SET NOT NULL;

ALTER TABLE conversation_members
    DROP CONSTRAINT IF EXISTS conversation_members_modified_order_check,
    DROP CONSTRAINT IF EXISTS conversation_members_added_by_fkey,
    DROP CONSTRAINT IF EXISTS conversation_members_role_check,
    DROP COLUMN modified_at,
    DROP COLUMN added_by,
    DROP COLUMN role;

ALTER TABLE conversations
    DROP CONSTRAINT IF EXISTS conversations_kind_shape_check,
    DROP CONSTRAINT IF EXISTS conversations_kind_check,
    DROP CONSTRAINT IF EXISTS conversations_avatar_url_length_check,
    DROP CONSTRAINT IF EXISTS conversations_description_length_check,
    DROP CONSTRAINT IF EXISTS conversations_name_length_check,
    DROP CONSTRAINT IF EXISTS conversations_creator_id_fkey,
    DROP COLUMN avatar_url,
    DROP COLUMN creator_id,
    DROP COLUMN description,
    DROP COLUMN name;

ALTER TABLE conversations
    ADD CONSTRAINT conversations_kind_check CHECK (kind IN ('direct')),
    ADD CONSTRAINT conversations_direct_pair_check CHECK (
        kind <> 'direct' OR (user_one_id IS NOT NULL AND user_two_id IS NOT NULL)
    );
-- +goose StatementEnd
