import apiClient, { type ApiResponse } from './client';

export interface Notification {
  id: string;
  user_id: string;
  type: string;
  title: string;
  body?: string;
  sender_id?: string;
  reference_id?: string;
  is_read: boolean;
  created_at: string;
}

export interface NotificationsResponse {
  notifications: Notification[];
  unread_count: number;
  limit: number;
  offset: number;
}

// GET /notifications - List notifications
export async function getNotifications(limit = 20, offset = 0): Promise<ApiResponse<NotificationsResponse>> {
  const response = await apiClient.get('/notifications', { params: { limit, offset } });
  return response.data;
}

// POST /notifications/read - Mark as read
export async function markNotificationAsRead(notificationId: string): Promise<ApiResponse<void>> {
  const response = await apiClient.post('/notifications/read', { notification_id: notificationId });
  return response.data;
}

// POST /notifications/read-all - Mark all as read
export async function markAllNotificationsAsRead(): Promise<ApiResponse<void>> {
  const response = await apiClient.post('/notifications/read-all');
  return response.data;
}

// DELETE /notifications/ - Delete notification
export async function deleteNotification(notificationId: string): Promise<ApiResponse<void>> {
  const response = await apiClient.delete('/notifications/', { data: { notification_id: notificationId } });
  return response.data;
}