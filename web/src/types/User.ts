import type { Movie } from "./Movie";

export type User = {
  ID: number;
  name: string;
  email: string;
  username: string;
  isAdmin: boolean;
  isActive?: boolean;
  thumb?: string;
  favoriteMovies?: Movie[] | null;
  CreatedAt?: string | Date;
  UpdatedAt?: string | Date;
  DeletedAt?: string | Date | null;
};

export type UsersResponse = {
  users: User[];
  count: number;
  pages: number;
  page: number;
  limit: number;
};
