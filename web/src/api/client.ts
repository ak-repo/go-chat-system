import axios, { type AxiosError, type InternalAxiosRequestConfig } from 'axios';

const configuredOrigin = (import.meta.env.VITE_API_ORIGIN as string | undefined)?.trim();
export const BASE_URL = `${(configuredOrigin || 'http://localhost:8002').replace(/\/$/, '')}/api/v1`;

// Token storage keys
const TOKEN_KEY = 'auth_token';
const REFRESH_TOKEN_KEY = 'refresh_token';
const TOKEN_EXPIRY_KEY = 'token_expiry';

// Get stored tokens
export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}

export function getRefreshToken(): string | null {
  return localStorage.getItem(REFRESH_TOKEN_KEY);
}

export function getTokenExpiry(): number | null {
  const expiry = localStorage.getItem(TOKEN_EXPIRY_KEY);
  return expiry ? parseInt(expiry, 10) : null;
}

// Store tokens
export function setToken(token: string, expiresAt: string): void {
  const expiry = Date.parse(expiresAt);
  if (Number.isNaN(expiry)) {
    clearTokens();
    return;
  }

  localStorage.setItem(TOKEN_KEY, token);
  localStorage.setItem(TOKEN_EXPIRY_KEY, expiry.toString());
}

export function setRefreshToken(token: string): void {
  localStorage.setItem(REFRESH_TOKEN_KEY, token);
}

// Clear tokens (logout)
export function clearTokens(): void {
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(REFRESH_TOKEN_KEY);
  localStorage.removeItem(TOKEN_EXPIRY_KEY);
}

// Check if token needs refresh (5 minutes before expiry)
export function isTokenExpiringSoon(): boolean {
  const expiry = getTokenExpiry();
  if (!expiry) return true;
  return Date.now() > expiry - 5 * 60 * 1000;
}

// API response types
export interface ApiResponse<T> {
  success: boolean;
  data?: T;
  error?: string;
  message?: string;
  status?: string;
  code?: string;
}

export interface BackendResponse<T> {
  status?: string;
  data?: T;
  error?: unknown;
  message?: string;
  code?: string;
}

export function toApiResponse<T>(payload: BackendResponse<T> | ApiResponse<T>): ApiResponse<T> {
  if ('success' in payload) {
    return payload;
  }

  const error =
    payload.message || (typeof payload.error === 'string' ? payload.error : undefined);

  return {
    success: payload.status === 'ok',
    status: payload.status,
    data: payload.data,
    error,
    message: payload.message,
    code: payload.code,
  };
}

// Create axios instance
const apiClient = axios.create({
  baseURL: BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
  timeout: 10000,
});

// Request interceptor - add auth token
apiClient.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    const token = getToken();
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => Promise.reject(error)
);

// Response interceptor - handle 401 for token refresh
let refreshPromise: Promise<string> | null = null;
let failedQueue: Array<{
  resolve: (value: unknown) => void;
  reject: (reason?: unknown) => void;
}> = [];

const processQueue = (error: AxiosError | null, token: string | null = null) => {
  failedQueue.forEach((prom) => {
    if (error) {
      prom.reject(error);
    } else {
      prom.resolve(token);
    }
  });
  failedQueue = [];
};

apiClient.interceptors.response.use(
  (response) => response,
  async (error: AxiosError) => {
    const originalRequest = error.config as InternalAxiosRequestConfig & { _retry?: boolean };

    if (error.response?.status === 401 && !originalRequest._retry && !originalRequest.url?.endsWith('/auth/refresh')) {
      if (refreshPromise) {
        return new Promise((resolve, reject) => {
          failedQueue.push({ resolve, reject });
        })
          .then((token) => {
            originalRequest.headers.Authorization = `Bearer ${token}`;
            return apiClient(originalRequest);
          })
          .catch((err) => Promise.reject(err));
      }

      originalRequest._retry = true;
      const refreshToken = getRefreshToken();
      if (!refreshToken) {
        clearTokens();
        return Promise.reject(error);
      }

      try {
        refreshPromise = axios.post(`${BASE_URL}/auth/refresh`, { refresh_token: refreshToken })
          .then((response) => {
            const data = response.data.data as AuthTokenResponse;
            setToken(data.token, data.exp);
            if (data.refresh_token) setRefreshToken(data.refresh_token);
            processQueue(null, data.token);
            return data.token;
          });
        const token = await refreshPromise;
        originalRequest.headers.Authorization = `Bearer ${token}`;
        return apiClient(originalRequest);
      } catch (refreshError) {
        processQueue(refreshError as AxiosError, null);
        clearTokens();
        window.location.href = '/login';
        return Promise.reject(refreshError);
      } finally {
        refreshPromise = null;
      }
    }

    return Promise.reject(error);
  }
);

export default apiClient;

export interface AuthTokenResponse {
  token: string;
  exp: string;
  refresh_token?: string;
  refresh_exp?: string;
}

/** Shared refresh operation for HTTP and WebSocket lifecycles. */
export async function refreshAccessToken(): Promise<string> {
  if (refreshPromise) return refreshPromise;
  const refreshToken = getRefreshToken();
  if (!refreshToken) throw new Error('No refresh token available');
  refreshPromise = axios.post(`${BASE_URL}/auth/refresh`, { refresh_token: refreshToken })
    .then((response) => {
      const data = response.data.data as AuthTokenResponse;
      setToken(data.token, data.exp);
      if (data.refresh_token) setRefreshToken(data.refresh_token);
      return data.token;
    })
    .finally(() => { refreshPromise = null; });
  return refreshPromise;
}
