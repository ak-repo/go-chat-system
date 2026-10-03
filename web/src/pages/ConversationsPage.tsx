import { useCallback, useEffect, useMemo, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { conversationInitials, hideConversation, listConversations, updateConversationPreferences, type Conversation } from '../api/conversations';
import { getPublicUser } from '../api/users';
import { markAllRead } from '../api/messages';
import { useAuth } from '../context/AuthContext';
import { useSocket } from '../context/SocketContext';

export default function ConversationsPage() {
  const { user } = useAuth();
  const { sendEvent, unread, clearAllUnread } = useSocket();
  const navigate = useNavigate();
  const [items, setItems] = useState<Conversation[]>([]);
  const [names, setNames] = useState<Record<string, string>>({});
  const [archived, setArchived] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [query, setQuery] = useState('');

  const load = useCallback(async () => {
    setLoading(true); setError('');
    try {
      const response = await listConversations(100, 0, archived);
      if (!response.success || !response.data) throw new Error(response.error || 'Could not load conversations');
      const conversations = response.data.conversations ?? [];
      setItems(conversations);
      const ids = [...new Set(conversations.filter((c) => c.kind === 'direct').map((c) => c.user_one_id === user?.id ? c.user_two_id : c.user_one_id))];
      const profiles = await Promise.all(ids.map(async (id) => [id, await getPublicUser(id)] as const));
      setNames(Object.fromEntries(profiles.map(([id, result]) => [id, result.data?.username ?? id])));
    } catch (cause) { setError(cause instanceof Error ? cause.message : 'Could not load conversations'); }
    finally { setLoading(false); }
  }, [archived, user?.id]);

  // eslint-disable-next-line react-hooks/set-state-in-effect
  useEffect(() => { void load(); }, [load]);
  const visible = useMemo(() => items.filter((item) => {
    const peer = item.kind === 'direct' ? (item.user_one_id === user?.id ? item.user_two_id : item.user_one_id) : '';
    return (item.kind === 'group' ? item.name : names[peer] ?? '').toLowerCase().includes(query.trim().toLowerCase());
  }).sort((a, b) => Number(b.pinned) - Number(a.pinned)), [items, names, query, user?.id]);
  const update = async (item: Conversation, patch: { archived?: boolean; pinned?: boolean; mute_minutes?: number }) => { const result = await updateConversationPreferences(item.id, patch); if (result.success) void load(); else setError(result.error || 'Could not update conversation'); };
  const hide = async (item: Conversation) => { if (!window.confirm('Hide this conversation from your list?')) return; const result = await hideConversation(item.id); if (result.success) void load(); else setError(result.error || 'Could not hide conversation'); };
  const readAll = async () => { const result = await markAllRead(); if (result.success) { clearAllUnread(); for (const receipt of result.data?.receipts ?? []) sendEvent('message.read', receipt.sender_id, { message_id: receipt.message_id, conversation_id: receipt.conversation_id }); } };

  return <main className="conversation-home">
    <header className="list-heading"><div><span className="eyebrow">YOUR SPACE</span><h1>Chats</h1><p>Pick up where you left off.</p></div><div className="list-heading-actions"><Link to="/conversations/new-group" className="primary-button new-group-button"><span aria-hidden="true">＋</span><span className="desktop-action-label">New group</span></Link><button onClick={() => void readAll()} className="soft-button mark-read-button">Mark all read</button></div></header>
    <section className="conversation-panel">
      <div className="conversation-tools"><label className="search-box"><span aria-hidden="true">⌕</span><input aria-label="Search conversations" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search conversations" /></label>
        <div className="list-filters" role="tablist" aria-label="Conversation views"><button role="tab" aria-selected={!archived} className={!archived ? 'selected' : ''} onClick={() => setArchived(false)}>All chats</button><button role="tab" aria-selected={archived} className={archived ? 'selected' : ''} onClick={() => setArchived(true)}>Archived</button></div>
      </div>
      {error && <div className="inline-alert" role="alert">{error}</div>}
      {loading ? <div className="list-placeholder">Loading your chats…</div> : visible.length === 0 ? <div className="list-placeholder"><div className="empty-orbit">◫</div><h2>{query ? 'No matches found' : archived ? 'Nothing archived' : 'Your chats start here'}</h2><p>{query ? 'Try another name.' : 'Find a friend or create a group to start a conversation.'}</p>{!query && !archived && <Link to="/friends" className="primary-button empty-action">Find people</Link>}</div> : <ul className="conversation-list">{visible.map((item) => {
        const peer = item.kind === 'direct' ? (item.user_one_id === user?.id ? item.user_two_id : item.user_one_id) : '';
        const title = item.kind === 'group' ? item.name : names[peer] ?? peer;
        const count = unread[item.id] ?? 0;
        return <li key={item.id} className="conversation-row-wrap">
          <button className="conversation-row" onClick={() => navigate(`/conversations/${item.id}`)}>
            <span className={`conversation-avatar${item.kind === 'group' ? ' group-avatar' : ''}`}>{conversationInitials(title)}{item.kind === 'direct' && <i className="avatar-presence" />}</span>
            <span className="conversation-copy"><span className="conversation-title-line"><strong>{title}</strong><time>{new Date(item.modified_at).toLocaleDateString([], { month: 'short', day: 'numeric' })}</time></span><span className="conversation-preview">{item.kind === 'group' ? `${item.member_count} members` : 'Direct conversation'}{item.muted && <span className="row-tag">Muted</span>}</span></span>
            <span className="conversation-indicators">{item.pinned && <span title="Pinned" aria-label="Pinned">⌖</span>}{count > 0 && <b>{count > 99 ? '99+' : count}</b>}</span>
          </button>
          <details className="row-menu"><summary aria-label={`Actions for ${title}`}>•••</summary><div className="row-menu-popover"><button onClick={() => void update(item, { pinned: !item.pinned })}>{item.pinned ? 'Unpin chat' : 'Pin chat'}</button><button onClick={() => void update(item, { mute_minutes: item.muted ? 0 : 1440 })}>{item.muted ? 'Unmute' : 'Mute for 24 hours'}</button><button onClick={() => void update(item, { archived: !item.archived })}>{item.archived ? 'Unarchive' : 'Archive'}</button><button className="danger-action" onClick={() => void hide(item)}>Hide chat</button></div></details>
        </li>;
      })}</ul>}
    </section>
  </main>;
}
