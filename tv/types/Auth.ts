import type { User } from "./User";

export type TokenPair = {
  access_token: string;
  refresh_token: string;
};

export type AuthResponse = {
  tokens: TokenPair;
  user: User;
};

export type AuthContextType = {
  user: User | null;
  isAuthenticated: boolean;
  login: (response: AuthResponse) => void;
  logout: () => void | undefined;
};
