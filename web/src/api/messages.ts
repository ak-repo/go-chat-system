import apiClient, { toApiResponse } from './client';
import type { ApiResponse } from './client';

// Message type
export interface Message {
  id: string;
  sender_id: string;
  receiver_id: string;
  content: string;
  is_group: boolean;
  created_at: string;
  modified_at: string;
  deleted_at?: { Time: string; Valid: boolean } | null;
}

// Messages response
export interface MessagesResponse {
  messages: Message[];
  limit: number;
  offset: number;
}

function getMessageTimestamp(message: Message): number {
  const timestamp = Date.parse(message.created_at);
  return Number.isNaN(timestamp) ? 0 : timestamp;
}

export function compareMessagesChronologically(a: Message, b: Message): number {
  const byCreatedAt = getMessageTimestamp(a) - getMessageTimestamp(b);
  if (byCreatedAt !== 0) return byCreatedAt;

  return a.id.localeCompare(b.id);
}

export function sortMessagesChronologically(messages: Message[]): Message[] {
  return [...messages].sort(compareMessagesChronologically);
}

export function mergeMessagesChronologically(
  currentMessages: Message[],
  nextMessage: Message
): Message[] {
  if (currentMessages.some((message) => message.id === nextMessage.id)) {
    return currentMessages;
  }

  return sortMessagesChronologically([...currentMessages, nextMessage]);
}

// Get messages between current user and another user
export async function getMessages(
  otherUserId: string,
  limit: number = 50,
  offset: number = 0
): Promise<ApiResponse<MessagesResponse>> {
  const response = await apiClient.get<ApiResponse<MessagesResponse>>(
    `/messages?user_id=${otherUserId}&limit=${limit}&offset=${offset}`
  );
  return toApiResponse(response.data);
}
