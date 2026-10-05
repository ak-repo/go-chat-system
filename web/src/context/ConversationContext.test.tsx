import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ConversationProvider, useConversations } from './ConversationContext';
import { listConversations } from '../api/conversations';
import type { Message } from '../api/messages';
const { handlers } = vi.hoisted(() => ({ handlers: new Map<string, Set<(event: { data: unknown }) => void>>() }));
vi.mock('./AuthContext', () => ({ useAuth: () => ({ user: { id: 'me' } }) }));
vi.mock('./SocketContext', () => ({ useSocket: () => ({ isConnected: true }) }));
vi.mock('../api/conversations', () => ({ listConversations: vi.fn() }));
vi.mock('../api/users', () => ({ getPublicUser: vi.fn() }));
vi.mock('../api/websocket', () => ({ default: { on: (name: string, handler: (event: { data: unknown }) => void) => { const listeners = handlers.get(name) ?? new Set(); listeners.add(handler); handlers.set(name, listeners); return () => listeners.delete(handler); } } }));
function CacheHarness() {
  const { previews, recordMessages, error, refresh } = useConversations();
  const latest = { id: 'new', sender_id: 'alice', receiver_id: 'me', content: 'Latest message', created_at: '2026-10-04T12:00:00Z', modified_at: '2026-10-04T12:00:00Z', is_group: false } satisfies Message;
  return <><output data-testid="previews">{JSON.stringify(previews)}</output><output data-testid="error">{error}</output><button onClick={() => recordMessages('chat', [latest])}>Cache latest</button><button onClick={() => recordMessages('chat', [{ ...latest, id: 'old', content: 'Older history', created_at: '2026-10-01T12:00:00Z' }])}>Cache older</button><button onClick={refresh}>Refresh</button></>;
}
function emit(name: string, data: unknown) { act(() => handlers.get(name)?.forEach((handler) => handler({ data }))); }
beforeEach(() => { vi.clearAllMocks(); handlers.clear(); vi.mocked(listConversations).mockResolvedValue({ success: true, data: { conversations: [], limit: 100, offset: 0 } }); });
afterEach(cleanup);
describe('conversation previews', () => {
  it('keeps latest cached content when older history arrives', async () => {
    render(<ConversationProvider><CacheHarness /></ConversationProvider>);
    await waitFor(() => expect(listConversations).toHaveBeenCalled());
    fireEvent.click(screen.getByText('Cache latest'));
    fireEvent.click(screen.getByText('Cache older'));
    expect(screen.getByTestId('previews')).toHaveTextContent('Latest message');
    expect(screen.getByTestId('previews')).not.toHaveTextContent('Older history');
  });
  it('handles reply wire IDs without inventing a server timestamp, then edits/deletes', () => {
    render(<ConversationProvider><CacheHarness /></ConversationProvider>);
    emit('message.replied', { conversation_id: 'chat', message_id: 'original', server_id: 'reply', content: 'Reply content' });
    const preview = JSON.parse(screen.getByTestId('previews').textContent!).chat;
    expect(preview).toEqual({ id: 'reply', content: 'Reply content' });
    emit('message.edited', { conversation_id: 'chat', message_id: 'reply', content: 'Edited reply' });
    expect(screen.getByTestId('previews')).toHaveTextContent('Edited reply');
    emit('message.deleted', { conversation_id: 'chat', message_id: 'reply' });
    expect(screen.getByTestId('previews')).toHaveTextContent('Message deleted');
  });
  it('ignores out-of-order live messages', () => {
    render(<ConversationProvider><CacheHarness /></ConversationProvider>);
    fireEvent.click(screen.getByText('Cache latest'));
    emit('message', { conversation_id: 'chat', message_id: 'delayed', content: 'Delayed message', timestamp: '2026-10-01T12:00:00Z' });
    expect(screen.getByTestId('previews')).toHaveTextContent('Latest message');
    expect(screen.getByTestId('previews')).not.toHaveTextContent('Delayed message');
  });
  it('shows request failures and can recover through refresh', async () => {
    vi.mocked(listConversations).mockRejectedValueOnce(new Error('Connection failed'));
    render(<ConversationProvider><CacheHarness /></ConversationProvider>);
    await waitFor(() => expect(screen.getByTestId('error')).toHaveTextContent('Connection failed'));
    fireEvent.click(screen.getByText('Refresh'));
    await waitFor(() => expect(screen.getByTestId('error')).toBeEmptyDOMElement());
  });
});
