import { BASE_URL, getToken } from './client';

function getWSUrl(): string {
  const url = new URL(`${BASE_URL}/ws`);
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
  return url.toString();
}

// WebSocket message types
export type WSEventType = 'message' | 'typing' | 'read' | 'ack' | 'error';

export interface WSMessage<T = unknown> {
  event: WSEventType;
  sender_id?: string;
  receiver_id?: string;
  receiver_type?: 'user' | 'group';
  data: T;
}

export interface ChatMessage {
  message_id: string;
  content: string;
  timestamp: string;
}

export interface TypingData {
  is_typing: boolean;
}

export interface ReadData {
  message_id: string;
  read_at: string;
}

export interface AckData {
  message_id: string;
  status: 'sent' | 'delivered' | 'read' | 'failed';
}

// WebSocket client class
class WSClient {
  private ws: WebSocket | null = null;
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 5;
  private reconnectDelay = 1000;
  private messageHandlers: Map<WSEventType, Set<(data: unknown) => void>> = new Map();
  private isConnected = false;
  private shouldReconnect = true;
  private onStateChange: ((connected: boolean) => void) | null = null;

  // Connect to WebSocket
  connect(token?: string): void {
    this.shouldReconnect = true;

    const authToken = token || getToken();
    if (!authToken) {
      console.error('No auth token available for WebSocket connection');
      return;
    }

    // Close existing connection before creating a new one
    if (this.ws) {
      this.ws.onclose = null;
      this.ws.close();
      this.ws = null;
    }

    const url = `${getWSUrl()}?token=${authToken}`;
    this.ws = new WebSocket(url);

    this.ws.onopen = () => {
      console.log('WebSocket connected');
      this.isConnected = true;
      this.reconnectAttempts = 0;
      this.onStateChange?.(true);
    };

    this.ws.onmessage = (event) => {
      try {
        const message: WSMessage = JSON.parse(event.data);
        const handlers = this.messageHandlers.get(message.event);
        if (handlers) {
          handlers.forEach((handler) => handler(message.data));
        }
      } catch (error) {
        console.error('Failed to parse WebSocket message:', error);
      }
    };

    this.ws.onclose = () => {
      console.log('WebSocket disconnected');
      this.isConnected = false;
      this.onStateChange?.(false);
      if (this.shouldReconnect && this.reconnectAttempts < this.maxReconnectAttempts) {
        this.scheduleReconnect(authToken);
      }
    };

    this.ws.onerror = (error) => {
      console.error('WebSocket error:', error);
      this.onStateChange?.(false);
    };
  }

  // Schedule reconnection
  private scheduleReconnect(token: string): void {
    this.reconnectAttempts++;
    const delay = this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1);
    console.log(`Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts})`);
    setTimeout(() => this.connect(token), delay);
  }

  // Disconnect
  disconnect(): void {
    this.shouldReconnect = false;
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
    this.isConnected = false;
    this.onStateChange?.(false);
  }

  // Set callback for connection state changes
  setOnStateChange(callback: (connected: boolean) => void): void {
    this.onStateChange = callback;
  }

  // Send message
  send(event: WSEventType, data: unknown, receiverId: string): void {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      console.error('WebSocket not connected');
      return;
    }

    const message: WSMessage = {
      event,
      receiver_id: receiverId,
      receiver_type: 'user',
      data,
    };

    this.ws.send(JSON.stringify(message));
  }

  // Send chat message
  sendMessage(receiverId: string, content: string): void {
    const messageId = crypto.randomUUID();
    const data: ChatMessage = {
      message_id: messageId,
      content,
      timestamp: new Date().toISOString(),
    };
    this.send('message', data, receiverId);
  }

  // Send typing indicator
  sendTyping(receiverId: string, isTyping: boolean): void {
    const data: TypingData = { is_typing: isTyping };
    this.send('typing', data, receiverId);
  }

  // Send read receipt
  sendReadReceipt(receiverId: string, messageId: string): void {
    const data: ReadData = {
      message_id: messageId,
      read_at: new Date().toISOString(),
    };
    this.send('read', data, receiverId);
  }

  // Register event handler
  on(event: WSEventType, handler: (data: unknown) => void): () => void {
    if (!this.messageHandlers.has(event)) {
      this.messageHandlers.set(event, new Set());
    }
    this.messageHandlers.get(event)!.add(handler);

    // Return unsubscribe function
    return () => {
      this.messageHandlers.get(event)?.delete(handler);
    };
  }

  // Register message handler
  onMessage(handler: (data: ChatMessage) => void): () => void {
    return this.on('message', handler as (data: unknown) => void);
  }

  // Register typing handler
  onTyping(handler: (data: TypingData) => void): () => void {
    return this.on('typing', handler as (data: unknown) => void);
  }

  // Register read receipt handler
  onRead(handler: (data: ReadData) => void): () => void {
    return this.on('read', handler as (data: unknown) => void);
  }

  // Register ack handler
  onAck(handler: (data: AckData) => void): () => void {
    return this.on('ack', handler as (data: unknown) => void);
  }

  // Register error handler
  onError(handler: (data: { message: string }) => void): () => void {
    return this.on('error', handler as (data: unknown) => void);
  }

  // Check if connected
  get connected(): boolean {
    return this.isConnected;
  }
}

// Export singleton instance
export const wsClient = new WSClient();
export default wsClient;
