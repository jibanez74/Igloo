import type { Genre } from "./Genre";
import type { Studio } from "./Studio";
import type { Crew } from "./Crew";
import type { Cast } from "./Cast";
import type { Video } from "./Video";
import type { Audio } from "./Audio";
import type { Subtitles } from "./Subtitles";
import type { MovieExtras } from "./MovieExtras";

export type SimpleMovie = {
  id: number;
  title: string;
  thumb: string;
  year: number;
};

export type MovieResponse = {
  movies: SimpleMovie[];
  current_page: number;
  total_pages: number;
  total_movies: number;
};

export type Movie = {
  id: number;
  title: string;
  file_path: string;
  file_name: string;
  container: string;
  size: number;
  content_type: string;
  run_time: number;
  tag_line: string;
  summary: string;
  thumb: string;
  art: string;
  tmdb_id: string;
  imdb_id: string;
  release_date: string | Date;
  year: number;
  budget: number;
  revenue: number;
  content_rating: string;
  audience_rating: number;
  critic_rating: number;
  spoken_languages: string;
  genres: Genre[];
  studios: Studio[];
  cast: Cast[];
  crew: Crew[];
  video_streams: Video[];
  audio_streams: Audio[];
  subtitles: Subtitles[];
  extras: MovieExtras[];
  created_at?: string | Date;
  updated_at?: string | Date;
};
