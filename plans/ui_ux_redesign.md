# WhatsApp Web desktop redesign

## Reference and scope
Use the six screenshots in `/home/ak/Pictures/whatsapp` as the dark-mode reference. Preserve existing backend contracts and authorization. No calling, media, communities, favourites, self-chat, or encryption claims.

## Implementation
1. Replace plum/teal and hard-coded utility colors with semantic light/dark tokens. Light is the default; persist the preference before rendering. Dark: sidebar/header #1c1d1d, rail #232424, selected #323232, composer #292a2a, incoming #2a2a2a, outgoing #1b4e3a, accent #26be63.
2. Persistent protected-route shell: 64px rail, clamp(360px, 30vw, 576px) sidebar, flexible conversation. Below 900px show one primary pane. Default authenticated navigation goes to Chats; preserve deep links.
3. Shared paginated list state, names, session previews, filters/archive, mutations, and reconnect refresh. No per-chat history fetches for previews or misleading modified_at timestamps. Real presence only.
4. Left-panel new-chat picker with existing friends and group routes; preserve open conversation.
5. Compact bubbles/tails, date separators, receipts, actions, SVG wallpaper, pill composer. Enter sends, Shift+Enter inserts a newline, mentions take precedence. Retain disconnected drafts and scroll position while reading.
6. Theme all existing pages; accessible icons, dialogs, inline feedback, and Profile theme selection.

## Interfaces
Frontend components/context only. REST, database, migrations, and WebSocket wire formats unchanged. API and socket transport stay in their existing layers.

## Verification
Frontend lint, build, tests; focused themes, filters, picker/navigation, composer keyboard, and dialog focus tests. Review dark against reference at 1920x912 application viewport; responsive review at 1440, 1024, 768, 390, and 320px. Browser comparison requires an available browser runtime.

## Defaults
Light is a chosen counterpart because screenshots are dark only. Keep configured application name, initials avatars, text-only messaging, and existing permissions.

## Completion
Implemented and reviewed. Frontend lint/build and 25 tests pass; Chromium verified desktop geometry, both themes, contact picker, mentions, responsive navigation, scroll preservation, and reconnect draft retention using isolated fixtures. See `plans/ui_ux_review.md` for evidence and limitations.
