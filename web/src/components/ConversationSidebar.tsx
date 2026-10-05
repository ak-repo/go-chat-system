import { useState } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { conversationInitials, hideConversation, updateConversationPreferences, type Conversation } from '../api/conversations';
import { markAllRead } from '../api/messages';
import { useAuth } from '../context/AuthContext';
import { useSocket } from '../context/SocketContext';
import { useConversations } from '../context/ConversationContext';
import { useDialogs } from '../context/DialogContext';
import { appName } from '../config/app';
import Icon from './Icon';
import NewChatPanel from './NewChatPanel';
export default function ConversationSidebar() {
  const { pathname } = useLocation();
  const conversationId = /^\/conversations\/([^/]+)$/.exec(pathname)?.[1];
  const { user } = useAuth();
  const { unread, presence, sendEvent, clearAllUnread } = useSocket();
  const { items, names, previews, archived, setArchived, loading, error, refresh, hasMore, loadMore, newChat, setNewChat } = useConversations();
  const { confirm } = useDialogs();
  const navigate = useNavigate();
  const [query, setQuery] = useState('');
  const [filter, setFilter] = useState<'all' | 'unread' | 'groups'>('all');
  const [actionError, setActionError] = useState('');
  const [busy, setBusy] = useState(false);
  const peer = (item: Conversation) => item.kind === 'direct' ? item.user_one_id === user?.id ? item.user_two_id : item.user_one_id : '';
  const title = (item: Conversation) => item.kind === 'group' ? item.name : names[peer(item)] ?? 'Loading contact…';
  const visible = items.filter((item) => title(item).toLowerCase().includes(query.trim().toLowerCase()) && (filter !== 'unread' || (unread[item.id] ?? 0) > 0) && (filter !== 'groups' || item.kind === 'group')).sort((a, b) => Number(b.pinned) - Number(a.pinned));
  async function act(action: () => Promise<void>) {
    if (busy) return;
    setBusy(true); setActionError('');
    try { await action(); } catch (cause) { setActionError(cause instanceof Error ? cause.message : 'Could not complete action'); }
    finally { setBusy(false); }
  }
  function update(item: Conversation, patch: { pinned?: boolean; archived?: boolean; mute_minutes?: number }) {
    void act(async () => { const result = await updateConversationPreferences(item.id, patch); if (!result.success) throw new Error(result.error || 'Could not update chat'); refresh(); });
  }
  async function hide(item: Conversation) {
    if (!await confirm('Hide this conversation from your list?')) return;
    void act(async () => { const result = await hideConversation(item.id); if (!result.success) throw new Error(result.error || 'Could not hide chat'); if (item.id === conversationId) navigate('/conversations'); refresh(); });
  }
  function readAll() {
    void act(async () => { const result = await markAllRead(); if (!result.success) throw new Error(result.error || 'Could not mark chats read'); clearAllUnread(); for (const receipt of result.data?.receipts ?? []) sendEvent('message.read', receipt.sender_id, { message_id: receipt.message_id, conversation_id: receipt.conversation_id }); });
  }
  return <aside className="chat-sidebar" aria-label={newChat ? 'New chat' : 'Conversations'}>{newChat ? <NewChatPanel /> : <>
    <header className="chat-sidebar-heading">{archived && <button className="icon-button" aria-label="Back to all chats" onClick={() => setArchived(false)}><Icon name="back" /></button>}<h1>{archived ? 'Archived' : appName}</h1><div className="sidebar-heading-actions"><details className="row-menu"><summary className="icon-button" aria-label="Chat list actions"><Icon name="more" /></summary><div className="row-menu-popover"><button disabled={busy} onClick={readAll}>Mark all as read</button><button onClick={refresh}>Refresh chats</button><button onClick={() => navigate('/conversations/new-group')}>New group</button></div></details><button className="new-chat-button" aria-label="New chat" title="New chat" onClick={() => setNewChat(true)}><Icon name="newChat" /></button></div></header>
    <label className="search-box sidebar-search"><Icon name="search" width={20} height={20} /><input value={query} onChange={(event) => setQuery(event.target.value)} aria-label="Search chats" placeholder="Search chats by name" /></label>
    <div className="sidebar-tabs" role="group" aria-label="Chat filters">{(['all', 'unread', 'groups'] as const).map((value) => <button key={value} aria-pressed={filter === value} className={filter === value ? 'selected' : ''} onClick={() => setFilter(value)}>{value === 'all' ? 'All' : value === 'unread' ? 'Unread' : 'Groups'}</button>)}</div>
    {(error || actionError) && <div className="inline-alert" role="alert">{error || actionError}<button className="soft-button" onClick={refresh}>Retry</button></div>}
    <div className="sidebar-chat-list scrollbar-thin">
      {!archived && <button className="archive-entry" onClick={() => setArchived(true)}><Icon name="archive" /><span>Archived</span></button>}
      {loading && items.length === 0 ? <div className="list-placeholder" role="status">Loading chats…</div> : visible.length === 0 ? <div className="list-placeholder"><Icon name="chats" width={40} height={40} /><h2>{query ? 'No matches found' : archived ? 'Nothing archived' : 'No chats here yet'}</h2><p>{query ? 'Try another name.' : 'Start a conversation with a friend.'}</p>{!archived && <button className="primary-button empty-action" onClick={() => setNewChat(true)}>New chat</button>}</div> : <ul className="conversation-list">{visible.map((item) => {
        const count = unread[item.id] ?? 0;
        const preview = previews[item.id];
        const label = title(item);
        return <li key={item.id} className={`conversation-row-wrap${item.id === conversationId ? ' selected' : ''}`}>
          <button onClick={() => navigate(`/conversations/${item.id}`)} aria-current={item.id === conversationId ? 'page' : undefined} className="sidebar-chat-row">
            <span className="conversation-avatar">{conversationInitials(label)}{item.kind === 'direct' && presence[peer(item)]?.online && <i className="avatar-presence" />}</span>
            <span className="sidebar-chat-copy"><span className="conversation-title-line"><strong>{label}</strong>{preview?.timestamp && <time className={count ? 'unread-time' : ''} dateTime={preview.timestamp}>{new Date(preview.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}</time>}</span><span className="conversation-preview"><span>{preview?.content || (item.kind === 'group' ? `${item.member_count} members` : 'Direct conversation')}</span><span className="conversation-indicators">{item.muted && <Icon name="muted" width={16} height={16} />}{item.pinned && <Icon name="pin" width={16} height={16} />}{count > 0 && <b className="sidebar-unread">{count > 99 ? '99+' : count}</b>}</span></span></span>
          </button>
          <details className="row-menu"><summary className="icon-button" aria-label={`Actions for ${label}`}><Icon name="more" width={18} height={18} /></summary><div className="row-menu-popover"><button disabled={busy} onClick={() => update(item, { pinned: !item.pinned })}>{item.pinned ? 'Unpin chat' : 'Pin chat'}</button><button disabled={busy} onClick={() => update(item, { mute_minutes: item.muted ? 0 : 1440 })}>{item.muted ? 'Unmute' : 'Mute for 24 hours'}</button><button disabled={busy} onClick={() => update(item, { archived: !item.archived })}>{item.archived ? 'Unarchive' : 'Archive'}</button><button disabled={busy} className="danger-action" onClick={() => void hide(item)}>Hide chat</button></div></details>
        </li>;
      })}</ul>}
      {hasMore && <button className="load-more soft-button" disabled={loading} onClick={loadMore}>{loading ? 'Loading…' : 'Load more chats'}</button>}
    </div>
  </>}</aside>;
}
