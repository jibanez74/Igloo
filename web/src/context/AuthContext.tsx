import {
  useTransition,
  useState,
  createContext,
  useContext,
  useEffect,
} from "react";
import api from "../lib/api";
import type { User } from "../types/User";

type UserResponse = {
  user: User;
};

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
  const [user, setUser] = useState<User | null>(null);
  const [isPending, startTransition] = useTransition();

  const getAuthUser = () => {
    startTransition(async () => {
      try {
        const { data } = await api.get<UserResponse>("/users/me");

        if (data.user) {
          setUser(data.user);
        }
      } catch (err) {
        console.error(err);
      }
    });
  };

  useEffect(() => getAuthUser(), []);

  return (
    <AuthContext.Provider value={{ user, setUser }}>
      {!isPending && children}
    </AuthContext.Provider>
  );
}

export { AuthContext };
