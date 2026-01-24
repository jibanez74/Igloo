// Types barrel file - re-exports all types for convenient imports
// Usage: import { TrackType, MovieDetailsType } from "@/types"

// Music library types
export type {
  NullableString,
  NullableInt64,
  NullableFloat64,
  SimpleAlbumType,
  AlbumType,
  TrackType,
  ArtistType,
  TrackGenreType,
  AlbumDetailsResponseType,
  TrackListItemType,
  TracksListResponseType,
  MusicStatsType,
  ShuffleTracksResponseType,
  AlbumsListResponseType,
  SimpleMusicianType,
  MusiciansListResponseType,
  VirtualItemLetter,
  VirtualItemTrack,
  VirtualItem,
  MusicianType,
  MusicianAlbumType,
  MusicianTrackType,
  MusicianDetailsResponseType,
  // Playlist types
  PlaylistSummaryType,
  PlaylistsListResponseType,
  PlaylistTrackType,
  PlaylistTracksResponseType,
  PlaylistCollaboratorType,
  PlaylistType,
  PlaylistDetailResponseType,
  // User listening stats types
  UserListeningStatsType,
  TopTrackType,
  TopMusicianType,
  TopGenreType,
  TopAlbumType,
  RecentlyPlayedTrackType,
  UserListeningStatsResponseType,
  TopTracksResponseType,
  TopMusiciansResponseType,
  TopGenresResponseType,
  TopAlbumsResponseType,
  RecentlyPlayedResponseType,
} from "./music";

// Movie types
export type {
  SimpleMovieType,
  CastMemberType,
  CrewMemberType,
  TheaterMovieType,
  MovieDetailsType,
} from "./movies";

// Audio player types
export type {
  AlbumInfoType,
  AudioPlayerState,
  AudioPlayerControls,
  AudioPlayerContextType,
} from "./audio-player";

// API and router types
export type {
  ApiSuccessType,
  ApiFailureType,
  ApiResponseType,
  RouterContextType,
} from "./api";

// User types
export type { AuthUser, AuthUserResponseType } from "./user";
