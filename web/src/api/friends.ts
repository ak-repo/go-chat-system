import apiClient, { toApiResponse } from './client';
import type { ApiResponse } from './client';

// Friend list response
export interface FriendsResponse {
  friends: Friend[];
  limit: number;
  offset: number;
}

export interface Friend {
  UserID: string;
  FriendID: string;
  FriendName: string;
  FriendEmail: string;
  created_at: string;
}

// Friend request types
// Backend returns PascalCase fields (ID, SenderID, etc.)
export interface FriendRequest {
  ID: string;
  SenderID: string;
  ReceiverID: string;
  FriendName: string;
  FriendEmail: string;
  Status: string;
  created_at: string;
  modified_at?: string;
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
