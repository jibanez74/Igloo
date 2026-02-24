/**
 * Normalized movie details view model and adapters.
 * Single source of truth for presentation: TMDB and library API responses
 * are mapped into this shape so shared components stay DRY.
 */

import {
  TMDB_IMAGE_BASE,
  TMDB_BACKDROP_SIZE,
  TMDB_LOGO_SIZE,
  TMDB_POSTER_SIZE,
  TMDB_PROFILE_SIZE,
} from "@/lib/constants";
import { formatCurrency } from "@/lib/format";
import type { MovieDetailsType } from "@/types";
import type { LibraryMovieDetailsResponse } from "@/types";

// ---------------------------------------------------------------------------
// Normalized view types (used only by shared UI components)
// ---------------------------------------------------------------------------

export type MovieCastMemberView = {
  id: number;
  name: string;
  character: string;
  imageUrl: string | null;
};

export type MovieCrewMemberView = {
  id: number;
  job: string;
  name: string;
};

export type MovieGenreView = {
  id: number;
  name: string;
};

export type MovieDetailsGridItemView = {
  label: string;
  value: string;
  ariaLabel?: string;
};

export type MovieExtraVideoView = {
  id: number;
  title: string;
  key: string;
  type: string;
  site: string;
  official: boolean;
};

export type MovieProductionCompanyView = {
  id: number;
  name: string;
  logoUrl: string | null;
};

export type MovieDetailsView = {
  id: number;
  title: string;
  posterUrl: string | null;
  backdropUrl: string | null;
  tagline: string | null;
  overview: string | null;
  releaseYear: number | null;
  releaseDate: string | null;
  runtimeMinutes: number | null;
  runtimeFormatted: string | null;
  rating: number | null;
  genres: MovieGenreView[];
  cast: MovieCastMemberView[];
  director: { name: string } | null;
  writers: MovieCrewMemberView[];
  productionCompanies: MovieProductionCompanyView[];
  trailerKey: string | null;
  extraVideos: MovieExtraVideoView[];
  detailsGridItems: MovieDetailsGridItemView[];
};

// ---------------------------------------------------------------------------
// Helpers (DRY: used by adapters and/or components)
// ---------------------------------------------------------------------------

/**
 * Builds a full TMDB image URL from a path and size.
 * Normalizes path (trims leading slash) to avoid double slashes.
 * All TMDB image URLs should be built through this helper.
 */
