import type { Artist } from "./Artist";

export type Crew = {
  ID?: number;
  artist: Artist;
  job: string;
  department: string;
  CreatedAt?: string | Date;
  UpdatedAt?: string | Date;
  DeletedAt?: string | Date | null;
};
