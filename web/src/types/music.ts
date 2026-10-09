// MUSIC LIBRARY TYPES
// Types for albums, tracks, artists, and related music data

import type { components } from "./openapi.gen";

type Schema = components["schemas"];

export type TrackItemVariant = "album" | "musician" | "library" | "playlist";

// Simplified album type for list views and cards
export type SimpleAlbumType = Schema["SimpleAlbum"];

// Album tracks also supply the global audio player queue.
export type TrackType = Schema["AlbumTrack"];

export type ArtistType = Schema["AlbumArtist"];

// Association between a track and a genre
export type TrackGenreType = Schema["AlbumTrackGenre"];

export type AlbumDetailsResponseType = Schema["AlbumDetailsData"];

// Track item for paginated track lists (denormalized with album/artist info)
export type TrackListItemType = Schema["TrackListItem"];

export type TracksListResponseType = Schema["TracksData"];

export type LikedTracksResponseType = Schema["LikedTracksData"];

export type MusicStatsType = Schema["MusicStats"];

export type ShuffleTracksResponseType = Schema["ShuffleTracksData"];

export type AlbumsListResponseType = Schema["AlbumsData"];

export type LatestAlbumsResponseType = Schema["LatestAlbumsData"];

export type TrackLikeToggleResponseType = Schema["TrackLikeToggleData"];

export type LikedTrackIdsResponseType = Schema["LikedTrackIDsData"];

export type RecordedPlayResponseType = Schema["RecordedPlayData"];

export type SpotifyStatusType = Schema["SpotifyStatusData"];

export type SpotifyAlbumSearchRequest = Schema["SpotifySearchAlbumsRequest"];

export type SpotifyAlbumSearchResultType = Schema["SpotifyAlbumSearchResult"];

export type SpotifyAlbumSearchResponseType =
  Schema["SpotifyAlbumSearchResultsData"];

export type SpotifyTrackSearchRequest = Schema["SpotifySearchTracksRequest"];

export type SpotifyTrackSearchResultType = Schema["SpotifyTrackSearchResult"];

export type SpotifyTrackSearchResponseType =
  Schema["SpotifyTrackSearchResultsData"];

// Simplified musician type for list views and cards
export type SimpleMusicianType = Schema["SimpleMusician"];

export type MusiciansListResponseType = Schema["MusiciansData"];

// Virtual list rows for the virtualized track list
type VirtualItemLetter = {
  type: "letter";
  letter: string;
};

type VirtualItemTrack = {
  type: "track";
  track: TrackListItemType;
  trackIndex: number;
};

export type VirtualItem = VirtualItemLetter | VirtualItemTrack;

// Album with track count for musician details page
export type MusicianAlbumType = Schema["MusicianAlbum"];

// Track for musician details (includes album info, sorted alphabetically)
export type MusicianTrackType = Schema["MusicianTrack"];

export type MusicianDetailsResponseType = Schema["MusicianDetailsData"];

// PLAYLIST TYPES

// Playlist summary for list views
export type PlaylistSummaryType = Schema["PlaylistSummary"];

export type PlaylistsListResponseType = Schema["MusicPlaylistsData"];

// Track in a playlist (includes position and added_at)
export type PlaylistTrackType = Schema["PlaylistTrack"];

export type PlaylistTracksResponseType = Schema["PlaylistTracksData"];

export type PlaylistCollaboratorType = Schema["PlaylistCollaborator"];

export type PlaylistDetailResponseType = Schema["MusicPlaylistDetailData"];

// Create and update answer with the bare playlist row, without the summary's
// track count or duration.
export type PlaylistMutationResponseType = Schema["MusicPlaylistMutationData"];

export type BulkAddResponseType = Schema["BulkAddData"];

export type CreatePlaylistRequest = Schema["CreatePlaylistRequest"];

export type UpdatePlaylistRequest = Schema["UpdatePlaylistRequest"];
