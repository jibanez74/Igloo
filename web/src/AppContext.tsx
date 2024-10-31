import { useState, createContext, useEffect, useContext } from "react";
import Spinner from "./components/Spinner";
import type { User } from "@/types/User";

type AppContextType = {
  user: User | null;
  setUser: React.Dispatch<React.SetStateAction<User | null>>;
  error?: string;
  setError?: React.Dispatch<React.SetStateAction<string>>;
  moviesPage: number;
  setMoviesPage: React.Dispatch<React.SetStateAction<number>>;
};

const AppContext = createContext<AppContextType | undefined>(undefined);

export function useAppContext(): AppContextType {
  const context = useContext(AppContext);

  if (context === undefined) {
    throw new Error("useAppContext must be used within an AppContextProvider");
  }

  return context;
}

export default function AppContextProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [moviesPage, setMoviesPage] = useState(1);

  const getAuthUser = async () => {
    const res = await fetch("/api/v1/auth/users/me", {
      method: "get",
      credentials: "include",
      headers: {
        "Content-Type": "application/json",
      },
    });

    if (!res.ok) {
      setLoading(false);
      return;
    }

    const r = await res.json();

    setUser(r.user);
    setLoading(false);
  };

  useEffect(() => {
    getAuthUser();
  }, []);

  const contextValue: AppContextType = {
    user,
    setUser,
    moviesPage,
    setMoviesPage,
  };

  return (
    <AppContext.Provider value={contextValue}>
      {loading ? <Spinner /> : children}
    </AppContext.Provider>
  );
}
