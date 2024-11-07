import axios from "axios";
import type { Axios } from "axios";

const api: Axios = axios.create({
  baseURL: "https://swifty.hare-crocodile.ts.net/api/v1",
});

export default api;
