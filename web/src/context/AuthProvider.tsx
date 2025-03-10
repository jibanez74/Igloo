import { useState, useEffect, type ReactNode } from "react";
import queryClient from "../utils/queryClient";
import LoadingScreen from "../components/LoadingScreen";
import AuthContext from "./AuthContext";
import type { User } from "../types/User";
import type { AuthResponse } from "../types/Auth";

export default function AuthProvider({ children }: { children: ReactNode }) {
  const [accessToken, setAccessToken] = useState<string | null>(null);
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

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

  useEffect(() => {
    setLoading(false);
  }, []);

  return (
    <AuthContext.Provider
      value={{
        user,
        isAuthenticated: !!accessToken,
        login,
        logout,
      }}
    >
      {loading ? <LoadingScreen text='Checking authentication...' /> : children}
    </AuthContext.Provider>
  );
}
