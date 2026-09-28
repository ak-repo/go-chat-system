export interface PresenceState { online: boolean; timestamp: string }
export type PresenceByUser = Record<string, PresenceState>;
export interface PresenceUpdate { user_id: string; online: boolean; timestamp: string }
export interface PresenceSnapshot { users: Array<{ user_id: string; online: boolean }>; timestamp: string }

export function applyPresenceUpdate(current: PresenceByUser, update: PresenceUpdate): PresenceByUser {
  const previous = current[update.user_id];
  if (previous && Date.parse(previous.timestamp) > Date.parse(update.timestamp)) return current;
  return { ...current, [update.user_id]: { online: update.online, timestamp: update.timestamp } };
}

export function applyPresenceSnapshot(current: PresenceByUser, snapshot: PresenceSnapshot): PresenceByUser {
  const included = new Set(snapshot.users.map((user) => user.user_id));
  const next = Object.fromEntries(Object.entries(current).filter(([id, value]) => included.has(id) || Date.parse(value.timestamp) > Date.parse(snapshot.timestamp)));
  return snapshot.users.reduce((state, user) => applyPresenceUpdate(state, { ...user, timestamp: snapshot.timestamp }), next);
}

export type TypingByConversation = Record<string, Record<string, number>>;

export function applyTypingUpdate(current: TypingByConversation, conversationId: string, userId: string, state: boolean, now: number, ttl: number): TypingByConversation {
  const conversation = { ...(current[conversationId] ?? {}) };
  if (state) conversation[userId] = now + ttl;
  else delete conversation[userId];
  return { ...current, [conversationId]: conversation };
}

export function expireTyping(current: TypingByConversation, now: number): TypingByConversation {
  let changed = false;
  const next: TypingByConversation = {};
  for (const [conversationId, users] of Object.entries(current)) {
    const active = Object.fromEntries(Object.entries(users).filter(([, expiresAt]) => expiresAt > now));
    if (Object.keys(active).length !== Object.keys(users).length) changed = true;
    next[conversationId] = active;
  }
  return changed ? next : current;
}

export function mergeUnreadSnapshot(current: Record<string, number>, snapshot: Record<string, number>): Record<string, number> {
  const next = { ...snapshot };
  for (const [conversationId, count] of Object.entries(current)) {
    next[conversationId] = Math.max(count, snapshot[conversationId] ?? 0);
  }
  return next;
}
