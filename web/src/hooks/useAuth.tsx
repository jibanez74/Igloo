import { useContext } from "react";
import { AuthContext } from "../context/AuthContext";
import type { User } from "../types/User";

type UseAuthReturn = {
  user: User | null;
  isAuthenticated: boolean;
  setUser: (user: User | null) => void;
};

export default function useAuth(): UseAuthReturn {
  const context = useContext(AuthContext);

  if (!context) {
    throw new Error("useAuth must be used within an AuthProvider");
  }

  return {
    user: context.user,
    isAuthenticated: !!context.user,
    setUser: context.setUser,
  };
}
