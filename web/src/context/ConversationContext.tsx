/* eslint-disable react-refresh/only-export-components, react-hooks/set-state-in-effect */
import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from 'react';
import { listConversations, type Conversation } from '../api/conversations';
import { getPublicUser } from '../api/users';
import type { Message } from '../api/messages';
import { useAuth } from './AuthContext';
import { useSocket } from './SocketContext';
import wsClient, { type ChatMessage } from '../api/websocket';
export type ChatPreview = { id: string; content: string; timestamp?: string };
interface ConversationState {
  items: Conversation[]; names: Record<string, string>; previews: Record<string, ChatPreview>;
  loading: boolean; error: string; archived: boolean; hasMore: boolean;
  setArchived: (value: boolean) => void; refresh: () => void; loadMore: () => void;
  recordMessages: (conversationId: string, messages: Message[]) => void;
  newChat: boolean; setNewChat: (value: boolean) => void;
}
const ConversationContext = createContext<ConversationState | null>(null);
export function ConversationProvider({ children }: { children: ReactNode }) {
  const { user } = useAuth();
  const { isConnected } = useSocket();
  const [items, setItems] = useState<Conversation[]>([]);
  const [names, setNames] = useState<Record<string, string>>({});
  const [previews, setPreviews] = useState<Record<string, ChatPreview>>({});
  const [archived, setArchived] = useState(false);
  const [newChat, setNewChat] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [hasMore, setHasMore] = useState(false);
  const generation = useRef(0);
  const offset = useRef(0);
  const busy = useRef(false);
  const namesRef = useRef(names);
  useEffect(() => { namesRef.current = names; }, [names]);
  const load = useCallback(async (more = false) => {
    if (more && busy.current) return;
    const version = more ? generation.current : ++generation.current;
    busy.current = true; setLoading(true); setError('');
    const start = more ? offset.current : 0;
    try {
      const result = await listConversations(100, start, archived);
      if (version !== generation.current) return;
      if (!result.success || !result.data) throw new Error(result.error || 'Could not load chats');
      const page = result.data.conversations ?? [];
      setItems((current) => more ? [...current.filter((item) => !page.some((next) => next.id === item.id)), ...page] : page);
      offset.current = start + page.length;
      setHasMore(page.length >= 100);
      const ids = [...new Set(page.filter((item) => item.kind === 'direct').map((item) => item.user_one_id === user?.id ? item.user_two_id : item.user_one_id))].filter((id) => !namesRef.current[id]);
      const profiles = await Promise.all(ids.map(async (id) => {
        try { const profile = await getPublicUser(id); return [id, profile.data?.username ?? 'Unknown contact'] as const; }
        catch { return [id, 'Unknown contact'] as const; }
      }));
      if (version === generation.current) setNames((current) => ({ ...current, ...Object.fromEntries(profiles) }));
    } catch (cause) {
      if (version === generation.current) setError(cause instanceof Error ? cause.message : 'Could not load chats');
    } finally { if (version === generation.current) { busy.current = false; setLoading(false); } }
  }, [archived, user?.id]);
  const refresh = useCallback(() => { void load(); }, [load]);
  const loadMore = useCallback(() => { void load(true); }, [load]);
  useEffect(() => { setItems([]); setHasMore(false); refresh(); return () => { generation.current += 1; busy.current = false; }; }, [refresh, isConnected]);
  useEffect(() => { setPreviews({}); setNames({}); setNewChat(false); }, [user?.id]);
  const recordMessages = useCallback((id: string, messages: Message[]) => {
    const last = messages.at(-1);
    if (!last) return;
    const preview = { id: last.id, content: last.deleted_at?.Valid ? 'Message deleted' : last.content, timestamp: last.created_at };
    setPreviews((current) => {
      const previous = current[id];
      if (previous && previous.timestamp && Date.parse(previous.timestamp) > Date.parse(preview.timestamp)) return current;
      if (previous?.id === preview.id && previous.content === preview.content && previous.timestamp === preview.timestamp) return current;
      return { ...current, [id]: preview };
    });
  }, []);
  useEffect(() => {
    const receive = (message: { data: Partial<ChatMessage> & { server_id?: string } }) => {
      const data = message.data;
      if (!data.conversation_id || !data.content || !(data.server_id || data.message_id)) return;
      setPreviews((current) => {
        const previous = current[data.conversation_id!];
        if (previous && previous.timestamp && data.timestamp && Date.parse(previous.timestamp) > Date.parse(data.timestamp)) return current;
        return { ...current, [data.conversation_id!]: { id: (data.server_id || data.message_id)!, content: data.content!, timestamp: data.timestamp } };
      });
    };
    const off = [wsClient.on('message', (message) => receive(message as { data: ChatMessage })), wsClient.on('message.replied', (message) => receive(message as { data: ChatMessage })), wsClient.on('message.edited', (message) => {
      const data = message.data as { conversation_id?: string; message_id?: string; content?: string };
      if (data.conversation_id) setPreviews((current) => { const preview = current[data.conversation_id!]; return preview?.id === data.message_id ? { ...current, [data.conversation_id!]: { ...preview, content: data.content ?? preview.content } } : current; });
    }), wsClient.on('message.deleted', (message) => {
      const data = message.data as { conversation_id?: string; message_id?: string };
      if (data.conversation_id) setPreviews((current) => { const preview = current[data.conversation_id!]; return preview?.id === data.message_id ? { ...current, [data.conversation_id!]: { ...preview, content: 'Message deleted' } } : current; });
    })];
    return () => off.forEach((remove) => remove());
  }, []);
  return <ConversationContext.Provider value={{ items, names, previews, loading, error, archived, hasMore, setArchived, refresh, loadMore, recordMessages, newChat, setNewChat }}>{children}</ConversationContext.Provider>;
}
export function useConversations() {
  const context = useContext(ConversationContext);
  if (!context) throw new Error('ConversationProvider is required');
  return context;
}
