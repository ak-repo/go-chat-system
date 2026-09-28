import apiClient, { toApiResponse } from './client';
import type { ApiResponse } from './client';

export type MessageStatus = 'sending' | 'sent' | 'delivered' | 'read' | 'failed';
export interface MessagePreview { id: string; sender_id: string; content: string }
export interface ReactionAggregate { reaction: string; count: number; reacted_by_me: boolean }
export interface MessageMention { kind: 'user' | 'everyone'; user_id?: string; offset: number; length: number }
export interface Message { id: string; sender_id: string; receiver_id: string; content: string; is_group: boolean; created_at: string; modified_at: string; deleted_at?: { Time: string; Valid: boolean } | null; conversation_id?: string; client_message_id?: string; edited_at?: string; reply_to_message_id?: string; reply_to?: MessagePreview; forwarded_from_message_id?: string; forwarded_from?: MessagePreview; reactions?: ReactionAggregate[]; mentions?: MessageMention[]; status?: MessageStatus }
export interface MessagesResponse { messages: Message[]; limit: number; offset: number; conversation_id?: string }
export type UnreadResponse = { unread: Record<string, number> };
const statusRank: Record<MessageStatus, number> = { sending: 0, failed: 0, sent: 1, delivered: 2, read: 3 };
export function mergeMessageStatus(current?: MessageStatus, next?: MessageStatus): MessageStatus | undefined { if (!next) return current; if (current && statusRank[current] > statusRank[next]) return current; return next; }
function timestamp(m: Message): number { const value = Date.parse(m.created_at); return Number.isNaN(value) ? 0 : value; }
export function compareMessagesChronologically(a: Message, b: Message): number { return timestamp(a) - timestamp(b) || a.id.localeCompare(b.id); }
export function sortMessagesChronologically(messages: Message[]): Message[] { return [...messages].sort(compareMessagesChronologically); }
export function mergeMessagesChronologically(current: Message[], next: Message): Message[] {
  const index = current.findIndex((m) => m.id === next.id || (!!next.client_message_id && m.client_message_id === next.client_message_id));
  if (index >= 0) { const result = [...current]; result[index] = { ...result[index], ...next, status: mergeMessageStatus(result[index].status, next.status) }; return sortMessagesChronologically(result); }
  return sortMessagesChronologically([...current, next]);
}
export function mergeMessagePages(current: Message[], incoming: Message[]): Message[] {
  return incoming.reduce(mergeMessagesChronologically, current);
}
export function optimisticReaction(reactions: ReactionAggregate[] = [], reaction: string, add: boolean): ReactionAggregate[] {
  const existing = reactions.find((item) => item.reaction === reaction);
  if (add) {
    if (existing?.reacted_by_me) return reactions;
    return existing ? reactions.map((item) => item.reaction === reaction ? { ...item, count: item.count + 1, reacted_by_me: true } : item) : [...reactions, { reaction, count: 1, reacted_by_me: true }];
  }
  if (!existing?.reacted_by_me) return reactions;
  return existing.count === 1 ? reactions.filter((item) => item.reaction !== reaction) : reactions.map((item) => item.reaction === reaction ? { ...item, count: item.count - 1, reacted_by_me: false } : item);
}
export async function getMessages(otherUserId: string, limit = 50, offset = 0): Promise<ApiResponse<MessagesResponse>> { const r = await apiClient.get<ApiResponse<MessagesResponse>>('/messages', { params: { user_id: otherUserId, limit, offset } }); return toApiResponse(r.data); }
export async function getConversationMessages(id: string, limit = 50, offset = 0): Promise<ApiResponse<MessagesResponse>> { const r = await apiClient.get<ApiResponse<MessagesResponse>>(`/conversations/${encodeURIComponent(id)}/messages`, { params: { limit, offset } }); return toApiResponse(r.data); }
export interface SendMessageRequest { content: string; client_message_id?: string; reply_to_message_id?: string; mentions?: MessageMention[] }
export async function sendMessage(id: string, data: SendMessageRequest): Promise<ApiResponse<Message>> { const r = await apiClient.post<ApiResponse<Message>>(`/conversations/${encodeURIComponent(id)}/messages`, data); return toApiResponse(r.data); }
export async function forwardMessage(conversationId: string, messageId: string, clientMessageId: string): Promise<ApiResponse<Message>> { const r = await apiClient.post<ApiResponse<Message>>(`/conversations/${encodeURIComponent(conversationId)}/messages/${encodeURIComponent(messageId)}/forward`, { client_message_id: clientMessageId }); return toApiResponse(r.data); }
export async function setMessageReaction(messageId: string, reaction: string, add: boolean): Promise<ApiResponse<{ message_id: string; reaction: string; reactions: ReactionAggregate[]; changed: boolean }>> { const path = `/messages/${encodeURIComponent(messageId)}/reactions/${encodeURIComponent(reaction)}`; const r = add ? await apiClient.put(path) : await apiClient.delete(path); return toApiResponse(r.data); }
export async function editMessage(id: string, content: string): Promise<ApiResponse<{ id: string; content: string }>> { const r = await apiClient.patch<ApiResponse<{ id: string; content: string }>>(`/messages/${encodeURIComponent(id)}`, { content }); return toApiResponse(r.data); }
export async function deleteMessage(id: string): Promise<ApiResponse<{ id: string; status: string }>> { const r = await apiClient.delete<ApiResponse<{ id: string; status: string }>>(`/messages/${encodeURIComponent(id)}`); return toApiResponse(r.data); }
export async function markDelivery(id: string, status: 'delivered' | 'read'): Promise<ApiResponse<{ status: string }>> { const r = await apiClient.post<ApiResponse<{ status: string }>>('/messages/delivery', { message_id: id, status }); return toApiResponse(r.data); }
export async function markConversationRead(id: string, messageId: string): Promise<ApiResponse<{ status: string }>> { const r = await apiClient.post<ApiResponse<{ status: string }>>(`/conversations/${encodeURIComponent(id)}/read`, { message_id: messageId }); return toApiResponse(r.data); }
export async function getUnread(): Promise<ApiResponse<UnreadResponse>> { const r = await apiClient.get<ApiResponse<UnreadResponse>>('/unread'); return toApiResponse(r.data); }
export interface ReadAllResponse { status: string; receipts: { conversation_id: string; message_id: string; sender_id: string }[] }
export async function markAllRead(): Promise<ApiResponse<ReadAllResponse>> { const r = await apiClient.post<ApiResponse<ReadAllResponse>>('/unread/read-all'); return toApiResponse(r.data); }
