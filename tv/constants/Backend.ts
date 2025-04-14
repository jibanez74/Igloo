import Constants from "expo-constants";

const API_URL = Constants.expoConfig?.extra?.EXPO_PUBLIC_API_URL;

if (!API_URL) {
  throw new Error("API_URL is not defined in app.json");
}

export default API_URL;
