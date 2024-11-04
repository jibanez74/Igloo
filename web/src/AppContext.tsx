import { useState, createContext, useEffect, useContext } from "react";
import Spinner from "./components/Spinner";
import type { User } from "@/types/User";

type AppContextType = {
  user: User | null;
  setUser: React.Dispatch<React.SetStateAction<User | null>>;
  jwt: string;
  setJwt: React.Dispatch<React.SetStateAction<string>>;
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
  const [jwt, setJwt] = useState("");

  useEffect(() => {
    setLoading(false);
  }, []);

  const contextValue: AppContextType = {
    user,
    setUser,
    jwt,
    setJwt,
  };

  return (
    <AppContext.Provider value={contextValue}>
      {loading ? <Spinner /> : children}
    </AppContext.Provider>
  );
}
