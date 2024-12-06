import axios from "axios";
import Constants from "expo-constants";

const apiUrl =
  Constants.expoConfig?.extra?.apiUrl ||
  Constants.expoConfig?.extra?.EXPO_PUBLIC_API_URL;

if (!apiUrl) {
  throw new Error(
    "API URL is not configured. Please check your app configuration."
  );
}

const api = axios.create({
  baseURL: apiUrl,
});

export default api;
