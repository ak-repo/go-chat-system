/* eslint-disable react-refresh/only-export-components, react-hooks/set-state-in-effect */
import { createContext, useContext, useEffect, useState } from 'react';
import type { ReactNode } from 'react';
import wsClient from '../api/websocket';
import type { ChatMessage, TypingData, ReadData, AckData, ErrorData, WSMessage, WSEventType } from '../api/websocket';
import { useAuth } from './AuthContext';
import { getToken } from '../api/client';
import { getUnread } from '../api/messages';
import { applyPresenceSnapshot, applyPresenceUpdate, mergeUnreadSnapshot, type PresenceByUser, type PresenceSnapshot, type PresenceUpdate } from './realtimeState';
import { getNotificationUnread } from '../api/notifications';

interface SocketContextType {
  isConnected: boolean;
  sendMessage: (receiverId: string, content: string, conversationId?: string, clientMessageId?: string) => string | null;
  sendTyping: (receiverId: string, state: boolean, conversationId?: string) => boolean;
  sendReadReceipt: (receiverId: string, messageId: string, conversationId: string) => boolean;
  sendEvent: (event: WSEventType, receiverId: string, data: unknown) => boolean;
  sendConversationEvent: (event: WSEventType, conversationId: string, receiverType: 'user' | 'group', receiverId: string, data: unknown) => boolean;
  onMessage: (handler: (message: WSMessage<ChatMessage>) => void) => () => void;
  onTyping: (handler: (message: WSMessage<TypingData>) => void) => () => void;
  onRead: (handler: (message: WSMessage<ReadData>) => void) => () => void;
  onAck: (handler: (message: WSMessage<AckData>) => void) => () => void;
  onError: (handler: (message: WSMessage<ErrorData>) => void) => () => void;
  onEvent: (event: WSEventType, handler: (message: WSMessage) => void) => () => void;
  reconnect: () => void;
  unread: Record<string, number>;
  presence: PresenceByUser;
  notificationUnread: number;
  clearConversationUnread: (conversationId: string) => void;
  clearAllUnread: () => void;
}
const SocketContext = createContext<SocketContextType | undefined>(undefined);
export function SocketProvider({ children }: { children: ReactNode }) {
  const { isAuthenticated, user } = useAuth(); const [isConnected, setIsConnected] = useState(false); const [unread, setUnread] = useState<Record<string, number>>({}); const [presence, setPresence] = useState<PresenceByUser>({}); const [notificationUnread, setNotificationUnread] = useState(0);
  useEffect(() => { wsClient.setOnStateChange(setIsConnected); return () => wsClient.setOnStateChange(() => undefined); }, []);
  useEffect(() => {
    setUnread({});
    setPresence({});
    setNotificationUnread(0);
    if (isAuthenticated && getToken()) wsClient.connect(); else wsClient.disconnect();
  }, [isAuthenticated, user?.id]);
  useEffect(() => { if (!isConnected || !user?.id) return; let active = true; void getUnread().then((response) => { if (active && response.success && response.data) setUnread((current) => mergeUnreadSnapshot(current, response.data!.unread)); }).catch(() => undefined); void getNotificationUnread().then((response) => { if (active && response.data) setNotificationUnread(response.data.unread); }).catch(() => undefined); return () => { active = false; }; }, [isConnected, user?.id]);
  useEffect(() => {
    const transition = (online: boolean) => (message: WSMessage) => { const data = message.data as Partial<PresenceUpdate>; if (data.user_id && data.timestamp) setPresence((current) => applyPresenceUpdate(current, { user_id: data.user_id!, online, timestamp: data.timestamp! })); };
    const off = [wsClient.on('user_online', transition(true)), wsClient.on('user_offline', transition(false)), wsClient.on('presence.snapshot', (message) => setPresence((current) => applyPresenceSnapshot(current, message.data as PresenceSnapshot)))];
    return () => off.forEach((remove) => remove());
  }, []);
  useEffect(() => { const off = [wsClient.on('notification.created', () => setNotificationUnread((count) => count + 1)), wsClient.on('notification.read', (message) => { const data = message.data as { all?: boolean }; setNotificationUnread((count) => data.all ? 0 : Math.max(0, count - 1)); })]; return () => off.forEach((remove) => remove()); }, []);
  useEffect(() => {
    const incrementUnread = (message: WSMessage) => {
      const data = message.data as ChatMessage;
      if (!data.conversation_id || message.sender_id === user?.id) return;
      setUnread((current) => ({ ...current, [data.conversation_id!]: (current[data.conversation_id!] ?? 0) + 1 }));
    };
    const off = [wsClient.on('message', incrementUnread), wsClient.on('message.replied', incrementUnread)];
    return () => off.forEach((remove) => remove());
  }, [user?.id]);
  const value: SocketContextType = {
    isConnected, sendMessage: (receiver, content, conversation, clientMessageId) => wsClient.sendMessage(receiver, content, conversation, clientMessageId),
    sendTyping: (receiver, state, conversation) => wsClient.sendTyping(receiver, state, conversation), sendReadReceipt: (receiver, message, conversation) => wsClient.sendReadReceipt(receiver, message, conversation), sendEvent: (event, receiver, data) => wsClient.sendEvent(event, receiver, data), sendConversationEvent: (event, conversation, type, receiver, data) => wsClient.sendConversationEvent(event, conversation, type, receiver, data),
    onMessage: (handler) => wsClient.on('message', handler), onTyping: (handler) => {
      const off = [wsClient.on('typing', handler), wsClient.on('typing.started', (message) => handler({ ...message, data: { state: true } })), wsClient.on('typing.stopped', (message) => handler({ ...message, data: { state: false } }))];
      return () => off.forEach((remove) => remove());
    }, onRead: (handler) => { const off = [wsClient.on('message.read', handler), wsClient.on('read', handler)]; return () => off.forEach((remove) => remove()); }, onAck: (handler) => wsClient.on('ack', handler), onError: (handler) => wsClient.on('error', handler), onEvent: (event, handler) => wsClient.on(event, handler), reconnect: () => wsClient.connect(), unread, presence, notificationUnread,
    clearConversationUnread: (conversationId) => setUnread((current) => current[conversationId] === undefined ? current : { ...current, [conversationId]: 0 }),
    clearAllUnread: () => setUnread({}),
  };
  return <SocketContext.Provider value={value}>{children}</SocketContext.Provider>;
}
export function useSocket(): SocketContextType { const context = useContext(SocketContext); if (!context) throw new Error('useSocket must be used within a SocketProvider'); return context; }
export default SocketContext;
