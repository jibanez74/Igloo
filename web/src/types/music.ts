// MUSIC LIBRARY TYPES
// Types for albums, tracks, artists, and related music data

// Nullable types - represent nullable database columns from Go backend
// The `Valid` boolean indicates whether the value is present (not NULL)

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

// Simplified album type for list views and cards
export type SimpleAlbumType = {
  id: number;
  title: string;
  cover: NullableString;
  musician: NullableString;
  year: NullableInt64;
};

// Full album details including Spotify metadata
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

// Full track details including audio file metadata
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

// Artist/musician information
export type ArtistType = {
  id: number;
  name: string;
  thumb: NullableString;
  spotify_id: NullableString;
};

// Association between a track and a genre
export type TrackGenreType = {
  track_id: number;
  genre_id: number;
  tag: string;
};

// Complete album details response from the API
export type AlbumDetailsResponseType = {
  album: AlbumType;
  tracks: TrackType[];
  artists: ArtistType[];
  track_genres: TrackGenreType[];
  album_genres: string[];
  total_duration: number;
};

// Track item for paginated track lists (denormalized with album/artist info)
export type TrackListItemType = {
  id: number;
  title: string;
  duration: number;
  codec: string;
  bit_rate: number;
  file_path: string;
  album_id: NullableInt64;
  album_title: NullableString;
  album_cover: NullableString;
  musician_id: NullableInt64;
  musician_name: NullableString;
};

// Paginated response for track listings
export type TracksListResponseType = {
  tracks: TrackListItemType[];
  total: number;
  offset: number;
  limit: number;
  has_more: boolean;
};

// Music library statistics for the dashboard
export type MusicStatsType = {
  total_albums: number;
  total_tracks: number;
  total_musicians: number;
};

// Response from the shuffle tracks endpoint
export type ShuffleTracksResponseType = {
  tracks: TrackListItemType[];
};

// Paginated response for album listings
export type AlbumsListResponseType = {
  albums: SimpleAlbumType[];
  total: number;
  page: number;
  per_page: number;
  total_pages: number;
};

// Simplified musician type for list views and cards
export type SimpleMusicianType = {
  id: number;
  name: string;
  sort_name: string;
  thumb: NullableString;
  album_count: number;
  track_count: number;
};

// Paginated response for musician listings
export type MusiciansListResponseType = {
  musicians: SimpleMusicianType[];
  total: number;
  page: number;
  per_page: number;
  total_pages: number;
};

// Virtual list item types for virtualized track lists
export type VirtualItemLetter = {
  type: "letter";
  letter: string;
};

export type VirtualItemTrack = {
  type: "track";
  track: TrackListItemType;
};

export type VirtualItem = VirtualItemLetter | VirtualItemTrack;

// Full musician details including Spotify metadata
export type MusicianType = {
  id: number;
  name: string;
  sort_name: string;
  summary: NullableString;
  spotify_popularity: NullableFloat64;
  spotify_followers: NullableInt64;
  spotify_id: NullableString;
  thumb: NullableString;
  created_at: string;
  updated_at: string;
};

// Album with track count for musician details page
export type MusicianAlbumType = {
  id: number;
  title: string;
  cover: NullableString;
  year: NullableInt64;
  release_date: NullableString;
  spotify_popularity: NullableFloat64;
  track_count: number;
};

// Track for musician details (includes album info, sorted alphabetically)
export type MusicianTrackType = {
  id: number;
  title: string;
  sort_title: string;
  duration: number;
  codec: string;
  bit_rate: number;
  file_path: string;
  track_index: number;
  disc: number;
  album_id: NullableInt64;
  album_title: NullableString;
  album_cover: NullableString;
};

// Complete musician details response from the API
export type MusicianDetailsResponseType = {
  musician: MusicianType;
  albums: MusicianAlbumType[];
  tracks: MusicianTrackType[];
  genres: string[];
  total_duration: number;
};
