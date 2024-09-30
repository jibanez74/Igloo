import type { Trailer } from "./Trailer";
import type { Studio } from "./Studio";
import type { Genre } from "./Genre";
import type { Cast } from "./Cast";
import type { Crew } from "./Crew";
import type { VideoStream } from "./VideoStream";
import type { AudioStream } from "./AudioStream";
import type { SubtitleStream } from "./SubtitleStream";

export type SimpleMovie = {
  ID: number;
  title: string;
  thumb: string;
  year: number;
};

export type MoviePagination = {
  movies: SimpleMovie[];
  currentPage: number;
  pageSize: number;
  totalPages: number;
  totalCount: number;
};

export type Movie = {
  ID: number;
  title: string;
  filePath: string;
  size: number;
  runTime: number;
  tagLine: string;
  summary: string;

  container: string;
  art: string;
  thumb: string;
  tmdbID: string;
  imdbID: string;
  year: number;
  releaseDate?: string | Date | null;
  budget: number;
  revenue: number;
  contentRating: string;
  audienceRating: number;
  criticRating: number;
  spokenLanguages: string;
  trailers: Trailer[];
  genres: Genre[];
  studios: Studio[];
  cast: Cast[];
  crew: Crew[];
  videoList: VideoStream[];
  audioList: AudioStream[];
  subtitleList: SubtitleStream[];
  CreatedAt?: string | Date;
  UpdatedAt?: string | Date;
  DeletedAt?: string | Date | null;
};
