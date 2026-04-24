import { post, get } from "./api";


// User services
export const loginService = (email, password) => {
  return post("/auth/login", { email, password });
};

export const registerService = (username, email, password) => {
  return post("/auth/register", { username, email, password });
};

export const getFriends = () => {
  return get("/friends");
};

export const getMessages = (userId, limit = 50, offset = 0) => {
  return get(`/messages?user_id=${userId}&limit=${limit}&offset=${offset}`);
};
