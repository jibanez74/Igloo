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

// Route search params
export type {
  SearchParams,
  MoviesSearchParams,
  MusicSearchParams,
  PlaySearchParams,
} from "./route-search";

// Library movie details page (UI sections)
export type {
  MovieDetailsBackdropProps,
  MovieDetailsSkipLinksProps,
  MovieDetailsPosterBlockProps,
  MovieDetailsTitleHeadingProps,
  MovieDetailsMetadataChipsProps,
  MovieDetailsGenresListProps,
  MovieDetailsHeroActionsProps,
  MovieOverviewSectionProps,
  MovieKeyCrewSectionProps,
  MovieAdditionalDetailsSectionProps,
  MovieExtraVideosSectionProps,
  MovieProductionCompaniesSectionProps,
  MovieChaptersSectionProps,
} from "./movie-details-page";

// Audio player types
export type {
  AlbumInfoType,
  AudioPlayerState,
  AudioPlayerActions,
  PlayableTrackData,
} from "./audio-player";

// API and router types
export type {
  ApiSuccessType,
  ApiFailureType,
  ApiResponseType,
  RouterContextType,
} from "./api";

// User types
export type { AuthUser, AdminUserType } from "./user";

// Notification types
export type {
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
  UpdatePlaybackSettingsResponseType,
} from "./settings";

export type {
  SettingsLayoutInput,
  SettingsLayoutState,
  SettingsTabDef,
  SettingsTabId,
} from "./settings-layout";

// Playback types
export type {
  HlsSessionRecoveryOptions,
  MoviePlaybackStatus,
  MoviePlaybackStatusArgs,
  MoviePlaybackSyncTarget,
  PlaybackModeOption,
  PlaybackSettings,
  PlaybackTimingOptions,
  RebaseOptions,
  StreamModeId,
  SubtitleTrackInfo,
  SubtitleTrackInfoOptions,
  UseMoviePlaybackDataArgs,
  VideoPlayerProps,
} from "./playback";

export type {
  UseYouTubePlayerOptions,
  UseYouTubePlayerReturn,
} from "./youtube-player";

export type { SidebarContextProps } from "./sidebar";

// Search types
export type {
  SearchSection,
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
  WatchRoomInviteUserType,
  WatchRoomInviteUsersResponseType,
  CreateWatchRoomRequestType,
  CreateWatchRoomResponseType,
  WatchRoomResponseType,
  JoinWatchRoomResponseType,
  WatchRoomPlaybackStateType,
  WatchRoomServerEventType,
} from "./watch-rooms";
