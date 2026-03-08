import { useState, useEffect } from "react";
import { AuthContext } from "./context";
import { loginService, registerService } from "../api/services";
import { setAuthToken } from "../api/api";

const USER_KEY = "user";
const TOKEN_KEY = "token";

function AuthProvider({ children }) {
  const [user, setUser] = useState(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    try {
      const savedUser = localStorage.getItem(USER_KEY);
      const savedToken = localStorage.getItem(TOKEN_KEY);
      if (savedUser && savedToken) {
        setUser(JSON.parse(savedUser));
        setAuthToken(savedToken);
      }
    } catch {
      localStorage.removeItem(USER_KEY);
      localStorage.removeItem(TOKEN_KEY);
      setAuthToken(null);
    } finally {
      setLoading(false);
    }
  }, []);

  const login = async (email, password) => {
    setLoading(true);
    try {
      const { data } = await loginService(email, password);
      setUser(data.user);
      if (data.token) {
        setAuthToken(data.token);
        localStorage.setItem(TOKEN_KEY, data.token);
      }
      localStorage.setItem(USER_KEY, JSON.stringify(data.user));
      return true;
    } finally {
      setLoading(false);
    }
  };

  const register = async (username, email, password) => {
    setLoading(true);
    try {
      const { data } = await registerService(username, email, password);
      if (data?.user && data?.token) {
        setUser(data.user);
        setAuthToken(data.token);
        localStorage.setItem(TOKEN_KEY, data.token);
        localStorage.setItem(USER_KEY, JSON.stringify(data.user));
        return true;
      }
      return true;
    } finally {
      setLoading(false);
    }
  };

  const logout = () => {
    setUser(null);
    setAuthToken(null);
    localStorage.removeItem(USER_KEY);
    localStorage.removeItem(TOKEN_KEY);
  };

  return (
    <AuthContext.Provider value={{ user, loading, login, register, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export default AuthProvider;
