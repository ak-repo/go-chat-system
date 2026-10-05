import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Outlet, Route, Routes, useLocation } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ConversationProvider } from '../context/ConversationContext';
import { DialogProvider } from '../context/DialogContext';
import { ThemeProvider } from '../context/ThemeContext';
import { createOrGetConversation, listConversations, type Conversation } from '../api/conversations';
import AppShell from './AppShell';
vi.mock('../context/AuthContext', () => ({ useAuth: () => ({ user: { id: 'me', username: 'Me' }, logout: vi.fn() }) }));
vi.mock('../context/SocketContext', () => ({ useSocket: () => ({ isConnected: true, unread: { direct: 2 }, presence: {}, notificationUnread: 0, sendEvent: vi.fn(), clearAllUnread: vi.fn() }) }));
vi.mock('../api/websocket', () => ({ default: { on: () => () => undefined } }));
vi.mock('../api/conversations', async (original) => ({ ...await original<typeof import('../api/conversations')>(), listConversations: vi.fn(), createOrGetConversation: vi.fn() }));
vi.mock('../api/users', () => ({ getPublicUser: vi.fn(async () => ({ success: true, data: { username: 'Alice' } })) }));
vi.mock('../api/friends', () => ({ listFriends: vi.fn(async () => ({ success: true, data: { friends: [{ FriendID: 'bob', FriendName: 'Bob' }] } })) }));
const base = { created_at: '2026-10-04T00:00:00Z', modified_at: '2026-10-04T00:00:00Z', archived: false, pinned: false, muted: false };
const direct: Conversation = { ...base, id: 'direct', kind: 'direct', user_one_id: 'me', user_two_id: 'alice' };
const group: Conversation = { ...base, id: 'group', kind: 'group', name: 'Team', member_count: 3, creator_id: 'me', current_role: 'owner', capabilities: { edit_group: true, add_members: true, remove_members: true, manage_roles: true, manage_invites: true } };
function Pane() { const { pathname } = useLocation(); return <div data-testid="pane">{pathname}</div>; }
function mount() {
  render(<MemoryRouter initialEntries={['/conversations']}><ThemeProvider><DialogProvider><Routes><Route element={<ConversationProvider><AppShell><Outlet /></AppShell></ConversationProvider>}><Route path="/conversations" element={<Pane />} /><Route path="/conversations/:id" element={<Pane />} /></Route></Routes></DialogProvider></ThemeProvider></MemoryRouter>);
}
beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(listConversations).mockImplementation(async (_limit, _offset, archived) => ({ success: true, data: { conversations: archived ? [{ ...group, id: 'old', name: 'Archived team', archived: true }] : [direct, group], limit: 100, offset: 0 } }));
  vi.mocked(createOrGetConversation).mockResolvedValue({ success: true, data: { ...direct, id: 'bob-chat' } });
});
afterEach(cleanup);
describe('shared conversation workspace', () => {
  it('filters unread/groups and loads archived chats', async () => {
    mount(); await screen.findByText('Alice');
    fireEvent.click(screen.getByRole('button', { name: 'Unread' }));
    expect(screen.getByText('Alice')).toBeInTheDocument();
    expect(screen.queryByText('Team')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Groups' }));
    expect(screen.getByText('Team')).toBeInTheDocument();
    expect(screen.queryByText('Alice')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Archived' }));
    await screen.findByText('Archived team');
    expect(listConversations).toHaveBeenLastCalledWith(100, 0, true);
  });
  it('keeps search and list state mounted when opening a conversation', async () => {
    mount(); await screen.findByText('Alice');
    fireEvent.change(screen.getByLabelText('Search chats'), { target: { value: 'Ali' } });
    fireEvent.click(screen.getByText('Alice'));
    expect(screen.getByTestId('pane')).toHaveTextContent('/conversations/direct');
    expect(screen.getByLabelText('Search chats')).toHaveValue('Ali');
    expect(listConversations).toHaveBeenCalledOnce();
    expect(screen.getByText('Alice').closest('button')).toHaveAttribute('aria-current', 'page');
  });
  it('opens contacts while keeping the conversation and uses existing create/get', async () => {
    mount(); await screen.findByText('Alice');
    fireEvent.click(screen.getByText('Alice'));
    fireEvent.click(screen.getByRole('button', { name: 'New chat' }));
    expect(screen.getByTestId('pane')).toHaveTextContent('/conversations/direct');
    await screen.findByText('Bob'); fireEvent.click(screen.getByText('Bob'));
    await waitFor(() => expect(screen.getByTestId('pane')).toHaveTextContent('/conversations/bob-chat'));
    expect(createOrGetConversation).toHaveBeenCalledWith('bob');
    expect(screen.queryByLabelText('Search contacts')).not.toBeInTheDocument();
  });
  it('loads another page without losing existing chats', async () => {
    const page = Array.from({ length: 100 }, (_, index) => ({ ...group, id: `group-${index}`, name: `Group ${index}` }));
    vi.mocked(listConversations).mockImplementation(async (_limit, offset) => ({ success: true, data: { conversations: offset ? [{ ...group, id: 'extra', name: 'Next page' }] : page, limit: 100, offset: offset ?? 0 } }));
    mount(); await screen.findByText('Group 0');
    fireEvent.click(screen.getByRole('button', { name: 'Load more chats' }));
    await screen.findByText('Next page');
    expect(screen.getByText('Group 0')).toBeInTheDocument();
    expect(listConversations).toHaveBeenLastCalledWith(100, 100, false);
    expect(screen.queryByRole('button', { name: 'Load more chats' })).not.toBeInTheDocument();
  });
});
