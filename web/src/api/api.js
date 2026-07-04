import axios from "axios";

const TOKEN_KEY = "token";

export const getToken = () => localStorage.getItem(TOKEN_KEY);
export const setToken = (token) => localStorage.setItem(TOKEN_KEY, token);
export const removeToken = () => localStorage.removeItem(TOKEN_KEY);

// Axios Instance
const api = axios.create({
  baseURL: "http://localhost:8002/api/v1",
  withCredentials: true,
  headers: {
    "Content-Type": "application/json",
  },
});

api.interceptors.request.use((config) => {
  const token = getToken();
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

/**
 * ----------------------------------------------------
 * GENERIC API HELPERS
 * ----------------------------------------------------
 * - Transport only
 * - Always return data
 * - Always throw on failure
 */

export const get = async (url, config = {}) => {
  const res = await api.get(url, config);
  return res.data;
};

export const post = async (url, body, config = {}) => {
  const res = await api.post(url, body, config);
  return res.data;
};

export const patch = async (url, body, config = {}) => {
  const res = await api.patch(url, body, config);
  return res.data;
};
export const put = async (url, body, config = {}) => {
  const res = await api.put(url, body, config);
  return res.data;
};

export const del = async (url, config = {}) => {
  const res = await api.delete(url, config);
  return res.data;
};

export default api;
