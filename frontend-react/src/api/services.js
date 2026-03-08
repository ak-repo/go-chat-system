import { get, post } from "./api";

// User services
export const loginService = (email, password) => {
  return post("/auth/login", { email, password });
};

export const registerService = (username, email, password) => {
  return post("/auth/register", { username, email, password });
};

// Chat services
export const getChatsService = () => get("/chats");

export const getOrCreateDMChatService = (otherUserId) =>
  post("/chats", { other_user_id: otherUserId });

export const getChatMessagesService = (chatId, params = {}) => {
  const sp = new URLSearchParams(params);
  return get(`/chats/${chatId}/messages?${sp}`);
};
