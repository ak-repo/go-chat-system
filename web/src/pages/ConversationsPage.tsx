import { useCallback, useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { hideConversation, listConversations, updateConversationPreferences, type Conversation } from '../api/conversations';
import { getPublicUser } from '../api/users';
import { getUnread, markAllRead } from '../api/messages';
import { useAuth } from '../context/AuthContext';
import { useSocket } from '../context/SocketContext';

export default function ConversationsPage() {
  const { user, logout } = useAuth();
  const { sendEvent } = useSocket();
  const navigate = useNavigate();
  const [items, setItems] = useState<Conversation[]>([]);
  const [names, setNames] = useState<Record<string, string>>({});
  const [unread, setUnread] = useState<Record<string, number>>({});
  const [archived, setArchived] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    setLoading(true); setError('');
    try {
      const [response, unreadResponse] = await Promise.all([listConversations(100, 0, archived), getUnread()]);
      if (!response.success || !response.data) throw new Error(response.error || 'Could not load conversations');
      setItems(response.data.conversations ?? []);
      setUnread(unreadResponse.data?.unread ?? {});
      const userIDs = [...new Set((response.data.conversations ?? []).map((c) => c.user_one_id === user?.id ? c.user_two_id : c.user_one_id))];
      const profiles = await Promise.all(userIDs.map(async (id) => [id, await getPublicUser(id)] as const));
      setNames(Object.fromEntries(profiles.map(([id, result]) => [id, result.data?.username ?? id])));
    } catch (cause) { setError(cause instanceof Error ? cause.message : 'Could not load conversations'); }
    finally { setLoading(false); }
  }, [archived, user?.id]);

  // eslint-disable-next-line react-hooks/set-state-in-effect
  useEffect(() => { void load(); }, [load]);
  const update = async (conversation: Conversation, patch: { archived?: boolean; pinned?: boolean; mute_minutes?: number }) => {
    const result = await updateConversationPreferences(conversation.id, patch);
    if (result.success) void load(); else setError(result.error || 'Could not update conversation');
  };
  const hide = async (conversation: Conversation) => {
    if (!window.confirm('Hide this conversation from your list? The other participant keeps their history.')) return;
    const result = await hideConversation(conversation.id);
    if (result.success) void load(); else setError(result.error || 'Could not hide conversation');
  };
  const readAll = async () => { const result = await markAllRead(); if (result.success) { setUnread({}); for (const receipt of result.data?.receipts ?? []) sendEvent('message.read', receipt.sender_id, { message_id: receipt.message_id, conversation_id: receipt.conversation_id }); } else setError(result.error || 'Could not mark conversations read'); };

  if (loading) return <div className="app-shell flex min-h-screen items-center justify-center text-slate-300">Loading conversations...</div>;
  return <div className="app-shell min-h-screen">
    <header className="app-header"><div className="mx-auto flex max-w-5xl items-center justify-between px-4 py-4 sm:px-6"><Link to="/friends" className="font-semibold text-blue-400">← Friends</Link><div className="flex items-center gap-4"><span className="text-sm text-slate-300">{user?.username}</span><button onClick={() => { void logout(); navigate('/login'); }} className="text-sm text-slate-400 hover:text-red-300">Logout</button></div></div></header>
    <main className="mx-auto max-w-5xl px-4 py-8 sm:px-6">
      <div className="mb-6 flex flex-wrap items-center justify-between gap-3"><div><p className="text-sm uppercase tracking-[0.2em] text-blue-400">Messages</p><h1 className="mt-1 text-3xl font-bold text-white">Conversations</h1></div><button onClick={() => void readAll()} className="soft-button px-4 py-2 text-sm">Mark all active read</button></div>
      <div className="mb-5 flex gap-2"><button onClick={() => setArchived(false)} className={`rounded-xl px-4 py-2 text-sm ${!archived ? 'bg-blue-600 text-white' : 'soft-button'}`}>Active</button><button onClick={() => setArchived(true)} className={`rounded-xl px-4 py-2 text-sm ${archived ? 'bg-blue-600 text-white' : 'soft-button'}`}>Archived</button></div>
      {error && <div className="mb-4 rounded-xl border border-red-900/60 bg-red-950/40 p-3 text-red-300">{error}</div>}
      <div className="app-card overflow-hidden">{items.length === 0 ? <div className="p-12 text-center text-slate-400">No {archived ? 'archived ' : ''}conversations yet.</div> : <ul className="divide-y divide-[#25364d]">{items.map((item) => { const peerID = item.user_one_id === user?.id ? item.user_two_id : item.user_one_id; return <li key={item.id} className="flex flex-wrap items-center justify-between gap-3 p-4"><button onClick={() => navigate(`/chat/${peerID}`)} className="min-w-0 flex-1 text-left"><span className="font-semibold text-white">{names[peerID] ?? peerID}</span>{unread[item.id] > 0 && <span className="ml-2 rounded-full bg-blue-600 px-2 py-0.5 text-xs text-white">{unread[item.id]}</span>}<span className="ml-2 text-xs text-slate-400">{item.pinned ? 'Pinned' : ''}{item.muted ? ' · Muted' : ''}</span></button><div className="flex gap-2"><button onClick={() => void update(item, { pinned: !item.pinned })} className="soft-button px-3 py-2 text-xs">{item.pinned ? 'Unpin' : 'Pin'}</button><button onClick={() => void update(item, { mute_minutes: item.muted ? 0 : 1440 })} className="soft-button px-3 py-2 text-xs">{item.muted ? 'Unmute' : 'Mute'}</button><button onClick={() => void update(item, { archived: !item.archived })} className="soft-button px-3 py-2 text-xs">{item.archived ? 'Restore' : 'Archive'}</button><button onClick={() => void hide(item)} className="soft-button px-3 py-2 text-xs text-red-300">Hide</button></div></li>; })}</ul>}</div>
    </main>
  </div>;
}
