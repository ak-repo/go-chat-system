import apiClient, { toApiResponse, type ApiResponse } from './client';

export type NotificationType = 'message' | 'reply' | 'mention';
export interface Notification { id: string; recipient_id: string; actor_id: string; type: NotificationType; conversation_id: string; message_id: string; payload: { content?: string }; created_at: string; read_at?: string }
export interface NotificationPreferences { message_enabled: boolean; reply_enabled: boolean; mention_enabled: boolean }
export async function listNotifications(limit = 30, offset = 0): Promise<ApiResponse<{ notifications: Notification[]; limit: number; offset: number }>> { const response = await apiClient.get('/notifications', { params: { limit, offset } }); return toApiResponse(response.data); }
export async function getNotificationUnread(): Promise<ApiResponse<{ unread: number }>> { const response = await apiClient.get('/notifications/unread'); return toApiResponse(response.data); }
export async function markNotificationRead(id: string): Promise<ApiResponse<Notification>> { const response = await apiClient.post(`/notifications/${encodeURIComponent(id)}/read`); return toApiResponse(response.data); }
export async function markAllNotificationsRead(): Promise<ApiResponse<{ updated: number }>> { const response = await apiClient.post('/notifications/read-all'); return toApiResponse(response.data); }
export async function getNotificationPreferences(): Promise<ApiResponse<NotificationPreferences>> { const response = await apiClient.get('/notification-preferences'); return toApiResponse(response.data); }
export async function updateNotificationPreferences(patch: Partial<NotificationPreferences>): Promise<ApiResponse<NotificationPreferences>> { const response = await apiClient.patch('/notification-preferences', patch); return toApiResponse(response.data); }
