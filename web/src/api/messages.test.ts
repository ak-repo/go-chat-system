import { describe, expect, it } from 'vitest';
import {
  mergeMessagesChronologically,
  mergeMessagePages,
  mergeMessageStatus,
  optimisticReaction,
  sortMessagesChronologically,
  type Message,
} from './messages';

function message(id: string, createdAt: string, overrides: Partial<Message> = {}): Message {
  return {
    id,
    sender_id: 'sender',
    receiver_id: 'receiver',
    content: `message ${id}`,
    is_group: false,
    created_at: createdAt,
    modified_at: createdAt,
    ...overrides,
  };
}

describe('message utilities', () => {
  it('only advances delivery status', () => {
    expect(mergeMessageStatus('sent', 'delivered')).toBe('delivered');
    expect(mergeMessageStatus('read', 'delivered')).toBe('read');
    expect(mergeMessageStatus('failed', 'sent')).toBe('sent');
    expect(mergeMessageStatus('sent', undefined)).toBe('sent');
  });

  it('sorts chronologically without mutating the input', () => {
    const later = message('later', '2026-09-27T12:01:00Z');
    const earlier = message('earlier', '2026-09-27T12:00:00Z');
    const input = [later, earlier];

    expect(sortMessagesChronologically(input).map(({ id }) => id)).toEqual(['earlier', 'later']);
    expect(input).toEqual([later, earlier]);
  });

  it('reconciles an optimistic message by client id and preserves its advanced status', () => {
    const optimistic = message('temporary', '2026-09-27T12:01:00Z', {
      client_message_id: 'client-1',
      status: 'delivered',
    });
    const earlier = message('earlier', '2026-09-27T12:00:00Z');
    const confirmed = message('server-1', '2026-09-27T12:01:00Z', {
      client_message_id: 'client-1',
      content: 'confirmed content',
      status: 'sent',
    });

    const result = mergeMessagesChronologically([optimistic, earlier], confirmed);

    expect(result).toHaveLength(2);
    expect(result.map(({ id }) => id)).toEqual(['earlier', 'server-1']);
    expect(result[1]).toMatchObject({ content: 'confirmed content', status: 'delivered' });
  });

  it('optimistically adds and removes a reaction idempotently', () => {
    const added = optimisticReaction([], '👍', true);
    expect(optimisticReaction(added, '👍', true)).toEqual(added);
    expect(optimisticReaction(added, '👍', false)).toEqual([]);
  });

  it('deduplicates overlapping pagination results', () => {
    const existing = message('two', '2026-09-27T12:01:00Z', { content: 'current' });
    const older = message('one', '2026-09-27T12:00:00Z');
    const overlap = message('two', '2026-09-27T12:01:00Z', { content: 'canonical' });

    const result = mergeMessagePages([existing], [older, overlap]);

    expect(result.map(({ id }) => id)).toEqual(['one', 'two']);
    expect(result[1].content).toBe('canonical');
  });
});
