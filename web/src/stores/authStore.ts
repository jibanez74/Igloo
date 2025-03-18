import { createStore } from "solid-js/store";
import type { User } from "../types/User";

type AuthState = {
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
};

export const [authState, setAuthState] = createStore<AuthState>({
  user: null,
  isAuthenticated: false,
  isLoading: true,
});
