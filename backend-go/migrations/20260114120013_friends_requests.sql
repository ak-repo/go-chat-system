-- +goose Up
-- +goose StatementBegin
CREATE TABLE friend_requests (
    id           UUID PRIMARY KEY,
    sender_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    receiver_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status       TEXT NOT NULL CHECK (status IN ('pending', 'accepted', 'blocked')),
    created_at   TIMESTAMP NOT NULL DEFAULT now(),

    CONSTRAINT no_self_request CHECK (sender_id <> receiver_id),
    CONSTRAINT unique_pending_request UNIQUE (sender_id, receiver_id)
);

CREATE INDEX idx_friend_requests_receiver ON friend_requests (receiver_id) WHERE status = 'pending';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS friend_requests;
-- +goose StatementEnd
