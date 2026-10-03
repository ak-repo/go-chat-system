import type { ReactNode } from 'react';
import { NavLink, useNavigate } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';
import { useSocket } from '../context/SocketContext';
import { appName } from '../config/app';

const destinations = [
  { to: '/conversations', label: 'Chats', icon: '◫' },
  { to: '/friends', label: 'People', icon: '♧' },
  { to: '/notifications', label: 'Alerts', icon: '♧' },
] as const;

export default function AppShell({ children }: { children: ReactNode }) {
  const { user, logout } = useAuth();
  const { notificationUnread } = useSocket();
  const navigate = useNavigate();
  const initials = user?.username?.slice(0, 1).toUpperCase() || 'U';

  return (
    <div className="workspace-shell">
      <aside className="workspace-rail" aria-label="Main navigation">
        <NavLink to="/conversations" className="brand-mark" aria-label={`${appName} chats`}>c</NavLink>
        <nav className="rail-links">
          {destinations.map(({ to, label, icon }) => (
            <NavLink key={to} to={to} className={({ isActive }) => `rail-link${isActive ? ' is-active' : ''}`} aria-label={label} title={label}>
              <span className="rail-icon" aria-hidden="true">{icon}</span>
              <span className="rail-label">{label}</span>
              {label === 'Alerts' && notificationUnread > 0 && <span className="rail-badge">{notificationUnread > 99 ? '99+' : notificationUnread}</span>}
            </NavLink>
          ))}
        </nav>
        <div className="rail-bottom">
          <NavLink to="/profile" className={({ isActive }) => `profile-chip${isActive ? ' is-active' : ''}`} title="Profile" aria-label="Profile">{initials}</NavLink>
          <button className="rail-link rail-logout" title="Sign out" aria-label="Sign out" onClick={() => { void logout(); navigate('/login', { replace: true }); }}>↪</button>
        </div>
      </aside>
      <div className="workspace-main">
        <div className="mobile-topbar"><span className="brand-mark small">c</span><strong>{appName}</strong><span className="mobile-user">{user?.username}</span></div>
        <div className="workspace-content">{children}</div>
        <nav className="mobile-nav" aria-label="Main navigation">
          {destinations.map(({ to, label, icon }) => <NavLink key={to} to={to} className={({ isActive }) => `mobile-nav-link${isActive ? ' is-active' : ''}`}><span aria-hidden="true">{icon}</span>{label}{label === 'Alerts' && notificationUnread > 0 && <i>{notificationUnread}</i>}</NavLink>)}
        </nav>
      </div>
    </div>
  );
}
