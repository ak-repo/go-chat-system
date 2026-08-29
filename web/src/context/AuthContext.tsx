/* eslint-disable react-refresh/only-export-components */
import { createContext, useContext, useState } from 'react';
import type { ReactNode } from 'react';
import { login as apiLogin, logout as apiLogout, register, clearLocalAuth, updateProfile } from '../api/auth';
import type { User, LoginRequest, RegisterRequest } from '../api/auth';
import {
  setStoredUser,
  getStoredUser,
} from '../api/auth';
import { getToken } from '../api/client';
import wsClient from '../api/websocket';

interface AuthContextType {
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  login: (data: LoginRequest) => Promise<{ success: boolean; error?: string }>;
  logout: () => Promise<void>;
  registerUser: (data: RegisterRequest) => Promise<{ success: boolean; error?: string }>;
  updateUser: (data: { username: string; email: string }) => Promise<{ success: boolean; error?: string }>;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(() => {
    const token = getToken();
    const storedUser = getStoredUser();
    return token && storedUser ? storedUser : null;
  });
  const isLoading = false;

  const login = async (data: LoginRequest): Promise<{ success: boolean; error?: string }> => {
    try {
      const response = await apiLogin(data);
      if (response.success && response.data) {
        setUser(response.data.user);
        setStoredUser(response.data.user);
        return { success: true };
      }
      return { success: false, error: response.error || 'Login failed' };
    } catch (error) {
      return { success: false, error: (error as Error).message || 'Login failed' };
    }
  };

  const logout = async (): Promise<void> => {
    // Stop reconnects before revoking/clearing credentials.
    wsClient.disconnect();
    try { await apiLogout(); } catch { /* local logout must still complete */ }
    clearLocalAuth();
    setUser(null);
  };

  const registerUser = async (data: RegisterRequest): Promise<{ success: boolean; error?: string }> => {
    try {
      const response = await register(data);
      if (response.success && response.data) {
        // Auto-login after registration
        setUser(response.data.user);
        setStoredUser(response.data.user);
        return { success: true };
      }
      return { success: false, error: response.error || 'Registration failed' };
    } catch (error) {
      return { success: false, error: (error as Error).message || 'Registration failed' };
    }
  };

  const updateUser = async (data: { username: string; email: string }) => {
    try {
      const response = await updateProfile(data);
      if (response.success && response.data) {
        setUser(response.data);
        setStoredUser(response.data);
        return { success: true };
      }
      return { success: false, error: response.error || 'Profile update failed' };
    } catch (error) {
      return { success: false, error: error instanceof Error ? error.message : 'Profile update failed' };
    }
  };

  const value: AuthContextType = {
    user,
    isAuthenticated: !!user,
    isLoading,
    login,
    logout,
    registerUser,
    updateUser,
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextType {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}

export default AuthContext;
