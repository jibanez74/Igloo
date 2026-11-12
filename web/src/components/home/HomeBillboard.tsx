import { useQuery } from "@tanstack/react-query";
import { Link, type LinkProps } from "@tanstack/react-router";
import { Info, Play } from "lucide-react";
import WatchProgressBar from "@/components/shared/WatchProgressBar";
import { buttonVariants } from "@/components/ui/button";
import { usePosterFallback } from "@/hooks/usePosterFallback";
import {
  FOCUS_VISIBLE_RING_CLASS,
  MEDIA_FALLBACK_SURFACE_CLASS,
  MOTION_SECTION_ENTER_CLASS,
  TMDB_BACKDROP_SIZE,
  TMDB_POSTER_SIZE,
} from "@/lib/constants";
import { episodeCode, formatTimeLeft } from "@/lib/format";
import { unwrapInt, unwrapString } from "@/lib/nullable";
import {
  continueWatchingQueryOpts,
  latestMoviesQueryOpts,
} from "@/lib/query-opts";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";
import { cn } from "@/lib/utils";
import type { ContinueWatchingItemType, LatestMovieType } from "@/types";

type Featured = {
  /** Why this title is here, above its name. */
  eyebrow: string;
  title: string;
  /** Facts under the title: a year, or the episode. */
  facts: string[];
  overview: string;
  backdropUrl: string;
  posterUrl: string;
  detailsLink: LinkProps;
  playLink: LinkProps;
  /** Set when the title is in progress: play resumes it. */
  progress?: { progressSec: number; durationSec: number };
};

function featuredInProgress(item: ContinueWatchingItemType): Featured {
  const shared = {
    eyebrow: "Continue watching",
    title: item.title,
    overview: unwrapString(item.overview) ?? "",
    backdropUrl: buildTmdbImageUrl(
      unwrapString(item.backdrop_path),
      TMDB_BACKDROP_SIZE,
    ),
    posterUrl: buildTmdbImageUrl(
      unwrapString(item.poster_path),
      TMDB_POSTER_SIZE,
    ),
    progress: {
      progressSec: item.progress_sec,
      durationSec: item.duration_sec,
    },
  };

  if (item.kind === "episode") {
    return {
      ...shared,
      facts: [
        episodeCode(item.season_number, item.episode_number),
        item.episode_name,
      ],
      detailsLink: { to: "/tv-shows/$id", params: { id: String(item.show_id) } },
      playLink: {
        to: "/tv-shows/$id/episodes/$episodeId/play",
        params: { id: String(item.show_id), episodeId: String(item.id) },
      },
    };
  }

  const year = unwrapInt(item.year);
  return {
    ...shared,
    facts: year == null ? [] : [String(year)],
    detailsLink: { to: "/movies/$id", params: { id: String(item.id) } },
    playLink: { to: "/movies/$id/play", params: { id: String(item.id) } },
  };
}

function featuredNewMovie(movie: LatestMovieType): Featured {
  const year = unwrapInt(movie.year);
  return {
    eyebrow: "New in your library",
    title: movie.title,
    facts: year == null ? [] : [String(year)],
    overview: unwrapString(movie.overview) ?? "",
    backdropUrl: buildTmdbImageUrl(
      unwrapString(movie.backdrop_path),
      TMDB_BACKDROP_SIZE,
    ),
    posterUrl: buildTmdbImageUrl(
      unwrapString(movie.poster_path),
      TMDB_POSTER_SIZE,
    ),
    detailsLink: { to: "/movies/$id", params: { id: String(movie.id) } },
    playLink: { to: "/movies/$id/play", params: { id: String(movie.id) } },
  };
}

/**
 * The one title the home page leads with: what the reader was last watching,
 * or, with nothing in progress, the newest movie in the library. It renders
 * nothing when there is neither, and the rows below say why the page is bare.
 *
 * Both queries are awaited by the home route's loader, so this draws
 * populated or not at all, never a placeholder that then collapses.
 */
export default function HomeBillboard() {
  const { data: watching } = useQuery(continueWatchingQueryOpts());
  const { data: latest } = useQuery(latestMoviesQueryOpts());

  const inProgress =
    watching && !watching.error ? watching.data?.items?.[0] : undefined;
  const newest =
    latest && !latest.error ? latest.data?.movies?.[0] : undefined;

  if (inProgress) return <Billboard featured={featuredInProgress(inProgress)} />;
  if (newest) return <Billboard featured={featuredNewMovie(newest)} />;
  return null;
}

