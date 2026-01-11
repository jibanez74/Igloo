import type { QueryClient } from "@tanstack/react-query";

// Types for the audio player state
export type AudioPlayerState = {
  currentTrack: TrackType | null;
  tracks: TrackType[];
  albumCover: NullableString | null;
  albumTitle: string;
  musicianName: string | null;
};

// Types for the audio player controls
export type AudioPlayerControls = {
  playTrack: (
    track: TrackType,
    playlist: TrackType[],
    albumInfo: {
      cover: NullableString | null;
      title: string;
      musician: string | null;
    }
  ) => void;
  playAlbum: (
    tracks: TrackType[],
    albumInfo: {
      cover: NullableString | null;
      title: string;
      musician: string | null;
    }
  ) => void;
  shuffleAlbum: (
    tracks: TrackType[],
    albumInfo: {
      cover: NullableString | null;
      title: string;
      musician: string | null;
    }
  ) => void;
  setTrack: (track: TrackType) => void;
  stop: () => void;
  togglePlay: () => void;
  isPlaying: boolean;
  isExpanded: boolean;
  expand: () => void;
  minimize: () => void;
};

export type AudioPlayerContextType = AudioPlayerState & AudioPlayerControls;

export type ApiSuccessType<T extends Record<string, unknown>> = {
  error: false;
  message?: string;
  data: T; // always present on success
};

export type ApiFailureType = {
  error: true;
  message: string;
  data?: never; // explicitly *no* data
};

export type ApiResponseType<T extends Record<string, unknown>> =
  | ApiSuccessType<T>
  | ApiFailureType;

export type RouterContextType = {
  queryClient: QueryClient;
  audioPlayer: AudioPlayerContextType;
};

export type SimpleAlbumType = {
  id: number;
  title: string;
  cover: {
    String: string;
    Valid: boolean;
  };
  musician: {
    String: string;
    Valid: boolean;
  };
  year: {
    Int64: number;
    Valid: boolean;
  };
};

export type SimpleMovieType = {
  id: number;
  title: string;
  thumb: string;
};

export type TheaterMovieType = {
  id: number;
  title: string;
  original_title: string;
  overview: string;
  release_date: string;
  poster_path: string;
  backdrop_path: string;
  popularity: number;
  vote_average: number;
  vote_count: number;
  adult: boolean;
  original_language: string;
  genre_ids: number[];
  video: boolean;
};

// Album types for full album details
export type NullableString = {
  String: string;
  Valid: boolean;
};

export type NullableInt64 = {
  Int64: number;
  Valid: boolean;
};

export type NullableFloat64 = {
  Float64: number;
  Valid: boolean;
};

export type AlbumType = {
  id: number;
  title: string;
  sort_title: string;
  musician: NullableString;
  spotify_id: NullableString;
  spotify_popularity: NullableFloat64;
  release_date: NullableString;
  year: NullableInt64;
  total_tracks: NullableInt64;
  cover: NullableString;
  created_at: string;
  updated_at: string;
};

export type TrackType = {
  id: number;
  title: string;
  sort_title: string;
  file_path: string;
  file_name: string;
  container: string;
  mime_type: string;
  codec: string;
  size: number;
  track_index: number;
  duration: number;
  disc: number;
  channels: string;
  channel_layout: string;
  bit_rate: number;
  profile: string;
  release_date: NullableString;
  year: NullableInt64;
  composer: NullableString;
  copyright: NullableString;
  language: NullableString;
  album_id: NullableInt64;
  musician_id: NullableInt64;
  created_at: string;
  updated_at: string;
};

export type ArtistType = {
  id: number;
  name: string;
  thumb: NullableString;
  spotify_id: NullableString;
};

export type TrackGenreType = {
  track_id: number;
  genre_id: number;
  tag: string;
};

export type AlbumDetailsResponseType = {
  album: AlbumType;
  tracks: TrackType[];
  artists: ArtistType[];
  track_genres: TrackGenreType[];
  album_genres: string[];
  total_duration: number;
};

// Track list item for paginated tracks list
export type TrackListItemType = {
  id: number;
  title: string;
  duration: number;
  codec: string;
  bit_rate: number;
  file_path: string;
  album_id: NullableInt64;
  album_title: NullableString;
  musician_id: NullableInt64;
  musician_name: NullableString;
};

// Response type for paginated tracks
export type TracksListResponseType = {
  tracks: TrackListItemType[];
  total: number;
  offset: number;
  limit: number;
  has_more: boolean;
};

// Music library stats
export type MusicStatsType = {
  total_albums: number;
  total_tracks: number;
  total_musicians: number;
};

export type MovieDetailsType = {
  id: number;
  title: string;
  original_title: string;
  overview: string;
  release_date: string;
  poster_path: string;
  backdrop_path: string;
  popularity: number;
  vote_average: number;
  vote_count: number;
  adult: boolean;
  original_language: string;
  genre_ids: number[];
  video: boolean;
  runtime: number;
  status: string;
  tagline: string;
  budget: number;
  revenue: number;
  homepage: string;
  imdb_id: string;
  production_companies: {
    id: number;
    logo_path: string;
    name: string;
    origin_country: string;
  }[];
  genres: {
    id: number;
    name: string;
  }[];
  credits: {
    cast: {
      id: number;
      name: string;
      character: string;
      profile_path: string;
      order: number;
    }[];
    crew: {
      id: number;
      name: string;
      job: string;
      department: string;
      profile_path: string;
    }[];
  };
  videos: {
    results: {
      id: string;
      key: string;
      name: string;
      site: string;
      type: string;
      official: boolean;
    }[];
  };
};
