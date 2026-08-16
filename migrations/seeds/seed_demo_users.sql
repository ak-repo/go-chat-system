-- ============================================================================
-- Seed: demo users + friendship
-- ----------------------------------------------------------------------------
-- Creates two demo users (alice, bob) and a mutual friendship between them.
--
-- Credentials (both users):
--   email:    alice@example.com / bob@example.com
--   password: password123
--
-- The password_hash is a bcrypt hash (cost 10, same as utils.HashPassword)
-- of "password123", so both users can log in through the normal login flow.
--
-- Idempotent: safe to run multiple times.
--   - Users are inserted only when their email does not already exist.
--   - The friendship is created only when it does not already exist.
--   - Friendship rows are resolved by email, so they also link users that
--     were registered through the application with different UUIDs.
--
-- Run with:
--   make seed
-- ============================================================================

INSERT INTO users (id, username, email, password_hash, role)
VALUES
    ('00000000-0000-0000-0000-000000000001', 'alice', 'alice@example.com', '$2a$10$C03cCQ2P5mFrojgz0MUStOUPrrUQ0IH3I5Q/bSMMWcFkUnhfBWuhy', 'user'),
    ('00000000-0000-0000-0000-000000000002', 'bob',   'bob@example.com',   '$2a$10$C03cCQ2P5mFrojgz0MUStOUPrrUQ0IH3I5Q/bSMMWcFkUnhfBWuhy', 'user')
ON CONFLICT (email) DO NOTHING;

-- Mutual friendship (both directions), resolved by email so it also works when
-- the users were registered through the app with different UUIDs.
INSERT INTO friends (user_id, friend_id)
SELECT a.id, b.id
FROM users a, users b
WHERE (a.email = 'alice@example.com' AND b.email = 'bob@example.com')
   OR (a.email = 'bob@example.com'   AND b.email = 'alice@example.com')
ON CONFLICT DO NOTHING;