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
  exp: number;
  refresh_token?: string;
  refresh_exp?: number;
}

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
export function logout(): void {
  clearTokens();
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
