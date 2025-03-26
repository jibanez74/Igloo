import { createStore } from "solid-js/store";
import { createRoot } from "solid-js";
import type { User } from "../types/User";

type AuthState = {
  user: User | null;
  isAuthenticated: boolean;
};

const initialState: AuthState = {
  user: null,
  isAuthenticated: false,
};

export const [authState, setAuthState] = createRoot(() =>
  createStore(initialState)
);
