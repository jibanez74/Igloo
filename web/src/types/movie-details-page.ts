import type { PlaybackSettings } from "@/lib/playback";
import type {
  LibraryMovieDetailsMovieType,
  LibraryMovieCrewType,
  LibraryMovieExtraVideoType,
  LibraryMovieGenreType,
  LibraryMovieProductionCompanyType,
} from "./movies";
import type { AuthUser } from "./user";

export type MovieDetailsBackdropProps = {
  backdropUrl: string | null;
};

export type MovieDetailsSkipLinksProps = {
  showCrewSection: boolean;
  castNonEmpty: boolean;
  extrasNonEmpty: boolean;
  companiesNonEmpty: boolean;
};

export type MovieDetailsPosterBlockProps = {
  posterUrl: string | null;
  movieTitle: string;
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

export type MovieAdditionalDetailsSectionProps = {
  language: string | null;
  budget: number | null;
  revenue: number | null;
};

export type MovieExtraVideosSectionProps = {
  videos: LibraryMovieExtraVideoType[];
  movieId: number;
};

export type MovieProductionCompaniesSectionProps = {
  companies: LibraryMovieProductionCompanyType[];
};
