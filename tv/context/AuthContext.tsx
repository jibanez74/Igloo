import { createContext, useState, useContext, type ReactNode, useEffect } from "react";
import { router } from "expo-router";

// Define the User type since we're not using Supabase
type User = {
  id: number;
  username: string;
  email: string;
  token: string;
};

// Define the context type
type AuthContextType = {
  user: User | null;
  isLoading: boolean;
  signIn: (token: string) => void;
  signOut: () => void;
};

// Create the context
const AuthContext = createContext<AuthContextType | undefined>(undefined);

// Create a hook to use the auth context
export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
}

// Create the auth provider component
export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  // Sign in handler
  const signIn = async (token: string) => {
    try {
      // Store the token
      await localStorage.setItem("userToken", token);

      // Decode the JWT to get user info
      // Note: In production, verify the JWT on your server
      const decodedToken = JSON.parse(atob(token.split(".")[1]));
      
      // Set the user
      setUser({
        id: decodedToken.id,
        username: decodedToken.username,
        email: decodedToken.email,
        token
      });

      // Navigate to home screen
      router.replace("/");
    } catch (error) {
      console.error("Error signing in:", error);
      throw error;
    }
  };

  // Sign out handler
  const signOut = async () => {
    try {
      // Remove the token
      await localStorage.removeItem("userToken");
      
      // Clear the user
      setUser(null);

      // Navigate to login screen
      router.replace("/login");
    } catch (error) {
      console.error("Error signing out:", error);
      throw error;
    }
  };

  // Check for existing token on mount
  useEffect(() => {
    const loadToken = async () => {
      try {
        const token = await localStorage.getItem("userToken");
        
        if (token) {
          await signIn(token);
        }
      } catch (error) {
        console.error("Error loading token:", error);
      } finally {
        setIsLoading(false);
      }
    };

    loadToken();
  }, []);

  return (
    <AuthContext.Provider value={{ user, isLoading, signIn, signOut }}>
      {children}
    </AuthContext.Provider>
  );
}
