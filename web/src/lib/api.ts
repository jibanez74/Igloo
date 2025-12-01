import type { ApiFailureType, ApiResponseType, SimpleAlbumType } from "@/types";

const ERROR_NOTFOUND: ApiFailureType = {
  error: true,
  message: "404 - The resource you requested was not found.",
};

const NETWORK_ERROR: ApiFailureType = {
  error: true,
  message: "500 - A network error occurred while processing your request.",
};

export async function login(email: string, password: string) {
  try {
    const res = await fetch("/api/auth/login", {
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
    const res = await fetch("/api/auth/user", {
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

export async function getLatestAlbums(): Promise<
  ApiResponseType<{ albums: SimpleAlbumType[] }>
> {
  try {
    const res = await fetch("/api/albums/latest", {
      credentials: "include",
    });

    if (res.status === 404) {
      return ERROR_NOTFOUND;
    }

    const data: ApiResponseType<{ albums: SimpleAlbumType[] }> =
      await res.json();

    return data;
  } catch (err) {
    console.error(err);
    return NETWORK_ERROR;
  }
}
