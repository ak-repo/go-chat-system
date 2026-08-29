import apiClient, {
  setToken,
  setRefreshToken,
  clearTokens,
  toApiResponse,
} from './client';
import type { ApiResponse } from './client';

// Types
export interface User {
  id: string;
  username: string;
  email: string;
  role: string;
}

export interface AuthResponse {
  user: User;
  token: string;
  exp: string;
  refresh_token?: string;
  refresh_exp?: string;
}

export type TokenResponse = Omit<AuthResponse, 'user'>;

export interface RegisterRequest {
  username: string;
  email: string;
  password: string;
}

export interface LoginRequest {
  email: string;
  password: string;
}

// Register new user
export async function register(
  data: RegisterRequest
): Promise<ApiResponse<AuthResponse>> {
  const response = await apiClient.post<ApiResponse<AuthResponse>>(
    '/auth/register',
    data
  );
  const apiResponse = toApiResponse(response.data);

  if (apiResponse.success && apiResponse.data) {
    setToken(apiResponse.data.token, apiResponse.data.exp);
    if (apiResponse.data.refresh_token) {
      setRefreshToken(apiResponse.data.refresh_token);
    }
  }

  return apiResponse;
}

// Login user
export async function login(
  data: LoginRequest
): Promise<ApiResponse<AuthResponse>> {
  const response = await apiClient.post<ApiResponse<AuthResponse>>(
    '/auth/login',
    data
  );

  const apiResponse = toApiResponse(response.data);

  if (apiResponse.success && apiResponse.data) {
    setToken(apiResponse.data.token, apiResponse.data.exp);
    if (apiResponse.data.refresh_token) {
      setRefreshToken(apiResponse.data.refresh_token);
    }
  }

  return apiResponse;
}

// Refresh token
export async function refreshToken(
  refreshToken: string
): Promise<ApiResponse<AuthResponse>> {
  const response = await apiClient.post<ApiResponse<AuthResponse>>(
    '/auth/refresh',
    { refresh_token: refreshToken }
  );

  const apiResponse = toApiResponse(response.data);

  if (apiResponse.success && apiResponse.data) {
    setToken(apiResponse.data.token, apiResponse.data.exp);
    if (apiResponse.data.refresh_token) {
      setRefreshToken(apiResponse.data.refresh_token);
    }
  }

  return apiResponse;
}

// Logout - clear tokens
export async function logout(): Promise<ApiResponse<{ user_id: string }>> {
  const refresh_token = localStorage.getItem('refresh_token');
  const response = await apiClient.post<ApiResponse<{ user_id: string }>>('/auth/logout', { refresh_token });
  return toApiResponse(response.data);
}

export function clearLocalAuth(): void {
  clearTokens();
  clearStoredUser();
}

export interface ProfileUpdate { username: string; email: string }
export interface PasswordChange { password: string }
export interface RecoveryRequest { email: string }
export interface RecoveryConfirm { token: string; password: string }

export async function getProfile(): Promise<ApiResponse<User>> {
  const response = await apiClient.get<ApiResponse<User>>('/users/me');
  return toApiResponse(response.data);
}
export async function updateProfile(data: ProfileUpdate): Promise<ApiResponse<User>> {
  const response = await apiClient.patch<ApiResponse<User>>('/users/me', data);
  return toApiResponse(response.data);
}
export async function changePassword(data: PasswordChange): Promise<ApiResponse<{ status: string }>> {
  const response = await apiClient.post<ApiResponse<{ status: string }>>('/users/me/change-password', data);
  return toApiResponse(response.data);
}
export async function deactivateAccount(): Promise<ApiResponse<{ status: string }>> {
  const response = await apiClient.delete<ApiResponse<{ status: string }>>('/users/me');
  return toApiResponse(response.data);
}
export async function requestPasswordReset(data: RecoveryRequest): Promise<ApiResponse<{ message: string }>> {
  const response = await apiClient.post<ApiResponse<{ message: string }>>('/auth/password-reset/request', data);
  return toApiResponse(response.data);
}
export async function confirmPasswordReset(data: RecoveryConfirm): Promise<ApiResponse<null>> {
  const response = await apiClient.post<ApiResponse<null>>('/auth/password-reset/confirm', data);
  return toApiResponse(response.data);
}
export async function requestVerification(email: string): Promise<ApiResponse<{ message: string }>> {
  const response = await apiClient.post<ApiResponse<{ message: string }>>('/auth/verification/request', { email });
  return toApiResponse(response.data);
}
export async function confirmVerification(token: string): Promise<ApiResponse<null>> {
  const response = await apiClient.post<ApiResponse<null>>('/auth/verification/confirm', { token });
  return toApiResponse(response.data);
}

// Get current user from stored token data
export function getStoredUser(): User | null {
  const userData = localStorage.getItem('user');
  if (!userData) return null;
  try {
    return JSON.parse(userData);
  } catch {
    return null;
  }
}

// Store user data
export function setStoredUser(user: User): void {
  localStorage.setItem('user', JSON.stringify(user));
}

// Clear user data
export function clearStoredUser(): void {
  localStorage.removeItem('user');
}