function Billboard({ featured }: { featured: Featured }) {
  const {
    eyebrow,
    title,
    facts,
    overview,
    backdropUrl,
    posterUrl,
    detailsLink,
    playLink,
    progress,
  } = featured;
  // A title with no backdrop wears its poster, blurred; one with neither, or
  // whose image fails to load, keeps the plain tile.
  const imageUrl = backdropUrl || posterUrl;
  const { showPoster: showImage, onError } = usePosterFallback(imageUrl);
  const timeLeft = progress
    ? formatTimeLeft(progress.progressSec, progress.durationSec)
    : null;
  const playVerb = progress ? "Resume" : "Play";

  return (
    <section
      aria-labelledby="home-billboard-title"
      className={cn(
        "relative isolate overflow-hidden rounded-3xl border border-border",
        MOTION_SECTION_ENTER_CLASS,
      )}
    >
      {/* The picture and the scrims that carry the text. The scrims are
          literal black over a fixed tile, so the white text reads the same in
          both themes (design-system §1.2). */}
      <div className="absolute inset-0 -z-10" aria-hidden="true">
        <div className={cn("absolute inset-0", MEDIA_FALLBACK_SURFACE_CLASS)} />
        {showImage && (
          <img
            src={imageUrl}
            alt=""
            onError={onError}
            className={cn(
              "absolute inset-0 size-full object-cover",
              !backdropUrl && "scale-110 blur-2xl",
            )}
          />
        )}
        {/* A phone has no room beside the picture, so the words sit under
            it: the card is taller there and its lower half nearly opaque. */}
        <div className="absolute inset-0 bg-linear-to-t from-black from-25% via-black/80 via-55% to-transparent sm:from-black/90 sm:from-0% sm:via-black/50 sm:via-50% sm:to-black/10" />
        <div className="absolute inset-0 hidden bg-linear-to-r from-black/75 via-black/35 to-transparent sm:block" />
      </div>

      <div className="flex min-h-120 max-w-2xl flex-col justify-end p-5 sm:min-h-96 sm:p-8 lg:min-h-104 lg:p-10">
        <p className="text-xs font-semibold tracking-[0.2em] text-white/80 uppercase drop-shadow-md">
          {eyebrow}
        </p>
        <h2
          id="home-billboard-title"
          className="mt-2 text-3xl font-semibold tracking-tight wrap-break-word text-white drop-shadow-lg sm:text-4xl lg:text-5xl"
        >
          {title}
        </h2>

        {(facts.length > 0 || timeLeft) && (
          <p className="mt-3 flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-white/85 drop-shadow-md">
            {facts.map(fact => (
              <span key={fact} className="after:ml-2 after:content-['·'] last:after:hidden">
                {fact}
              </span>
            ))}
            {timeLeft && (
              <span>
                <span aria-hidden="true">{timeLeft.text}</span>
                <span className="sr-only">{timeLeft.spoken}</span>
              </span>
            )}
          </p>
        )}

        {progress && (
          <WatchProgressBar
            progressSec={progress.progressSec}
            durationSec={progress.durationSec}
            trackClassName="bg-white/25"
            className="mt-3 max-w-xs"
          />
        )}

        {overview && (
          <p className="mt-4 line-clamp-2 max-w-xl text-sm text-white/85 drop-shadow-md sm:line-clamp-3 sm:text-base">
            {overview}
          </p>
        )}

        <div className="mt-5 flex flex-wrap gap-3">
          <Link
            {...playLink}
            aria-label={`${playVerb} ${title}`}
            className={cn(
              buttonVariants({ size: "xl", shape: "pill" }),
              "px-6 font-semibold shadow-glow",
              FOCUS_VISIBLE_RING_CLASS,
            )}
          >
            <Play className="size-4 fill-current" aria-hidden="true" />
            {playVerb}
          </Link>
          <Link
            {...detailsLink}
            aria-label={`Details for ${title}`}
            className={cn(
              buttonVariants({ variant: "outline", size: "xl", shape: "pill" }),
              "px-6 font-semibold",
              FOCUS_VISIBLE_RING_CLASS,
            )}
          >
            <Info className="size-4" aria-hidden="true" />
            Details
          </Link>
        </div>
      </div>
    </section>
  );
}
