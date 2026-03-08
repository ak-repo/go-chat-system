import axios from "axios";

// Axios Instance
const api = axios.create({
  baseURL: "http://localhost:8002/api/v1",
  withCredentials: true,
  headers: {
    "Content-Type": "application/json",
  },
});

// Token for authenticated requests (set by AuthContext on login/load, cleared on logout)
let authToken = null;

export function setAuthToken(token) {
  authToken = token;
}

export function getAuthToken() {
  return authToken;
}

api.interceptors.request.use((config) => {
  if (authToken) {
    config.headers.Authorization = `Bearer ${authToken}`;
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
