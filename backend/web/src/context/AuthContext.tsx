import { useEffect, useState, createContext, useContext } from "react";
import type { User } from "../types/User";

type AuthContextType = {
  user: User | null;
  setUser: (user: User | null) => void;
  loading?: boolean;
};

const AuthContext = createContext<AuthContextType>({
  user: null,
  setUser: () => {},
  loading: false,
});

export function useAuth() {
  return useContext(AuthContext);
}

export default function AuthProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  const [loading, setLoading] = useState(true);
  const [user, setUser] = useState<User | null>(null);

  useEffect(() => {
    setLoading(false);
  }, []);

  return (
    <AuthContext.Provider value={{ user, setUser }}>
      {!loading && children}
    </AuthContext.Provider>
  );
}

export { AuthContext };
