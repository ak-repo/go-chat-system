import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import AppShell from './AppShell';
vi.mock('../context/AuthContext', () => ({ useAuth: () => ({ user: { username: 'Alice' }, logout: vi.fn() }) }));
vi.mock('../context/SocketContext', () => ({ useSocket: () => ({ unread: {}, notificationUnread: 0 }) }));
vi.mock('../context/DialogContext', () => ({ useDialogs: () => ({ confirm: vi.fn() }) }));
vi.mock('../context/ConversationContext', () => ({ useConversations: () => ({ newChat: false }) }));
vi.mock('./ConversationSidebar', () => ({ default: () => <aside aria-label="Conversations">Inbox</aside> }));
afterEach(cleanup);
describe('workspace layout', () => {
 it('renders a single sidebar on a conversation route', () => {
  render(<MemoryRouter initialEntries={['/conversations/123']}><AppShell>Chat</AppShell></MemoryRouter>);
  expect(screen.getAllByRole('complementary')).toHaveLength(1);
  expect(screen.getByRole('navigation', {name:'Main navigation'})).toBeInTheDocument();
 });
 it.each(['/profile','/conversations/new-group','/conversations/123/settings'])('uses the full content area on %s', path => {
  render(<MemoryRouter initialEntries={[path]}><AppShell>Page content</AppShell></MemoryRouter>);
  expect(screen.queryByRole('complementary')).not.toBeInTheDocument();
  expect(screen.getByText('Page content')).toBeInTheDocument();
 });
});
