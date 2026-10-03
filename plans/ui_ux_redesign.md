# UI/UX Redesign Implementation Plan

## Goal

Give the existing chat application a cohesive, responsive, WhatsApp Web-inspired workspace while using a distinct plum-and-teal visual identity. Preserve all existing routes, API calls, permissions, and implemented capabilities.

## Product boundaries

- Use the familiar conversation-first pattern without copying WhatsApp branding or visual assets.
- Keep the current text-only message composer; do not imply media, calling, search, threads, push, last-seen, or avatar-upload support.
- Preserve current account, friends, group, notification, and message behaviors.
- Backend, database, and API changes are out of scope.

## Work sequence

1. Define shared visual tokens, accessible controls, and responsive shell for signed-in routes.
2. Rework conversations into a clear chat landing/list with active and archived states and conversation actions.
3. Refine direct/group chat hierarchy, message states/actions, and composer while retaining realtime behavior.
4. Apply the design system to friends/discovery, group creation/settings, notifications, profiles, invites, and auth/recovery/verification screens.
5. Review keyboard/focus, small-screen behavior, empty/loading/error/offline states, then run frontend lint and build.

## Verification

`cd web && npm run lint && npm run build`

Manually verify authentication, list navigation, direct and group chat interactions, unread state, notifications, group administration, and mobile navigation.
