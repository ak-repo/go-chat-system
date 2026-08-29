/* eslint-disable react-refresh/only-export-components */
import { createContext, useContext, useEffect, useState } from 'react';
import type { ReactNode } from 'react';
import wsClient from '../api/websocket';
import type { ChatMessage, TypingData, ReadData, AckData, ErrorData, WSMessage, WSEventType } from '../api/websocket';
import { useAuth } from './AuthContext';
import { getToken } from '../api/client';
import { getUnread } from '../api/messages';

interface SocketContextType {
  isConnected: boolean;
  sendMessage: (receiverId: string, content: string, conversationId?: string, clientMessageId?: string) => string | null;
  sendTyping: (receiverId: string, state: boolean, conversationId?: string) => boolean;
  sendReadReceipt: (receiverId: string, messageId: string, conversationId: string) => boolean;
  onMessage: (handler: (message: WSMessage<ChatMessage>) => void) => () => void;
  onTyping: (handler: (message: WSMessage<TypingData>) => void) => () => void;
  onRead: (handler: (message: WSMessage<ReadData>) => void) => () => void;
  onAck: (handler: (message: WSMessage<AckData>) => void) => () => void;
  onError: (handler: (message: WSMessage<ErrorData>) => void) => () => void;
  onEvent: (event: WSEventType, handler: (message: WSMessage) => void) => () => void;
  reconnect: () => void;
  unread: Record<string, number>;
}
const SocketContext = createContext<SocketContextType | undefined>(undefined);
export function SocketProvider({ children }: { children: ReactNode }) {
  const { isAuthenticated } = useAuth(); const [isConnected, setIsConnected] = useState(false); const [unread, setUnread] = useState<Record<string, number>>({});
  useEffect(() => { wsClient.setOnStateChange(setIsConnected); return () => wsClient.setOnStateChange(() => undefined); }, []);
  useEffect(() => { if (isAuthenticated && getToken()) wsClient.connect(); else wsClient.disconnect(); }, [isAuthenticated]);
  useEffect(() => { if (!isConnected) return; void getUnread().then((response) => { if (response.success && response.data) setUnread(response.data.unread); }).catch(() => undefined); }, [isConnected]);
  const value: SocketContextType = {
    isConnected, sendMessage: (receiver, content, conversation, clientMessageId) => wsClient.sendMessage(receiver, content, conversation, clientMessageId),
    sendTyping: (receiver, state, conversation) => wsClient.sendTyping(receiver, state, conversation), sendReadReceipt: (receiver, message, conversation) => wsClient.sendReadReceipt(receiver, message, conversation),
    onMessage: (handler) => wsClient.on('message', handler), onTyping: (handler) => {
      const off = [wsClient.on('typing', handler), wsClient.on('typing.started', (message) => handler({ ...message, data: { state: true } })), wsClient.on('typing.stopped', (message) => handler({ ...message, data: { state: false } }))];
      return () => off.forEach((remove) => remove());
    }, onRead: (handler) => { const off = [wsClient.on('message.read', handler), wsClient.on('read', handler)]; return () => off.forEach((remove) => remove()); }, onAck: (handler) => wsClient.on('ack', handler), onError: (handler) => wsClient.on('error', handler), onEvent: (event, handler) => wsClient.on(event, handler), reconnect: () => wsClient.connect(), unread,
  };
  return <SocketContext.Provider value={value}>{children}</SocketContext.Provider>;
}
export function useSocket(): SocketContextType { const context = useContext(SocketContext); if (!context) throw new Error('useSocket must be used within a SocketProvider'); return context; }
export default SocketContext;
