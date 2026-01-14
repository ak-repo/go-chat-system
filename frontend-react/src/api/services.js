import { post } from "./api";


// User services
export const loginService = (email, password) => {
  return post("/login", { email, password });
};

export const registerService = (username, email, password) => {
  return post("/register", { username, email, password });
};
