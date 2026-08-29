import apiClient, { toApiResponse } from './client';
import type { ApiResponse } from './client';

export type MessageStatus = 'sending' | 'sent' | 'delivered' | 'read' | 'failed';
export interface Message { id: string; sender_id: string; receiver_id: string; content: string; is_group: boolean; created_at: string; modified_at: string; deleted_at?: { Time: string; Valid: boolean } | null; conversation_id?: string; client_message_id?: string; edited_at?: string; reply_to_message_id?: string; status?: MessageStatus }
export interface MessagesResponse { messages: Message[]; limit: number; offset: number; conversation_id?: string }
export type UnreadResponse = { unread: Record<string, number> };
function timestamp(m: Message): number { const value = Date.parse(m.created_at); return Number.isNaN(value) ? 0 : value; }
export function compareMessagesChronologically(a: Message, b: Message): number { return timestamp(a) - timestamp(b) || a.id.localeCompare(b.id); }
export function sortMessagesChronologically(messages: Message[]): Message[] { return [...messages].sort(compareMessagesChronologically); }
export function mergeMessagesChronologically(current: Message[], next: Message): Message[] {
  const index = current.findIndex((m) => m.id === next.id || (!!next.client_message_id && m.client_message_id === next.client_message_id));
  if (index >= 0) { const result = [...current]; result[index] = { ...result[index], ...next, status: next.status ?? result[index].status }; return sortMessagesChronologically(result); }
  return sortMessagesChronologically([...current, next]);
}
export async function getMessages(otherUserId: string, limit = 50, offset = 0): Promise<ApiResponse<MessagesResponse>> { const r = await apiClient.get<ApiResponse<MessagesResponse>>('/messages', { params: { user_id: otherUserId, limit, offset } }); return toApiResponse(r.data); }
export async function getConversationMessages(id: string, limit = 50, offset = 0): Promise<ApiResponse<MessagesResponse>> { const r = await apiClient.get<ApiResponse<MessagesResponse>>(`/conversations/${encodeURIComponent(id)}/messages`, { params: { limit, offset } }); return toApiResponse(r.data); }
export interface SendMessageRequest { content: string; client_message_id?: string; reply_to_message_id?: string }
export async function sendMessage(id: string, data: SendMessageRequest): Promise<ApiResponse<Message>> { const r = await apiClient.post<ApiResponse<Message>>(`/conversations/${encodeURIComponent(id)}/messages`, data); return toApiResponse(r.data); }
export async function editMessage(id: string, content: string): Promise<ApiResponse<{ id: string; content: string }>> { const r = await apiClient.patch<ApiResponse<{ id: string; content: string }>>(`/messages/${encodeURIComponent(id)}`, { content }); return toApiResponse(r.data); }
export async function deleteMessage(id: string): Promise<ApiResponse<{ id: string; status: string }>> { const r = await apiClient.delete<ApiResponse<{ id: string; status: string }>>(`/messages/${encodeURIComponent(id)}`); return toApiResponse(r.data); }
export async function markDelivery(id: string, status: 'delivered' | 'read'): Promise<ApiResponse<{ status: string }>> { const r = await apiClient.post<ApiResponse<{ status: string }>>('/messages/delivery', { message_id: id, status }); return toApiResponse(r.data); }
export async function markConversationRead(id: string, messageId: string): Promise<ApiResponse<{ status: string }>> { const r = await apiClient.post<ApiResponse<{ status: string }>>(`/conversations/${encodeURIComponent(id)}/read`, { message_id: messageId }); return toApiResponse(r.data); }
export async function getUnread(): Promise<ApiResponse<UnreadResponse>> { const r = await apiClient.get<ApiResponse<UnreadResponse>>('/unread'); return toApiResponse(r.data); }
