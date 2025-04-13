import axios from "axios";
import API_URL from "@/constants/Backend";

const api = axios.create({
  baseURL: `${API_URL}/api/v1`,
});

export default api;
