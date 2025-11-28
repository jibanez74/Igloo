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

    return res.json();
  } catch (err) {
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
