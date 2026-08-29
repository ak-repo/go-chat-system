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
  if (loading) return <div className="min-h-screen flex items-center justify-center">Loading...</div>;
  return <div className="min-h-screen bg-gray-50 flex flex-col"><header className="bg-white shadow"><div className="max-w-4xl mx-auto px-4 py-4 flex justify-between"><Link to="/friends" className="text-purple-600">← Back</Link><div className="flex gap-4"><span className={isConnected ? 'text-green-600' : 'text-red-600'}>{isConnected ? 'Connected' : 'Disconnected'}</span><Link to="/profile">{user?.username}</Link><button onClick={() => { void logout(); navigate('/login'); }} className="text-sm text-red-600">Logout</button></div></div></header>{error && <div className="max-w-4xl w-full mx-auto p-3 text-red-700">{error}</div>}<main className="flex-1 overflow-y-auto p-4"><div className="max-w-4xl mx-auto space-y-4">{hasMore && <button onClick={() => void load(true)} className="text-purple-600 text-sm">Load older messages</button>}{messages.length === 0 ? <div className="text-center text-gray-500 py-8">No messages yet. Start the conversation!</div> : messages.map((message) => <div key={message.id} className={`flex ${message.sender_id === user?.id ? 'justify-end' : 'justify-start'}`}><div className={`max-w-xs md:max-w-md px-4 py-2 rounded-lg ${message.sender_id === user?.id ? 'bg-purple-600 text-white' : 'bg-gray-200 text-gray-800'}`}><div>{message.content}</div><div className="text-xs mt-1 opacity-70">{new Date(message.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })} {message.sender_id === user?.id && `· ${message.status ?? 'sent'}`}{message.status === 'failed' && <button onClick={() => retry(message)} className="ml-2 underline">Retry</button>}</div></div></div>)}{typing && <div className="text-gray-500 text-sm italic">Partner is typing...</div>}<div ref={endRef} /></div></main><div className="bg-white border-t p-4"><form onSubmit={submit} className="max-w-4xl mx-auto flex gap-2"><input value={input} onChange={(event) => { setInput(event.target.value); if (userId && conversationId) { sendTyping(userId, true, conversationId); if (typingTimer.current) clearTimeout(typingTimer.current); typingTimer.current = setTimeout(() => sendTyping(userId, false, conversationId), 1000); } }} className="flex-1 px-3 py-2 border rounded-md" placeholder={isConnected ? 'Type a message...' : 'Reconnect to send'} disabled={!isConnected} /><button disabled={!isConnected || !input.trim()} className="px-4 py-2 bg-purple-600 text-white rounded-md disabled:opacity-50">Send</button></form></div></div>;
}
