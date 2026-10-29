import NotificationPreferencesPanel from '../components/NotificationPreferencesPanel';
import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { listNotifications, markAllNotificationsRead, markNotificationRead, type Notification } from '../api/notifications';
import { useSocket } from '../context/SocketContext';

export default function NotificationsPage() {
  const { onEvent } = useSocket();
  const [items, setItems] = useState<Notification[]>([]);
  const [error, setError] = useState('');
  const load = async () => { try { const notifications = await listNotifications(); setItems(notifications.data?.notifications ?? []); } catch { setError('Could not load notifications'); } };
  useEffect(() => { const timer = window.setTimeout(() => void load(), 0); return () => window.clearTimeout(timer); }, []);
  useEffect(() => onEvent('notification.created', (event) => { const notification = event.data as Notification; setItems((current) => current.some((item) => item.id === notification.id) ? current : [notification, ...current]); }), [onEvent]);
  const read = async (notification: Notification) => { if (!notification.read_at) { const result = await markNotificationRead(notification.id); if (result.data) setItems((current) => current.map((item) => item.id === notification.id ? result.data! : item)); } };
  const readAll = async () => { const result = await markAllNotificationsRead(); if (result.success) setItems((current) => current.map((item) => ({ ...item, read_at: item.read_at ?? new Date().toISOString() }))); };

  return <main className="app-shell min-h-screen px-4 py-8"><div className="mx-auto max-w-3xl"><div className="mb-6 flex items-center justify-between"><div><Link to="/conversations" className="text-sm text-accent">Back to conversations</Link><h1 className="mt-2 text-2xl font-bold">Notifications</h1></div><button className="soft-button px-3 py-2 text-sm" onClick={() => void readAll()}>Mark all read</button></div>{error && <p className="text-danger">{error}</p>}<NotificationPreferencesPanel /><div className="space-y-3">{items.map((item) => <Link key={item.id} to={`/conversations/${item.conversation_id}`} onClick={() => void read(item)} className={`block rounded-2xl border p-4 ${item.read_at ? 'border-theme bg-raised' : 'border-accent bg-accent-soft'}`}><div className="flex justify-between gap-3"><p className="font-medium">New {item.type}</p><time className="text-xs text-secondary">{new Date(item.created_at).toLocaleString()}</time></div><p className="mt-1 truncate text-sm text-secondary">{item.payload.content ?? 'Open conversation'}</p></Link>)}{items.length === 0 && <p className="py-12 text-center text-secondary">No notifications yet.</p>}</div></div></main>;
}
