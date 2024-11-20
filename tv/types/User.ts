import type { Movie } from "./Movie";

export interface User {
  name: string;
  email: string;
  username: string;
  isAdmin: boolean;
  thumb?: string;
}
