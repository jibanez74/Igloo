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

// Home types
export type {
  ContinueWatchingDataType,
  ContinueWatchingItemType,
  ContinueWatchingMovieItemType,
  ContinueWatchingEpisodeItemType,
} from "./home";

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
  MediaCapabilityBadge,
  MovieTechnicalDetailsResponse,
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

// TV show types
export type {
  LatestShowType,
  LatestShowsDataType,
  ShowDetailsDataType,
  ShowSeasonEpisodesDataType,
  ShowSeasonSummaryType,
  ShowEpisodeSummaryType,
  ShowEpisodePlaybackDataType,
  ShowEpisodeTechnicalDetailsDataType,
  ShowEpisodeType,
  ShowCrewCreditType,
  ShowPersonType,
  ShowGenreType,
  ShowNetworkType,
  ShowProductionCompanyType,
  ShowLibraryItemType,
  ShowsLibraryDataType,
  ShowsStatsDataType,
  ShowGenreWithCountType,
  ShowGenresDataType,
} from "./shows";

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
  PlaybackStatus,
  PlaybackMediaKind,
  PlaybackMediaRef,
  PlaybackVideoStreamType,
  PlaybackAudioStreamType,
  PlaybackSubtitleType,
  PlaybackChapterType,
  PlaybackTechnicalFile,
  WatchProgressType,
  PlaybackSettings,
  StreamModeId,
} from "./playback";

// Search types
export type {
  PaginatedSearchResponse,
  SearchAllResponseType,
  SearchMoviesResponseType,
  SearchShowsResponseType,
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
