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

api.interceptors.response.use(
  res => res,
  err => {
    if (err.response?.status === 401) {
      window.location.href = "/login";
    }

    return Promise.reject(err);
  }
);

export default api;
