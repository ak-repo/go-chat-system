import apiClient, { toApiResponse } from './client';
import type { ApiResponse } from './client';

export interface Conversation { id: string; kind: string; user_one_id: string; user_two_id: string; created_at: string; modified_at: string }
export interface ConversationPage { conversations: Conversation[]; limit: number; offset: number }
export async function createOrGetConversation(userId: string): Promise<ApiResponse<Conversation>> { const r = await apiClient.post<ApiResponse<Conversation>>('/conversations', { user_id: userId }); return toApiResponse(r.data); }
export async function listConversations(limit = 50, offset = 0): Promise<ApiResponse<ConversationPage>> { const r = await apiClient.get<ApiResponse<ConversationPage>>('/conversations', { params: { limit, offset } }); return toApiResponse(r.data); }
export async function getConversation(id: string): Promise<ApiResponse<Conversation>> { const r = await apiClient.get<ApiResponse<Conversation>>(`/conversations/${encodeURIComponent(id)}`); return toApiResponse(r.data); }
