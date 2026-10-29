import { useEffect, type ReactNode } from 'react';
import { NavLink, useLocation, useNavigate } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';
import { useSocket } from '../context/SocketContext';
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
  const { confirm } = useDialogs();
  const { newChat } = useConversations();
  const navigate = useNavigate();
  const { pathname } = useLocation();
  const chatArea = pathname === '/conversations' || /^\/conversations\/[^/]+$/.test(pathname) && pathname !== '/conversations/new-group' || pathname.startsWith('/chat/');
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
    <header className="workspace-topbar"><strong>{appName}</strong><nav aria-label="Main navigation">{[...destinations, { to: '/profile', label: 'Settings', icon: 'settings' as IconName }].map(({to,label,icon}) => <NavLink key={to} to={to} className={({isActive}) => `topbar-link${isActive ? ' is-active' : ''}`}><Icon name={icon}/><span>{label}</span>{counts[label] > 0 && <b>{counts[label]}</b>}</NavLink>)}</nav><details className="account-menu"><summary className="profile-chip" aria-label="Account menu">{user?.username?.slice(0,1).toUpperCase() || 'U'}</summary><div className="row-menu-popover"><NavLink to="/profile">Profile and settings</NavLink><button onClick={() => void signOut()}>Sign out</button></div></details></header>
    <div className="workspace-body">
    {chatArea && <ConversationSidebar />}
    <div className="workspace-main">
      <div className="workspace-content">{children}</div>
    </div></div>
  </div>;
}
