import type { ReactNode } from "react";
import type { PlaybackSettings } from "./playback";
import type {
  ChapterType,
  LibraryMovieDetailsMovieType,
  LibraryMovieCrewType,
  LibraryMovieExtraVideoType,
  LibraryMovieGenreType,
  LibraryMovieProductionCompanyType,
  MediaCapabilityBadge,
} from "./movies";
import type { AuthUser } from "./user";

export type MovieDetailsBackdropProps = {
  backdropUrl: string | null;
};

export type MovieDetailsHeroProps = {
  backdropUrl: string | null;
  posterUrl: string | null;
  movieTitle: string;
  releaseYear: number | null;
  releaseDateStr: string | null;
  tagLine: string | null;
  genres: LibraryMovieGenreType[];
  /** Metadata chip row (ratings, certification, runtime, date). */
  metadataSlot: ReactNode;
  /** Resume progress bar (library page only). */
  progressSlot?: ReactNode;
  /** Hero action row (Play/Watch/Like/⋮ or the in-theaters trailer link). */
  actionsSlot?: ReactNode;
};

export type MovieDetailsSkipLinksProps = {
  showCrewSection: boolean;
  castNonEmpty: boolean;
  chaptersNonEmpty: boolean;
  extrasNonEmpty: boolean;
};

export type MovieDetailsTitleHeadingProps = {
  title: string;
  releaseYear: number | null;
  releaseDateStr: string | null;
};

export type MovieDetailsMetadataChipsProps = {
  criticRating: number | null;
  audienceRating: number | null;
  certificationLabel: string | null;
  runtime: string | null;
  runTimeMins: number | null;
  releaseDateStr: string | null;
  /** TMDB community vote average (in-theaters page only); shown as a neutral “TMDB” chip. */
  tmdbVoteAverage?: number | null;
  /** Source-capability chips (4K/HDR/7.1/CC); library page only. */
  capabilityBadges?: MediaCapabilityBadge[];
};

export type MovieDetailsGenresListProps = {
  genres: LibraryMovieGenreType[];
};

export type MovieDetailsHeroActionsProps = {
  movieId: number;
  movie: LibraryMovieDetailsMovieType;
  movieTitle: string;
  user: AuthUser | null;
  playbackSettings: PlaybackSettings;
  onPlaybackSettingsChange: (settings: PlaybackSettings) => void;
  playbackSettingsOpen: boolean;
  onPlaybackSettingsOpenChange: (open: boolean) => void;
  technicalDetailsOpen: boolean;
  onTechnicalDetailsOpenChange: (open: boolean) => void;
  editOpen: boolean;
  onEditOpenChange: (open: boolean) => void;
  deleteOpen: boolean;
  onDeleteOpenChange: (open: boolean) => void;
};

export type MovieOverviewSectionProps = {
  overview: string | null;
};

export type MovieKeyCrewSectionProps = {
  crew: LibraryMovieCrewType[];
};

export type MovieAboutSectionProps = {
  movieTitle: string;
  /** Release status (e.g. Released); in-theaters TMDB page only. */
  status?: string | null;
  language: string | null;
  budget: number | null;
  revenue: number | null;
  companies: LibraryMovieProductionCompanyType[];
};

export type MovieExtraVideosSectionProps = {
  videos: LibraryMovieExtraVideoType[];
  movieId: number;
  /** Override trailer `returnTo` search param (e.g. `/movies/in-theaters/123`). */
  trailerReturnTo?: string;
};

export type MovieChaptersSectionProps = {
  chapters: ChapterType[];
  movieId: number;
  playbackSettings: PlaybackSettings;
};
