export type Genre = {
  ID?: number;
  tag: string;
  genreType: "movie" | "music" | "tv";
  CreatedAt?: string | Date;
  UpdatedAt?: string | Date;
  DeletedAt?: string | Date | null;
};
