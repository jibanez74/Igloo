import { fetch } from "expo/fetch";

const BASE_URL = "https://swifty.hare-crocodile.ts.net/api/v1";

type RequestConfig = {
  headers?: Record<string, string>;
};

class Api {
  private token: string | null = null;

  setToken(token: string | null) {
    this.token = token;
  }

  private getHeaders(): HeadersInit {
    const headers: HeadersInit = {
      "Content-Type": "application/json",
    };

    if (this.token) {
      headers["Authorization"] = `Bearer ${this.token}`;
    }

    return headers;
  }

  async get<T>(path: string, config?: RequestConfig): Promise<T> {
    const response = await fetch(`${BASE_URL}${path}`, {
      method: "GET",
      headers: {
        ...this.getHeaders(),
        ...config?.headers,
      },
    });

    if (!response.ok) {
      const error = new Error(`API Error: ${response.status}`);
      error.cause = await response.json().catch(() => null);
      throw error;
    }

    return response.json();
  }

  async post<T>(
    path: string,
    data: unknown,
    config?: RequestConfig
  ): Promise<T> {
    const response = await fetch(`${BASE_URL}${path}`, {
      method: "POST",
      headers: {
        ...this.getHeaders(),
        ...config?.headers,
      },
      body: JSON.stringify(data),
    });

    if (!response.ok) {
      const error = new Error(`API Error: ${response.status}`);
      error.cause = await response.json().catch(() => null);
      throw error;
    }

    return response.json();
  }
}

const api = new Api();

export default api;
