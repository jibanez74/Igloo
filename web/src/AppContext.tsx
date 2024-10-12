import { useState, createContext, useEffect, useContext } from "react";
import type { User } from "@/types/User";

type AppContextType = {
  user: User | null;
  setUser: React.Dispatch<React.SetStateAction<User | null>>;
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
  const [moviesPage, setMoviesPage] = useState(1);

  useEffect(() => {
    console.log("app is loading");

    setLoading(false);
  }, []);

  const contextValue: AppContextType = {
    user,
    setUser,
    moviesPage,
    setMoviesPage,
  };

  return (
    <AppContext.Provider value={contextValue}>
      {!loading && children}
    </AppContext.Provider>
  );
}
