import type { Genre } from "./Genre";
import type { Studio } from "./Studio";
import type { Trailer } from "./Trailer";
import type { Crew } from "./Crew";
import type { Cast } from "./Cast";
import type { Video } from "./Video";
import type { Audio } from "./Audio";
import type { Subtitles } from "./Subtitles";

export type SimpleMovie = {
  ID: number;
  title: string;
  thumb: string;
  year: number;
};

export type Movie = {
  ID: number;
  title: string;
  filePath: string;
  container: string;
  size: number;
  contentType: string;
  resolution: number;
  runTime: number;
  tagLine: string;
  summary: string;
  thumb: string;
  art: string;
  tmdbID: string;
  imdbID: string;
  releaseDate: string | Date;
  year: number;
  budget: number;
  revenue: number;
  contentRating: string;
  audienceRating: number;
  criticRating: number;
  spokenLanguages: string;
  tailers: Trailer[];
  genres: Genre[];
  studios: Studio;
  castList: Cast[];
  crewList: Crew[];
  videoList: Video[];
  audioList: Audio[];
  subtitleList: Subtitles[];
  CreatedAt?: string | Date;
  UpdatedAt?: string | Date;
  DeletedAt?: string | Date | null;
};