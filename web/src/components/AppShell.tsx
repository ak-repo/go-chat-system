import { useEffect, type ReactNode } from 'react';
import { NavLink, useLocation, useNavigate } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';
import { useSocket } from '../context/SocketContext';
import { useTheme } from '../context/ThemeContext';
import { useDialogs } from '../context/DialogContext';
import { useConversations } from '../context/ConversationContext';
import { appName } from '../config/app';
import Icon, { type IconName } from './Icon';
import ConversationSidebar from './ConversationSidebar';
const destinations: { to: string; label: string; icon: IconName }[] = [
  { to: '/conversations', label: 'Chats', icon: 'chats' },
  { to: '/friends', label: 'People', icon: 'people' },
  { to: '/notifications', label: 'Notifications', icon: 'bell' },
];
export default function AppShell({ children }: { children: ReactNode }) {
  const { user, logout } = useAuth();
  const { notificationUnread, unread } = useSocket();
  const { theme, setTheme } = useTheme();
  const { confirm } = useDialogs();
  const { newChat } = useConversations();
  const navigate = useNavigate();
  const { pathname } = useLocation();
  const chatArea = pathname.startsWith('/conversations') || pathname.startsWith('/chat/');
  const listPage = pathname === '/conversations';
  const counts: Record<string, number> = { Chats: Object.values(unread).reduce((sum, value) => sum + value, 0), Notifications: notificationUnread };
  useEffect(() => {
    const closeMenus = (event: MouseEvent | KeyboardEvent) => {
      if (event instanceof KeyboardEvent && event.key !== 'Escape') return;
      const target = event.target instanceof Element ? event.target : null;
      document.querySelectorAll<HTMLDetailsElement>('details[open]').forEach((menu) => {
        if (event instanceof KeyboardEvent || !target || !menu.contains(target) || target.closest('button, a')) menu.open = false;
      });
    };
    document.addEventListener('click', closeMenus);
    document.addEventListener('keydown', closeMenus);
    return () => { document.removeEventListener('click', closeMenus); document.removeEventListener('keydown', closeMenus); };
  }, []);
  async function signOut() { if (await confirm('Sign out of your account?')) { await logout(); navigate('/login', { replace: true }); } }
  return <div className={`workspace-shell${chatArea ? ' has-chats' : ''}${listPage ? ' shows-list' : ''}${newChat ? ' shows-picker' : ''}`}>
    <aside className="workspace-rail" aria-label="Main navigation">
      <nav className="rail-links">{destinations.map(({ to, label, icon }) => <NavLink key={to} to={to} className={({ isActive }) => `rail-link${isActive ? ' is-active' : ''}`} aria-label={label} title={label}><Icon name={icon} />{counts[label] > 0 && <span className="rail-badge">{counts[label] > 99 ? '99+' : counts[label]}</span>}</NavLink>)}</nav>
      <div className="rail-bottom">
        <button className="rail-link" aria-label={`Switch to ${theme === 'light' ? 'dark' : 'light'} theme`} title="Switch theme" onClick={() => setTheme(theme === 'light' ? 'dark' : 'light')}><Icon name={theme === 'light' ? 'moon' : 'sun'} /></button>
        <NavLink to="/profile" className="rail-link" aria-label="Settings" title="Settings"><Icon name="settings" /></NavLink>
        <NavLink to="/profile" className="profile-chip" title="Profile" aria-label="Profile">{user?.username?.slice(0, 1).toUpperCase() || 'U'}</NavLink>
        <button className="rail-link" title="Sign out" aria-label="Sign out" onClick={() => void signOut()}><Icon name="logout" /></button>
      </div>
    </aside>
    {chatArea && <ConversationSidebar />}
    <div className="workspace-main">
      {!chatArea && <header className="mobile-topbar"><strong>{appName}</strong><NavLink to="/profile" aria-label="Profile"><Icon name="settings" /></NavLink></header>}
      <div className="workspace-content">{children}</div>
      <nav className="mobile-nav" aria-label="Mobile navigation">{[...destinations, { to: '/profile', label: 'Profile', icon: 'settings' as const }].map(({ to, label, icon }) => <NavLink key={to} to={to} className={({ isActive }) => `mobile-nav-link${isActive ? ' is-active' : ''}`}><Icon name={icon} />{label}{counts[label] > 0 && <span className="rail-badge">{counts[label] > 99 ? '99+' : counts[label]}</span>}</NavLink>)}</nav>
    </div>
  </div>;
}
