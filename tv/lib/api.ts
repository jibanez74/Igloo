import axios from "axios";

const BASE_URL = "https://swifty.hare-crocodile.ts.net/api/v1";

const api = axios.create({
  baseURL: BASE_URL,
});

export default api;
