import {
  LOGIN_API_ROUTE,
  AUTH_API_ROUTE,
  NETWORK_ERROR,
  ERROR_NOTFOUND,
} from "@/lib/constants";

export async function login(email: string, password: string) {
  try {
    const res = await fetch(LOGIN_API_ROUTE, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      credentials: "include",
      body: JSON.stringify({ email, password }),
    });

    if (res.status === 404) {
      return ERROR_NOTFOUND;
    }

    // Try to parse JSON, but handle cases where response might be empty or invalid
    let data;
    try {
      const text = await res.text();
      data = text ? JSON.parse(text) : {};
    } catch {
      // If JSON parsing fails, return error based on HTTP status
      return {
        error: true,
        message: `HTTP ${res.status}: ${res.statusText}`,
      };
    }

    // The API always returns { error: boolean, message?: string, data?: any }
    // If error field is not present but status is not ok, treat as error
    if (data.error === undefined && !res.ok) {
      return {
        error: true,
        message: data.message || `HTTP ${res.status}: ${res.statusText}`,
        data: data.data,
      };
    }

    return {
      error: data.error ?? false,
      message: data.message || "",
      data: data.data,
    };
  } catch (err) {
    // Only catch actual network errors (fetch failures, not HTTP errors)
    console.error(err);
    return NETWORK_ERROR;
  }
}

export async function getAuthUser() {
  try {
    const res = await fetch(AUTH_API_ROUTE, {
      credentials: "include",
    });

    if (res.status === 404) {
      return ERROR_NOTFOUND;
    }

    return res.json();
  } catch (err) {
    console.error(err);
    return NETWORK_ERROR;
  }
}
