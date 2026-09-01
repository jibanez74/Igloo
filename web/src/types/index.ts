// Types barrel file - re-exports all types for convenient imports
// Usage: import { TrackType, MovieDetailsType } from "@/types"

// Music library types
export type {
  NullableString,
  NullableInt64,
  NullableFloat64,
} from "./nullable";

export type {
  SimpleAlbumType,
  AlbumType,
  TrackType,
  ArtistType,
  TrackGenreType,
  AlbumDetailsResponseType,
  TrackListItemType,
  TracksListResponseType,
  LikedTracksResponseType,
  MusicStatsType,
  ShuffleTracksResponseType,
  AlbumsListResponseType,
  SpotifyStatusType,
  SpotifyAlbumSearchRequest,
  SpotifyAlbumSearchResultType,
  SpotifyTrackSearchRequest,
  SpotifyTrackSearchResultType,
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
  CreatePlaylistRequest,
  UpdatePlaylistRequest,
} from "./music";

// Movie types
export type {
  LatestMovieType,
  ContinueWatchingMovieType,
  CastMemberType,
  CrewMemberType,
  TheaterMovieType,
  MovieDetailsType,
  LibraryMovieDetailsMovieType,
  LibraryMovieDetailsResponse,
  LibraryMovieCastType,
  LibraryMovieCrewType,
  LibraryMovieGenreType,
  LibraryMovieProductionCompanyType,
  LibraryMovieExtraVideoType,
  MediaCapabilityBadge,
  MovieTechnicalDetailsResponse,
  MovieWatchProgressType,
  TmdbSearchResultType,
  VideoStreamType,
  AudioStreamType,
  SubtitleType,
  ChapterType,
  MoviesLibraryListItemType,
  MoviesLibraryPaginatedDataType,
  MoviesStatsDataType,
  MovieGenreWithCountType,
  MoviePlaylistRowType,
  MoviePlaylistSummaryType,
  MoviePlaylistsListResponseType,
  MoviePlaylistDetailResponseType,
  CreateMoviePlaylistRequest,
  TmdbStatusType,
  TmdbSearchMoviesRequest,
  UpdateMovieMetadataRequest,
} from "./movies";

// Audio player types
export type {
  AlbumInfoType,
  AudioPlayerQueueState,
  AudioPlayerActions,
  AudioPlayerNowPlaying,
  PlayableTrackData,
} from "./audio-player";

// API types
export type {
  ApiSuccessType,
  ApiFailureType,
  ApiResponseType,
} from "./api";

// User types
export type { AuthUser, AdminUserType } from "./user";

export type {
  DeviceType,
  DevicesListResponseType,
  QuickConnectLookupType,
} from "./devices";

// Notification types
export type {
  NotificationTitle,
  NotificationType,
  CreateNotificationRequest,
  CreateNotificationResponseType,
  NotificationListItemType,
  NotificationsListResponseType,
  UnreadNotificationCountResponseType,
} from "./notifications";

// Settings types
export type {
  GeneralSettingsResponseType,
  GeneralSettingsType,
  HardwareAccelerationDevice,
  PlaybackProfileType,
  PlaybackSettingsResponseType,
  PlaybackSettingsType,
  SettingsType,
  UpdateGeneralSettingsRequest,
  UpdateGeneralSettingsResponseType,
  UpdateLibrarySettingsRequest,
  UpdateLibrarySettingsResponseType,
  UpdatePlaybackSettingsRequest,
} from "./settings";

// Playback types
export type {
  DevicePlaybackPreferences,
  MoviePlaybackStatus,
  PlaybackSettings,
  StreamModeId,
} from "./playback";

// Search types
export type {
  SearchSection,
  PaginatedSearchResponse,
  SearchAllResponseType,
  SearchMoviesResponseType,
  SearchAlbumsResponseType,
  SearchMusiciansResponseType,
  SearchTracksResponseType,
  SearchTab,
} from "./search";

export type { TrackItemVariant } from "./music";

// Watch room types
export type {
  WatchRoomMemberType,
  WatchRoomType,
  WatchRoomDetailType,
  WatchRoomInviteUsersResponseType,
  CreateWatchRoomRequestType,
  CreateWatchRoomResponseType,
  WatchRoomResponseType,
  JoinWatchRoomResponseType,
  WatchRoomPlaybackStateType,
  WatchRoomServerEventType,
} from "./watch-rooms";
