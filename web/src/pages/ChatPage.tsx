import { useCallback, useEffect, useRef, useState, type FormEvent } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';
import { useSocket } from '../context/SocketContext';
import { createOrGetConversation } from '../api/conversations';
import { getConversationMessages, markConversationRead, mergeMessagesChronologically, sortMessagesChronologically, type Message } from '../api/messages';

export default function ChatPage() {
  const { userId } = useParams<{ userId: string }>();
  const navigate = useNavigate();
  const { user, logout } = useAuth();
  const { isConnected, sendMessage, sendTyping, onMessage, onTyping, onAck, onError } = useSocket();
  const [messages, setMessages] = useState<Message[]>([]);
  const [conversationId, setConversationId] = useState<string | null>(null);
  const [input, setInput] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [typing, setTyping] = useState(false);
  const [offset, setOffset] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const typingTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const endRef = useRef<HTMLDivElement>(null);
  const requestVersion = useRef(0);
  const conversationForUser = useRef<{ userId: string; id: string } | null>(null);

  const load = useCallback(async (older = false) => {
    if (!userId) return;
    const version = requestVersion.current;
    setLoading(!older); setError('');
    try {
      let id = conversationForUser.current?.userId === userId ? conversationForUser.current.id : null;
      if (!id) {
        const conversation = await createOrGetConversation(userId);
        if (version !== requestVersion.current) return;
        if (!conversation.success || !conversation.data) throw new Error(conversation.error || 'Conversation unavailable');
        id = conversation.data.id; conversationForUser.current = { userId, id }; setConversationId(id);
      }
      const result = await getConversationMessages(id, 50, older ? offset + 50 : 0);
      if (version !== requestVersion.current) return;
      if (!result.success || !result.data) throw new Error(result.error || 'Failed to load messages');
      setMessages((current) => older ? sortMessagesChronologically([...(result.data?.messages ?? []), ...current]) : sortMessagesChronologically(result.data?.messages ?? []));
      setOffset(older ? offset + 50 : 0); setHasMore((result.data.messages?.length ?? 0) >= result.data.limit);
    } catch (cause) { if (version === requestVersion.current) setError(cause instanceof Error ? cause.message : 'Failed to load messages'); }
    finally { if (version === requestVersion.current) setLoading(false); }
  }, [offset, userId]);

  // Route changes must clear conversation-local state before loading the next peer.
  // eslint-disable-next-line react-hooks/set-state-in-effect
  useEffect(() => { requestVersion.current += 1; conversationForUser.current = null; setMessages([]); setConversationId(null); setOffset(0); setHasMore(false); setTyping(false); setError(''); }, [userId]);
  // eslint-disable-next-line react-hooks/set-state-in-effect
  useEffect(() => { if (!user) { navigate('/login', { replace: true }); return; } void load(); }, [user, userId, navigate, load]);
  // eslint-disable-next-line react-hooks/set-state-in-effect, react-hooks/exhaustive-deps
  useEffect(() => { if (isConnected && conversationId) void load(); }, [isConnected]);
  useEffect(() => {
    const offMessage = onMessage((event) => {
      if (event.sender_id !== userId || event.receiver_id !== user?.id) return;
      const data = event.data;
      setMessages((current) => mergeMessagesChronologically(current, { id: data.message_id, sender_id: event.sender_id ?? userId ?? '', receiver_id: event.receiver_id ?? user?.id ?? '', content: data.content, is_group: false, created_at: data.timestamp, modified_at: data.timestamp, client_message_id: data.client_message_id, status: 'sent' }));
    });
    const offTyping = onTyping((event) => { if (event.sender_id === userId) setTyping(event.data.state); });
    const offAck = onAck((event) => { const data = event.data; if (data.client_message_id) setMessages((current) => current.map((message) => message.client_message_id === data.client_message_id ? { ...message, id: data.server_id ?? message.id, status: data.status } : message)); });
    const offError = onError(() => setMessages((current) => current.map((message) => message.status === 'sending' ? { ...message, status: 'failed' } : message)));
    return () => { offMessage(); offTyping(); offAck(); offError(); };
  }, [onMessage, onTyping, onAck, onError, userId, user?.id]);
  useEffect(() => { const incoming = [...messages].reverse().find((message) => message.sender_id === userId); if (conversationId && incoming) void markConversationRead(conversationId, incoming.id); endRef.current?.scrollIntoView({ behavior: 'smooth' }); }, [messages, conversationId, userId]);

  const submit = (event: FormEvent) => {
    event.preventDefault(); if (!userId || !input.trim()) return;
    const content = input.trim(); const id = crypto.randomUUID(); const now = new Date().toISOString();
    setMessages((current) => mergeMessagesChronologically(current, { id, client_message_id: id, sender_id: user?.id ?? '', receiver_id: userId, content, is_group: false, created_at: now, modified_at: now, status: 'sending' }));
    if (!sendMessage(userId, content, conversationId ?? undefined, id)) setMessages((current) => current.map((message) => message.client_message_id === id ? { ...message, status: 'failed' } : message));
    setInput(''); if (conversationId) sendTyping(userId, false, conversationId);
  };
  const retry = (message: Message) => { if (!userId || !message.client_message_id) return; setMessages((current) => current.map((item) => item.client_message_id === message.client_message_id ? { ...item, status: 'sending' } : item)); if (!sendMessage(userId, message.content, conversationId ?? undefined, message.client_message_id)) setMessages((current) => current.map((item) => item.client_message_id === message.client_message_id ? { ...item, status: 'failed' } : item)); };

  if (loading) return <div className="app-shell flex min-h-screen items-center justify-center text-slate-300">Loading...</div>;
  return (
    <div className="app-shell flex min-h-screen flex-col">
      <header className="app-header shrink-0">
        <div className="mx-auto flex w-full max-w-5xl items-center justify-between px-4 py-4 sm:px-6">
          <Link to="/friends" className="text-sm font-semibold text-blue-400 hover:text-blue-300">← Back to friends</Link>
          <div className="flex items-center gap-3 sm:gap-5">
            <span className={isConnected ? 'flex items-center gap-2 text-sm text-emerald-400' : 'flex items-center gap-2 text-sm text-red-300'}><span className={isConnected ? 'status-dot' : 'h-2 w-2 rounded-full bg-red-400'} />{isConnected ? 'Connected' : 'Disconnected'}</span>
            <Link to="/profile" className="hidden text-sm font-medium text-slate-200 hover:text-white sm:block">{user?.username}</Link>
            <button onClick={() => { void logout(); navigate('/login'); }} className="text-sm text-slate-400 hover:text-red-300">Logout</button>
          </div>
        </div>
      </header>
      {error && <div className="mx-auto w-full max-w-5xl px-4 pt-4 text-sm text-red-300 sm:px-6">{error}</div>}
      <main className="chat-pattern scrollbar-thin flex-1 overflow-y-auto px-4 py-6 sm:px-6">
        <div className="mx-auto flex max-w-3xl flex-col space-y-4">
          <div className="mb-3 flex items-center gap-3 border-b border-[#25364d] pb-5"><div className="avatar h-11 w-11">{userId?.charAt(0).toUpperCase()}</div><div><p className="font-semibold text-white">Conversation</p><p className="text-xs text-slate-400">{typing ? 'Typing...' : isConnected ? 'Online now' : 'Offline'}</p></div></div>
          {hasMore && <button onClick={() => void load(true)} className="self-center rounded-full bg-[#1c3049] px-4 py-2 text-sm text-blue-300 hover:bg-[#25405f]">Load older messages</button>}
          {messages.length === 0 ? <div className="py-16 text-center text-slate-400"><p className="font-medium text-slate-200">No messages yet</p><p className="mt-1 text-sm">Start the conversation below.</p></div> : messages.map((message) => <div key={message.id} className={`flex ${message.sender_id === user?.id ? 'justify-end' : 'justify-start'}`}><div className={`max-w-[85%] px-4 py-3 shadow-lg shadow-black/10 md:max-w-md ${message.sender_id === user?.id ? 'message-out' : 'message-in'}`}><div className="break-words text-[15px] leading-6">{message.content}</div><div className="mt-1 text-right text-[11px] opacity-65">{new Date(message.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })} {message.sender_id === user?.id && `· ${message.status ?? 'sent'}`}{message.status === 'failed' && <button onClick={() => retry(message)} className="ml-2 underline">Retry</button>}</div></div></div>)}
          {typing && <div className="text-sm italic text-slate-400">Partner is typing...</div>}<div ref={endRef} />
        </div>
      </main>
      <div className="shrink-0 border-t border-[#25364d] bg-[#111d2d] p-4"><form onSubmit={submit} className="mx-auto flex max-w-3xl gap-2"><input value={input} onChange={(event) => { setInput(event.target.value); if (userId && conversationId) { sendTyping(userId, true, conversationId); if (typingTimer.current) clearTimeout(typingTimer.current); typingTimer.current = setTimeout(() => sendTyping(userId, false, conversationId), 1000); } }} className="app-input min-w-0 flex-1 px-4 py-3" placeholder={isConnected ? 'Write a message...' : 'Reconnect to send'} disabled={!isConnected} /><button disabled={!isConnected || !input.trim()} className="primary-button px-5 py-3 text-sm font-semibold disabled:opacity-50">Send</button></form></div>
    </div>
  );
}