export function buildTmdbImageUrl(path: string | null, size: string): string | null {
  if (!path || !path.trim()) return null;
  const normalized = path.replace(/^\//, "");
  return `${TMDB_IMAGE_BASE}/${size}/${normalized}`;
}

export function formatRuntimeMinutes(minutes: number | null | undefined): string | null {
  if (minutes == null || minutes <= 0) return null;
  const h = Math.floor(minutes / 60);
  const m = minutes % 60;
  return h > 0 ? `${h}h ${m}m` : `${m}m`;
}

export function getRatingColorClass(score: number): string {
  if (score >= 7) return "bg-amber-500 text-slate-900";
  if (score >= 5) return "bg-amber-600/70 text-white";
  return "bg-slate-500 text-white";
}

// ---------------------------------------------------------------------------
// Adapter: TMDB → MovieDetailsView (client builds all image URLs)
// ---------------------------------------------------------------------------

export function tmdbToMovieDetailsView(movie: MovieDetailsType): MovieDetailsView {
  const releaseYear = movie.release_date
    ? new Date(movie.release_date).getFullYear()
    : null;
  const runtimeFormatted = formatRuntimeMinutes(movie.runtime ?? null);

  const director = movie.credits?.crew?.find(c => c.job === "Director") ?? null;
  const writers =
    movie.credits?.crew
      ?.filter(c => c.department === "Writing")
      .slice(0, 3)
      .map(c => ({ id: c.id, job: c.job, name: c.name })) ?? [];

  const trailer =
    movie.videos?.results?.find(
      v => v.type === "Trailer" && v.site === "YouTube",
    ) ?? null;

  const cast: MovieCastMemberView[] = (movie.credits?.cast ?? []).map(c => ({
    id: c.id,
    name: c.name,
    character: c.character,
    imageUrl: buildTmdbImageUrl(c.profile_path ?? null, TMDB_PROFILE_SIZE),
  }));

  const genres: MovieGenreView[] = (movie.genres ?? []).map(g => ({
    id: g.id,
    name: g.name,
  }));

  const detailsGridItems: MovieDetailsGridItemView[] = [
    {
      label: "Status",
      value: movie.status ?? "-",
      ariaLabel: `Status: ${movie.status ?? "Unknown"}`,
    },
    {
      label: "Original Language",
      value: (movie.original_language ?? "-").toUpperCase(),
      ariaLabel: `Original Language: ${(movie.original_language ?? "Unknown").toUpperCase()}`,
    },
    {
      label: "Budget",
      value: formatCurrency(movie.budget ?? 0),
      ariaLabel: `Budget: ${formatCurrency(movie.budget ?? 0)}`,
    },
    {
      label: "Revenue",
      value: formatCurrency(movie.revenue ?? 0),
      ariaLabel: `Revenue: ${formatCurrency(movie.revenue ?? 0)}`,
    },
  ];

  const productionCompanies: MovieProductionCompanyView[] = (movie.production_companies ?? []).map(
    (pc) => ({
      id: pc.id,
      name: pc.name,
      logoUrl: buildTmdbImageUrl(pc.logo_path ?? null, TMDB_LOGO_SIZE),
    }),
  );

  return {
    id: movie.id,
    title: movie.title,
    posterUrl: buildTmdbImageUrl(movie.poster_path ?? null, TMDB_POSTER_SIZE),
    backdropUrl: buildTmdbImageUrl(movie.backdrop_path ?? null, TMDB_BACKDROP_SIZE),
    tagline: movie.tagline ?? null,
    overview: movie.overview ?? null,
    releaseYear,
    releaseDate: movie.release_date ?? null,
    runtimeMinutes: movie.runtime ?? null,
    runtimeFormatted,
    rating: movie.vote_average > 0 ? movie.vote_average : null,
    genres,
    cast,
    director: director ? { name: director.name } : null,
    writers,
    productionCompanies,
    trailerKey: trailer?.key ?? null,
    extraVideos: [],
    detailsGridItems,
  };
}

// ---------------------------------------------------------------------------
// Adapter: Library API response → MovieDetailsView
// ---------------------------------------------------------------------------

function safeNum(v: unknown): number | null {
  if (v == null) return null;
  const n = Number(v);
  return Number.isFinite(n) ? n : null;
}

function safeStr(v: unknown): string | null {
  if (v == null) return null;
  return String(v).trim() || null;
}

export function libraryToMovieDetailsView(
  res: LibraryMovieDetailsResponse,
): MovieDetailsView {
  const m = res.movie;
  const releaseDate = safeStr(m.release_date) ?? null;
  const releaseYear = releaseDate
    ? new Date(releaseDate).getFullYear()
    : m.year != null
      ? Number(m.year)
      : null;
  const runTime = safeNum(m.run_time) ?? null;
  const runtimeFormatted = formatRuntimeMinutes(runTime);

  const rating =
    safeNum(m.audience_rating) ?? safeNum(m.critic_rating) ?? null;

  const director =
    res.crew.find(c => c.job === "Director") ?? null;
  const writers = res.crew
    .filter(c => c.department === "Writing")
    .slice(0, 3)
    .map(c => ({ id: c.id, job: c.job, name: c.artist_name }));

  const cast: MovieCastMemberView[] = res.cast.map(c => ({
    id: c.id,
    name: c.artist_name,
    character: c.character,
    imageUrl: buildTmdbImageUrl(c.artist_profile ?? null, TMDB_PROFILE_SIZE),
  }));

  const genres: MovieGenreView[] = res.genres.map(g => ({
    id: g.id,
    name: g.tag,
  }));

  const extraVideos: MovieExtraVideoView[] = res.extra_videos.map(v => ({
    id: v.id,
    title: v.title,
    key: v.key,
    type: v.type,
    site: v.site,
    official: v.official,
  }));

  const detailsGridItems: MovieDetailsGridItemView[] = [
    {
      label: "Original Language",
      value: (safeStr(m.language) ?? "-").toUpperCase(),
      ariaLabel: `Original Language: ${(safeStr(m.language) ?? "Unknown").toUpperCase()}`,
    },
    {
      label: "Budget",
      value: formatCurrency(Number(m.budget) || 0),
      ariaLabel: `Budget: ${formatCurrency(Number(m.budget) || 0)}`,
    },
    {
      label: "Revenue",
      value: formatCurrency(Number(m.revenue) || 0),
      ariaLabel: `Revenue: ${formatCurrency(Number(m.revenue) || 0)}`,
    },
  ].filter(Boolean);

  const productionCompanies: MovieProductionCompanyView[] = (
    res.production_companies ?? []
  ).map((pc) => ({
    id: pc.id,
    name: pc.name,
    logoUrl: buildTmdbImageUrl(pc.logo ?? null, TMDB_LOGO_SIZE),
  }));

  return {
    id: m.id,
    title: m.title,
    posterUrl: buildTmdbImageUrl(safeStr(m.poster_path) ?? null, TMDB_POSTER_SIZE),
    backdropUrl: buildTmdbImageUrl(safeStr(m.backdrop_path) ?? null, TMDB_BACKDROP_SIZE),
    tagline: safeStr(m.tag_line) ?? null,
    overview: safeStr(m.overview) ?? null,
    releaseYear,
    releaseDate,
    runtimeMinutes: runTime,
    runtimeFormatted,
    rating,
    genres,
    cast,
    director: director ? { name: director.artist_name } : null,
    writers,
    productionCompanies,
    trailerKey: null,
    extraVideos,
    detailsGridItems,
  };
}
