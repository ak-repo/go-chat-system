import { BASE_URL, getToken, refreshAccessToken } from './client';

export type WSEventType = 'message' | 'message.edited' | 'message.deleted' | 'message.replied' | 'message.delivered' | 'message.read' | 'typing' | 'typing.started' | 'typing.stopped' | 'read' | 'user_online' | 'user_offline' | 'ack' | 'error';
export interface WSMessage<T = unknown> { event: WSEventType; sender_id?: string; receiver_id?: string; receiver_type?: 'user' | 'group'; data: T }
export interface ChatMessage { message_id: string; client_message_id?: string; content: string; timestamp: string; conversation_id?: string }
export interface TypingData { state: boolean }
export interface ReadData { message_id: string; read_at?: string; conversation_id?: string }
export interface AckData { server_id?: string; message_id?: string; client_message_id?: string; status: 'sent' | 'delivered' | 'read' | 'failed'; event?: string }
export interface ErrorData { code: string; message: string }
const events = new Set<WSEventType>(['message', 'message.edited', 'message.deleted', 'message.replied', 'message.delivered', 'message.read', 'typing', 'typing.started', 'typing.stopped', 'read', 'user_online', 'user_offline', 'ack', 'error']);
function wsUrl(): string { const u = new URL(`${BASE_URL}/ws`); u.protocol = u.protocol === 'https:' ? 'wss:' : 'ws:'; return `${u}?token=${encodeURIComponent(getToken() ?? '')}`; }

class WSClient {
  private ws: WebSocket | null = null; private attempts = 0; private timer: ReturnType<typeof setTimeout> | null = null; private enabled = false; private lifecycle = 0; private connectedState = false; private handlers = new Map<WSEventType, Set<(m: WSMessage) => void>>(); private stateHandler: ((connected: boolean) => void) | null = null;
  connect(token?: string): void { this.enabled = true; if (this.ws?.readyState === WebSocket.OPEN || this.ws?.readyState === WebSocket.CONNECTING) return; if (token) void this.open(token); else { const current = getToken(); if (current) void this.open(current); } }
  private async open(token: string, lifecycle = this.lifecycle): Promise<void> { if (!this.enabled || lifecycle !== this.lifecycle) return; this.ws?.close(); if (!this.enabled || lifecycle !== this.lifecycle) return; this.ws = new WebSocket(`${wsUrl().split('?')[0]}?token=${encodeURIComponent(token)}`); const socket = this.ws;
    socket.onopen = () => { this.attempts = 0; this.connectedState = true; this.stateHandler?.(true); };
    socket.onmessage = (raw) => { try { const value: unknown = JSON.parse(raw.data as string); if (!value || typeof value !== 'object') return; const m = value as Partial<WSMessage>; if (typeof m.event !== 'string' || !events.has(m.event as WSEventType)) return; this.handlers.get(m.event as WSEventType)?.forEach((handler) => handler(m as WSMessage)); } catch { /* malformed frames are ignored by the client */ } };
     socket.onclose = () => { if (this.ws !== socket) return; this.ws = null; this.connectedState = false; this.stateHandler?.(false); if (this.enabled && lifecycle === this.lifecycle && this.attempts < 5) { const delay = Math.min(16000, 1000 * 2 ** this.attempts); this.attempts += 1; this.timer = setTimeout(() => { void this.reconnect(lifecycle); }, delay); } };
    socket.onerror = () => this.stateHandler?.(false);
  }
  private async reconnect(lifecycle = this.lifecycle): Promise<void> { if (!this.enabled || lifecycle !== this.lifecycle) return; try { const token = await refreshAccessToken(); if (this.enabled && lifecycle === this.lifecycle) await this.open(token, lifecycle); } catch { const token = getToken(); if (token && this.enabled && lifecycle === this.lifecycle) await this.open(token, lifecycle); } }
  disconnect(): void { this.enabled = false; this.lifecycle += 1; if (this.timer) clearTimeout(this.timer); this.timer = null; this.ws?.close(); this.ws = null; this.connectedState = false; this.stateHandler?.(false); }
  setOnStateChange(handler: (connected: boolean) => void): void { this.stateHandler = handler; }
  send<T>(event: WSEventType, data: T, receiverId: string): boolean { if (!this.ws || this.ws.readyState !== WebSocket.OPEN) return false; this.ws.send(JSON.stringify({ event, receiver_id: receiverId, receiver_type: 'user', data })); return true; }
  sendMessage(receiverId: string, content: string, conversationId?: string, clientMessageId: string = crypto.randomUUID()): string | null { return this.send('message', { content, client_message_id: clientMessageId, conversation_id: conversationId }, receiverId) ? clientMessageId : null; }
  sendTyping(receiverId: string, state: boolean, conversationId?: string): boolean { return this.send('typing', { state, conversation_id: conversationId }, receiverId); }
  sendReadReceipt(receiverId: string, messageId: string, conversationId: string): boolean { return this.send('message.read', { message_id: messageId, conversation_id: conversationId }, receiverId); }
  on<T = unknown>(event: WSEventType, handler: (message: WSMessage<T>) => void): () => void { const set = this.handlers.get(event) ?? new Set(); this.handlers.set(event, set); const callback = handler as (m: WSMessage) => void; set.add(callback); return () => set.delete(callback); }
  get connected(): boolean { return this.connectedState; }
}
export const wsClient = new WSClient();
export default wsClient;
