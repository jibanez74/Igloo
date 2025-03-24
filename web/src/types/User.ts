export type User = {
  id: number;
  name: string;
  email: string;
  username: string;
  is_admin: boolean;
  is_active?: boolean;
  avatar?: string;
};

export type UserForm = {
  id: number;
  name: string;
  email: string;
  username: string;
  password: string;
  is_admin: boolean;
  is_active?: boolean;
  avatar?: string;
};

export type SimpleUser = {
  id: number;
  name: string;
  email: string;
  username: string;
  is_active: boolean;
  is_admin: boolean;
};
