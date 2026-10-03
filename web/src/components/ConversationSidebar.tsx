import { useEffect, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { conversationInitials, listConversations, type Conversation } from '../api/conversations';
import { getPublicUser } from '../api/users';
import { useAuth } from '../context/AuthContext';
import { useSocket } from '../context/SocketContext';

export default function ConversationSidebar() {
  const { conversationId } = useParams();
  const { user } = useAuth();
  const { unread } = useSocket();
  const navigate = useNavigate();
  const [items, setItems] = useState<Conversation[]>([]);
  const [names, setNames] = useState<Record<string, string>>({});
  const [query, setQuery] = useState('');
  useEffect(() => {
    let active = true;
    void listConversations(100, 0, false).then(async (result) => {
      const conversations = result.data?.conversations ?? [];
      const ids = [...new Set(conversations.filter((item) => item.kind === 'direct').map((item) => item.user_one_id === user?.id ? item.user_two_id : item.user_one_id))];
      const profiles = await Promise.all(ids.map(async (id) => [id, await getPublicUser(id)] as const));
      if (active) { setItems(conversations); setNames(Object.fromEntries(profiles.map(([id, profile]) => [id, profile.data?.username ?? id]))); }
    });
    return () => { active = false; };
  }, [user?.id]);
  const filtered = items.filter((item) => {
    const peer = item.kind === 'direct' ? (item.user_one_id === user?.id ? item.user_two_id : item.user_one_id) : '';
    return (item.kind === 'group' ? item.name : names[peer] ?? '').toLowerCase().includes(query.toLowerCase());
  }).sort((a, b) => Number(b.pinned) - Number(a.pinned));
  return <aside className="chat-sidebar">
    <div className="chat-sidebar-heading"><div><span className="eyebrow">MESSAGES</span><h1>Chats</h1></div><Link to="/conversations/new-group" className="round-action" aria-label="Create group" title="Create group">＋</Link></div>
    <label className="search-box sidebar-search"><span aria-hidden="true">⌕</span><input value={query} onChange={(event) => setQuery(event.target.value)} aria-label="Search chats" placeholder="Search or start a chat" /></label>
    <div className="sidebar-tabs"><button className="selected">All chats</button><button onClick={() => navigate('/conversations')}>View archived</button></div>
    <ul className="sidebar-chat-list">{filtered.map((item) => {
      const peer = item.kind === 'direct' ? (item.user_one_id === user?.id ? item.user_two_id : item.user_one_id) : '';
      const title = item.kind === 'group' ? item.name : names[peer] ?? peer;
      return <li key={item.id}><button onClick={() => navigate(`/conversations/${item.id}`)} className={`sidebar-chat-row${item.id === conversationId ? ' selected' : ''}`}><span className={`conversation-avatar small${item.kind === 'group' ? ' group-avatar' : ''}`}>{conversationInitials(title)}</span><span className="sidebar-chat-copy"><strong>{title}</strong><small>{item.kind === 'group' ? `${item.member_count} members` : 'Direct conversation'}</small></span>{(unread[item.id] ?? 0) > 0 && <b className="sidebar-unread">{unread[item.id]}</b>}</button></li>;
    })}</ul>
    <div className="sidebar-footer"><Link to="/friends">＋ Find people</Link><Link to="/conversations">All conversations</Link></div>
  </aside>;
}
