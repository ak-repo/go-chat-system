import { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { listFriends, type Friend } from '../api/friends';
import { createOrGetConversation, conversationInitials } from '../api/conversations';
import { useConversations } from '../context/ConversationContext';
import Icon from './Icon';
export default function NewChatPanel() {
  const { setNewChat, refresh, setArchived } = useConversations();
  const navigate = useNavigate();
  const [friends, setFriends] = useState<Friend[]>([]);
  const [query, setQuery] = useState('');
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  useEffect(() => {
    let active = true;
    async function load() {
      try {
        const all: Friend[] = [];
        for (let offset = 0; ; offset += 100) {
          const result = await listFriends(100, offset);
          if (!active) return;
          if (!result.success || !result.data) throw new Error(result.error || 'Could not load contacts');
          const page = result.data.friends ?? [];
          all.push(...page);
          if (page.length < 100) break;
        }
        if (active) setFriends(all);
      } catch (cause) { if (active) setError(cause instanceof Error ? cause.message : 'Could not load contacts'); }
      finally { if (active) setLoading(false); }
    }
    void load(); return () => { active = false; };
  }, []);
  async function open(friend: Friend) {
    if (busy) return;
    setBusy(true); setError('');
    try {
      const result = await createOrGetConversation(friend.FriendID);
      if (!result.success || !result.data) throw new Error(result.error || 'Could not open chat');
      setArchived(false); refresh(); setNewChat(false);
      navigate(`/conversations/${result.data.id}`);
    } catch (cause) { setError(cause instanceof Error ? cause.message : 'Could not open chat'); }
    finally { setBusy(false); }
  }
  const filtered = friends.filter((friend) => friend.FriendName.toLowerCase().includes(query.trim().toLowerCase())).sort((a, b) => a.FriendName.localeCompare(b.FriendName));
  return <>
    <header className="chat-sidebar-heading"><button className="icon-button" aria-label="Back to chats" onClick={() => setNewChat(false)}><Icon name="back" /></button><h1 className="panel-title">New chat</h1></header>
    <label className="search-box sidebar-search"><Icon name="search" width={20} height={20} /><input autoFocus aria-label="Search contacts" placeholder="Search contacts by name" value={query} onChange={(event) => setQuery(event.target.value)} /></label>
    <div className="sidebar-chat-list">
      <Link to="/conversations/new-group" onClick={() => setNewChat(false)} className="contact-action"><span className="action-avatar"><Icon name="people" /></span>New group</Link>
      <Link to="/friends" onClick={() => setNewChat(false)} className="contact-action"><span className="action-avatar"><Icon name="plus" /></span>Find people</Link>
      <p className="contacts-label">Your contacts</p>
      {error && <p className="inline-alert" role="alert">{error}</p>}
      {loading ? <p className="list-placeholder">Loading contacts…</p> : filtered.length === 0 ? <p className="list-placeholder">{query ? 'No matching contacts.' : 'Add friends to start chatting.'}</p> : filtered.map((friend, index) => <div key={friend.FriendID}>{(index === 0 || filtered[index - 1].FriendName[0]?.toUpperCase() !== friend.FriendName[0]?.toUpperCase()) && <div className="contacts-label">{friend.FriendName[0]?.toUpperCase()}</div>}<button disabled={busy} className="sidebar-chat-row contact-row" onClick={() => void open(friend)}><span className="conversation-avatar">{conversationInitials(friend.FriendName)}</span><span className="sidebar-chat-copy"><strong>{friend.FriendName}</strong></span></button></div>)}
    </div>
  </>;
}
