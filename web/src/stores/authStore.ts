import { createStore } from "solid-js/store";
import { createRoot } from "solid-js";
import type { User } from "../types/User";

type AuthState = {
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
};

const initialState: AuthState = {
  user: null,
  isAuthenticated: false,
  isLoading: true,
};

export const [authState, setAuthState] = createRoot(() => 
  createStore<AuthState>(initialState)
);
