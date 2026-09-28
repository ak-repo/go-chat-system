import { describe, expect, it } from 'vitest';
import { applyPresenceSnapshot, applyPresenceUpdate, applyTypingUpdate, expireTyping, mergeUnreadSnapshot } from './realtimeState';

describe('presence ordering', () => {
  it('ignores stale updates and stale snapshot entries', () => {
    const latest = applyPresenceUpdate({}, { user_id: 'u1', online: true, timestamp: '2026-01-01T00:00:02Z' });
    expect(applyPresenceUpdate(latest, { user_id: 'u1', online: false, timestamp: '2026-01-01T00:00:01Z' })).toBe(latest);
    expect(applyPresenceSnapshot(latest, { timestamp: '2026-01-01T00:00:01Z', users: [{ user_id: 'u1', online: false }] }).u1.online).toBe(true);
  });
});

describe('typing expiry', () => {
  it('tracks multiple users and expires a lost stop', () => {
    let state = applyTypingUpdate({}, 'conversation', 'one', true, 100, 5000);
    state = applyTypingUpdate(state, 'conversation', 'two', true, 200, 5000);
    expect(Object.keys(expireTyping(state, 1000).conversation)).toEqual(['one', 'two']);
    expect(Object.keys(expireTyping(state, 5150).conversation)).toEqual(['two']);
    expect(Object.keys(expireTyping(state, 5300).conversation)).toEqual([]);
  });
});

describe('unread snapshot ordering', () => {
  it('does not overwrite a newer realtime increment with an older snapshot', () => {
    expect(mergeUnreadSnapshot({ first: 3, second: 1 }, { first: 2, third: 4 })).toEqual({
      first: 3,
      second: 1,
      third: 4,
    });
  });
});
