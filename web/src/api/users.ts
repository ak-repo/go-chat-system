import apiClient, { toApiResponse } from './client';
import type { ApiResponse } from './client';
import type { User } from './auth';

// Search users response
export interface SearchUsersResponse {
  users: User[];
}

// Search users
export async function searchUsers(
  filter?: string,
  limit: number = 20
): Promise<ApiResponse<SearchUsersResponse>> {
  const params = new URLSearchParams();
  if (filter) params.append('filter', filter);
  if (limit) params.append('limit', limit.toString());

  const response = await apiClient.get<ApiResponse<SearchUsersResponse>>(
    `/users?${params.toString()}`
  );
  return toApiResponse(response.data);
}
