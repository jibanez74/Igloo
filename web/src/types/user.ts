// User types for authentication and account management

export type AuthUser = {
  id: number;
  name: string;
  email: string;
  is_admin: boolean;
  avatar: string | null;
  has_pin: boolean;
  created_at: string;
  updated_at: string;
};

export type AdminUserType = AuthUser;
