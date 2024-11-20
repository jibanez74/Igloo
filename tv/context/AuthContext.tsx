import { createContext, useState, useContext, useEffect } from "react";
import AsyncStorage from "@react-native-async-storage/async-storage";
import { router } from "expo-router";
import api from "@/lib/api";
import setAuthToken from "@/lib/setAuthToken";
import type { User } from "@/types/User";

type AuthContextType = {
  user: User | null;
  signIn: (token: string, userData: User) => Promise<void>;
  signOut: () => Promise<void>;
};

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null);

  useEffect(() => {
    loadStoredAuth();
  }, []);

  const loadStoredAuth = async () => {
    try {
      const [token, userData] = await Promise.all([
        AsyncStorage.getItem("userToken"),
        AsyncStorage.getItem("userData"),
      ]);

      if (token && userData) {
        const parsedUser = JSON.parse(userData) as User;
        setAuthToken(token);
        setUser(parsedUser);
      }
    } catch (error) {
      console.error("Error loading auth:", error);
      await signOut();
    }
  };

  const signIn = async (token: string, userData: User) => {
    try {
      setUser(userData);
      setAuthToken(token);

      await AsyncStorage.multiSet([
        ["userToken", token],
        ["userData", JSON.stringify(userData)],
      ]);
    } catch (error) {
      console.error("Error signing in:", error);
      throw error;
    }
  };

  const signOut = async () => {
    try {
      setAuthToken(null);
      setUser(null);
      await AsyncStorage.multiRemove(["userToken", "userData"]);
      router.replace("/login");
    } catch (error) {
      console.error("Error signing out:", error);
      throw error;
    }
  };

  return (
    <AuthContext.Provider value={{ user, signIn, signOut }}>
      {children}
    </AuthContext.Provider>
  );
}

export const useAuth = () => {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
};
