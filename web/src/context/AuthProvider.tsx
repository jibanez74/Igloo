import { useState, useEffect, type ReactNode } from "react";
import LoadingScreen from "../components/LoadingScreen";
import { AuthContext } from "./AuthContext";
import type { User } from "../types/User";

export default function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    const checkAuth = async () => {
      try {
        const hasSession = localStorage.getItem("hasSession") === "true";

        if (hasSession) {
          const res = await fetch("/api/v1/auth/me", {
            credentials: "include",
          });

          if (res.ok) {
            const data = await res.json();

            setUser(data);
          } else {
            localStorage.removeItem("hasSession");
          }
        }
      } catch (error) {
        console.error("Auth check failed:", error);
        localStorage.removeItem("hasSession");
      } finally {
        setIsLoading(false);
      }
    };

    checkAuth();
  }, []);

  useEffect(() => {
    if (user) {
      localStorage.setItem("hasSession", "true");
    } else {
      localStorage.removeItem("hasSession");
    }
  }, [user]);

  const value = {
    user,
    setUser,
  };

  return (
    <AuthContext.Provider value={value}>
      {isLoading ? <LoadingScreen /> : children}
    </AuthContext.Provider>
  );
}
