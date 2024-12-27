import axios from "axios";

const api = axios.create({
  baseURL: "/api/v1",
  withCredentials: true,
  headers: {
    "Content-Type": "application/json",
    Accept: "application/json",
    "X-CSRF-Token":
      document
        .querySelector('meta[name="csrf-token"]')
        ?.getAttribute("content") || "",
  },
});

export default api;
