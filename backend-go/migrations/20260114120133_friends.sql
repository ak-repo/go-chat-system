-- +goose Up
-- +goose StatementBegin
CREATE TABLE friends (
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    friend_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    since      TIMESTAMP NOT NULL DEFAULT now(),

    PRIMARY KEY (user_id, friend_id),
    CONSTRAINT no_self_friend CHECK (user_id <> friend_id)
);

CREATE INDEX idx_friends_user ON friends (user_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS friends;
-- +goose StatementEnd
