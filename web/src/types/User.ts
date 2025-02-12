export type User = {
  id: number;
  name: string;
  email: string;
  username: string;
  is_admin: boolean;
  is_active?: boolean;
  avatar?: string;
};

export interface UsersResponse {
  users: User[];
  current_page: number;
  total_pages: number;
  total_users: number;
}
