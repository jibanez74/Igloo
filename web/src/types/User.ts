export type User = {
  ID: number;
  name: string;
  email: string;
  username: string;
  isAdmin: boolean;
  isActive: boolean;
  CreatedAt?: string | Date;
  UpdatedAt?: string | Date;
  DeletedAt?: string | Date | null;
};
