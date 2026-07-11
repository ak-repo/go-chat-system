import apiClient, { toApiResponse } from './client';
import type { ApiResponse } from './client';
import type { User } from './auth';

// Friend list response
export interface FriendsResponse {
  friends: Friend[];
  limit: number;
  offset: number;
}

export interface Friend {
  id: string;
  user_id: string;
  friend_id: string;
  created_at: string;
  friend?: User;
}

// Friend request types
export interface FriendRequest {
  id: string;
  sender_id: string;
  receiver_id: string;
  status: 'pending' | 'accepted' | 'rejected' | 'cancelled';
  created_at: string;
  modified_at: string;
  sender?: User;
  receiver?: User;
}

export interface FriendRequestsResponse {
  requests: FriendRequest[];
}

// List friends
export async function listFriends(
  limit: number = 20,
  offset: number = 0
): Promise<ApiResponse<FriendsResponse>> {
  const response = await apiClient.get<ApiResponse<FriendsResponse>>(
    `/friends?limit=${limit}&offset=${offset}`
  );
  return toApiResponse(response.data);
}

// Get all friend requests
export async function getFriendRequests(): Promise<
  ApiResponse<FriendRequestsResponse>
> {
  const response = await apiClient.get<ApiResponse<FriendRequestsResponse>>(
    '/friend-requests'
  );
  return toApiResponse(response.data);
}

// Create friend request
export async function createFriendRequest(
  toUserId: string
): Promise<ApiResponse<null>> {
  const response = await apiClient.post<ApiResponse<null>>('/friend-requests', {
    to: toUserId,
  });
  return toApiResponse(response.data);
}

// Accept friend request
export async function acceptFriendRequest(
  requestId: string,
  receiverId: string
): Promise<ApiResponse<null>> {
  const response = await apiClient.post<ApiResponse<null>>(
    '/friend-requests/accept',
    {
      request_id: requestId,
      received_id: receiverId,
    }
  );
  return toApiResponse(response.data);
}

// Reject friend request
export async function rejectFriendRequest(
  requestId: string,
  receiverId: string
): Promise<ApiResponse<null>> {
  const response = await apiClient.post<ApiResponse<null>>(
    '/friend-requests/reject',
    {
      request_id: requestId,
      receiver_id: receiverId,
    }
  );
  return toApiResponse(response.data);
}

// Cancel friend request
export async function cancelFriendRequest(
  requestId: string
): Promise<ApiResponse<null>> {
  const response = await apiClient.post<ApiResponse<null>>(
    '/friend-requests/cancel',
    {
      request_id: requestId,
    }
  );
  return toApiResponse(response.data);
}

// Block user
export async function blockUser(userId: string): Promise<ApiResponse<null>> {
  const response = await apiClient.post<ApiResponse<null>>('/blocks', {
    user_id: userId,
  });
  return toApiResponse(response.data);
}

// Unblock user
export async function unblockUser(userId: string): Promise<ApiResponse<null>> {
  const response = await apiClient.post<ApiResponse<null>>('/blocks/unblock', {
    user_id: userId,
  });
  return toApiResponse(response.data);
}
