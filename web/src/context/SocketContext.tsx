/* eslint-disable react-refresh/only-export-components */
import { createContext, useContext, useEffect, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import wsClient from '../api/websocket';
import type { ChatMessage, TypingData, ReadData, AckData } from '../api/websocket';
import { useAuth } from './AuthContext';
import { getToken } from '../api/client';

interface SocketContextType {
  isConnected: boolean;
  sendMessage: (receiverId: string, content: string) => void;
  sendTyping: (receiverId: string, isTyping: boolean) => void;
  sendReadReceipt: (receiverId: string, messageId: string) => void;
  onMessage: (handler: (data: ChatMessage) => void) => () => void;
  onTyping: (handler: (data: TypingData) => void) => () => void;
  onRead: (handler: (data: ReadData) => void) => () => void;
  onAck: (handler: (data: AckData) => void) => () => void;
  reconnect: () => void;
}

const SocketContext = createContext<SocketContextType | undefined>(undefined);

export function SocketProvider({ children }: { children: ReactNode }) {
  const { isAuthenticated } = useAuth();
  const [isConnected, setIsConnected] = useState(false);
  const tokenRef = useRef<string | null>(null);

  // Register state change listener once
  useEffect(() => {
    wsClient.setOnStateChange(setIsConnected);
    return () => wsClient.setOnStateChange(() => {});
  }, []);

  // Connect/disconnect based on auth state
  useEffect(() => {
    const token = getToken();
    tokenRef.current = token;
    
    if (isAuthenticated && token) {
      wsClient.connect(token);
    } else {
      wsClient.disconnect();
    }
  }, [isAuthenticated]);

  // Manual reconnect
  const reconnect = () => {
    if (tokenRef.current) {
      wsClient.connect(tokenRef.current);
    }
  };

  const sendMessage = (receiverId: string, content: string) => {
    wsClient.sendMessage(receiverId, content);
  };

  const sendTyping = (receiverId: string, isTyping: boolean) => {
    wsClient.sendTyping(receiverId, isTyping);
  };

  const sendReadReceipt = (receiverId: string, messageId: string) => {
    wsClient.sendReadReceipt(receiverId, messageId);
  };

  const onMessage = (handler: (data: ChatMessage) => void) => {
    return wsClient.onMessage(handler);
  };

  const onTyping = (handler: (data: TypingData) => void) => {
    return wsClient.onTyping(handler);
  };

  const onRead = (handler: (data: ReadData) => void) => {
    return wsClient.onRead(handler);
  };

  const onAck = (handler: (data: AckData) => void) => {
    return wsClient.onAck(handler);
  };

  const value: SocketContextType = {
    isConnected,
    sendMessage,
    sendTyping,
    sendReadReceipt,
    onMessage,
    onTyping,
    onRead,
    onAck,
    reconnect,
  };

  return <SocketContext.Provider value={value}>{children}</SocketContext.Provider>;
}

export function useSocket(): SocketContextType {
  const context = useContext(SocketContext);
  if (!context) {
    throw new Error('useSocket must be used within a SocketProvider');
  }
  return context;
}

export default SocketContext;
