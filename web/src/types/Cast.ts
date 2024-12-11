import type { Artist } from "./Artist";

export type Cast = {
  ID: number;
  artist: Artist;
  character: string;
  order: number;
  CreatedAt?: string | Date;
  UpdatedAt?: string | Date;
  DeletedAt?: string | Date | null;
};
