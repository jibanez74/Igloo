// User types for authentication and account management

export type AuthUser = {
  id: number;
  name: string;
  email: string;
  is_admin: boolean;
  avatar: string | null;
  created_at: string;
  updated_at: string;
};

// API response type for auth user endpoint
export type AuthUserResponseType = {
  user: AuthUser;
};
