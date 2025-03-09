import { useState, useEffect, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import queryClient from "../utils/queryClient";
import LoadingScreen from "../components/LoadingScreen";
import AuthContext from "./AuthContext";
import type { User } from "../types/User";
import type { AuthResponse } from "../types/Auth";

async function refreshAuth(): Promise<AuthResponse> {
  const refreshToken = localStorage.getItem("refresh_token");
  if (!refreshToken) {
    throw new Error("No refresh token available");
  }

  const res = await fetch("/api/v1/auth/refresh", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ refresh_token: refreshToken }),
  });

  if (!res.ok) {
    throw new Error("Failed to refresh authentication");
  }

  return res.json();
}

export default function AuthProvider({ children }: { children: ReactNode }) {
  const [accessToken, setAccessToken] = useState<string | null>(null);
  const [user, setUser] = useState<User | null>(null);

  const { isPending, data, error } = useQuery({
    queryKey: ["auth"],
    queryFn: refreshAuth,
    retry: false,
    enabled: !!localStorage.getItem("refresh_token"),
    refetchInterval: 1000 * 60 * 4,
    refetchIntervalInBackground: true,
  });

  useEffect(() => {
    if (data) {
      setAccessToken(data.tokens.access_token);
      setUser(data.user);
      localStorage.setItem("refresh_token", data.tokens.refresh_token);
    }
  }, [data]);

  useEffect(() => {
    if (error) {
      setAccessToken(null);
      setUser(null);
      localStorage.removeItem("refresh_token");
    }
  }, [error]);

  const login = (response: AuthResponse) => {
    setAccessToken(response.tokens.access_token);
    setUser(response.user);
    localStorage.setItem("refresh_token", response.tokens.refresh_token);
    queryClient.invalidateQueries({ queryKey: ["auth"] });
  };

  const logout = () => {
    setAccessToken(null);
    setUser(null);
    localStorage.removeItem("refresh_token");
    queryClient.clear();
  };

  return (
    <AuthContext.Provider
      value={{
        user,
        isAuthenticated: !!accessToken,
        login,
        logout,
      }}
    >
      {isPending ? (
        <LoadingScreen text='Checking authentication...' />
      ) : (
        children
      )}
    </AuthContext.Provider>
  );
}
