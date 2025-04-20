import Constants from "expo-constants";

let API_URL =
  Constants.expoConfig?.extra?.apiUrl ||
  Constants.expoConfig?.extra?.EXPO_PUBLIC_API_URL;

if (!API_URL) {
  API_URL = "http://127.0.0.1:8080";
}

export default API_URL;
