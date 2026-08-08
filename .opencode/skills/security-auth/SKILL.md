---
name: security-auth
description: Apply authentication, authorization, WebSocket trust, rate-limit, CORS, secret, and data-exposure rules to go-chat-system changes.
compatibility: opencode
metadata:
  project: go-chat-system
---

# Security / Auth

Use this skill for authentication, users, friendships, blocks, messages,
WebSockets, CORS, rate limiting, credentials, and uploads.

## Identity

Authentication establishes actor identity.

Never accept the authenticated actor from:

- request body
- query body equivalent
- WebSocket JSON payload

Use server-authenticated context.

## Authorization

Every cross-user operation must answer:

- who is the actor?
- what resource/user is targeted?
- what relationship is required?
- does a block relationship forbid it?
- can one user enumerate another user's private data?

Do not rely on frontend controls for authorization.

## JWT

Validate:

- signing method
- signature
- expiry
- required claims
- issuer when project policy uses it

Do not log full tokens.

JWT secrets must not be committed.

## WebSocket

Review:

- auth before upgrade
- allowed origins
- trusted actor identity
- event-level authorization
- message size
- per-client rate limits
- malformed payload handling
- backpressure
- replay/duplicate effects

## CORS / origin

Production origins should be explicit.

Do not use permissive `*`/allow-all origin logic together with credentials or
authenticated browser workflows without an explicit reason.

HTTP CORS and WebSocket origin policy should not contradict each other.

## Passwords

- hash before persistence
- never return password hashes
- never log plaintext passwords
- use bounded validation

## Error leakage

Client errors must not expose:

- SQL errors
- stack traces
- filesystem paths
- secrets
- internal hostnames/credentials

## Rate limiting / abuse

Consider separate abuse surfaces:

- login/register
- user search
- friend requests
- messages
- WebSocket event flood
- reconnect storm

Rate limits must fail safely if infrastructure behavior is defined.

## SQL

All user input must be parameterized.

Authorization filters belong in the query/service path, not only after fetching
private data.

## Secrets

Treat `config/config.yaml` production credentials as sensitive.

Prefer environment overrides/secret management for deployment.
