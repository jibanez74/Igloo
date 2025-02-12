export type User = {
  id: number;
  name: string;
  email: string;
  username: string;
  is_admin: boolean;
  is_active?: boolean;
  avatar?: string;
};

export type UsersResponse = {
  users: User[];
  count: number;
  pages: number;
  page: number;
  limit: number;
};
