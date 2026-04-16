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
} from "./movies";

// Movie playback route (search params)
export type { PlaySearchParams } from "./movie-play";
export { playSearchSchema } from "./movie-play";

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
} from "./movie-details-page";

// Audio player types
export type {
  AlbumInfoType,
  AudioPlayerState,
  AudioPlayerActions,
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
export type { AuthUser, AuthUserResponseType, AdminUserType } from "./user";

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
